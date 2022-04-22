/*
	Copyright 2022 Evan Hazlett

	Permission is hereby granted, free of charge, to any person obtaining a copy of
	this software and associated documentation files (the "Software"), to deal in the
	Software without restriction, including without limitation the rights to use, copy,
	modify, merge, publish, distribute, sublicense, and/or sell copies of the Software,
	and to permit persons to whom the Software is furnished to do so, subject to the
	following conditions:

	The above copyright notice and this permission notice shall be included in all copies
	or substantial portions of the Software.

	THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED,
	INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR
	PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE
	FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT,
	TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE
	USE OR OTHER DEALINGS IN THE SOFTWARE.
*/

package server

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"time"

	ping "github.com/digineo/go-ping"
	"github.com/ehazlett/heimdall"
	v1 "github.com/ehazlett/heimdall/api/v1"
	"github.com/ehazlett/heimdall/client"
	"github.com/ehazlett/heimdall/version"
	"github.com/ehazlett/heimdall/wg"
	"github.com/gogo/protobuf/proto"
	ptypes "github.com/gogo/protobuf/types"
	"github.com/gomodule/redigo/redis"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

const (
	masterKey                 = "heimdall:master"
	clusterKey                = "heimdall:key"
	keypairsKey               = "heimdall:keypairs"
	nodesKey                  = "heimdall:nodes"
	peersKey                  = "heimdall:peers"
	routesKey                 = "heimdall:routes"
	peerIPsKey                = "heimdall:peerips"
	nodeIPsKey                = "heimdall:nodeips"
	nodeNetworksKey           = "heimdall:nodenetworks"
	authorizedPeersKey        = "heimdall:authorized"
	nodeEventJoinKey          = "heimdall:join"
	nodeEventRestartTunnelKey = "heimdall:restarttunnel"

	wireguardConfigDir = "/etc/wireguard"
)

var (
	empty                    = &ptypes.Empty{}
	masterHeartbeatInterval  = time.Second * 5
	nodeHeartbeatInterval    = time.Second * 15
	nodeHeartbeatExpiry      = 86400
	peerConfigUpdateInterval = time.Second * 10

	// ErrRouteExists is returned when a requested route is already reserved
	ErrRouteExists = errors.New("route already reserved")
	// ErrNodeDoesNotExist is returned when an invalid node is requested
	ErrNodeDoesNotExist = errors.New("node does not exist")
)

// Server represents the Heimdall server
type Server struct {
	cfg               *heimdall.Config
	nodeInterface     string
	redisCmd          *exec.Cmd
	redisURL          string
	rpool             *redis.Pool
	wpool             *redis.Pool
	replicaCh         chan struct{}
	currentConfigHash string
}

// NewServer returns a new Heimdall server
func NewServer(cfg *heimdall.Config) (*Server, error) {
	if err := os.MkdirAll(cfg.DataDir, 0750); err != nil {
		return nil, err
	}
	ctx := context.Background()
	// start embedded managed redis server
	logrus.Debugf("starting redis on %d", cfg.RedisPort)
	redisCmd, err := startRedis(ctx, &redisConfig{
		DataDir:    cfg.DataDir,
		ListenAddr: "127.0.0.1",
		Port:       cfg.RedisPort,
	})
	if err != nil {
		return nil, err
	}
	redisURL := fmt.Sprintf("redis://127.0.0.1:%d", cfg.RedisPort)
	pool, err := getPool(redisURL)
	if err != nil {
		return nil, err
	}
	return &Server{
		cfg:           cfg,
		redisCmd:      redisCmd,
		redisURL:      redisURL,
		rpool:         pool,
		wpool:         pool,
		replicaCh:     make(chan struct{}, 1),
		nodeInterface: cfg.NodeInterface,
	}, nil
}

// Register enables callers to register this service with an existing GRPC server
func (s *Server) Register(server *grpc.Server) error {
	v1.RegisterHeimdallServer(server, s)
	return nil
}

// GenerateProfile generates a new Go profile
func (s *Server) GenerateProfile() (string, error) {
	tmpfile, err := ioutil.TempFile("", "heimdall-profile-")
	if err != nil {
		return "", err
	}
	runtime.GC()
	if err := pprof.WriteHeapProfile(tmpfile); err != nil {
		return "", err
	}
	tmpfile.Close()
	return tmpfile.Name(), nil
}

func (s *Server) Run() error {
	ctx := context.Background()
	// check peer address and make a grpc request for master info if present
	masterRedisURL := ""
	if s.cfg.GRPCPeerAddress != "" {
		logrus.Debugf("joining %s", s.cfg.GRPCPeerAddress)
		c, err := s.getClient(s.cfg.GRPCPeerAddress)
		if err != nil {
			return err
		}
		defer c.Close()

		r, err := c.Join(ctx, &v1.JoinRequest{
			ID:            s.cfg.ID,
			Name:          s.cfg.Name,
			ClusterKey:    s.cfg.ClusterKey,
			GRPCAddress:   s.cfg.GRPCAddress,
			EndpointIP:    s.cfg.EndpointIP,
			EndpointPort:  uint64(s.cfg.EndpointPort),
			InterfaceName: s.cfg.InterfaceName,
		})
		if err != nil {
			return err
		}

		logrus.Debugf("master info received: id=%s grpc=%s", r.Master.ID, r.Master.GRPCAddress)
		// start tunnel
		if err := s.updatePeerConfig(ctx, r.Node, r.Peers); err != nil {
			return errors.Wrap(err, "error updating peer config")
		}

		// wait for tunnel to come up
		logrus.Infof("waiting for master %s", r.Master.ID)
		if err := s.waitForMaster(ctx, r.Master); err != nil {
			return errors.Wrap(err, "error waiting for master")
		}

		logrus.Debugf("joining master: %s", r.Master.ID)
		if err := s.joinMaster(r.Master); err != nil {
			return err
		}

		logrus.Infof("waiting for redis sync with %s", r.Master.ID)
		if err := s.waitForRedisSync(ctx); err != nil {
			return err
		}

		masterRedisURL = r.Master.RedisURL

		go s.replicaMonitor()
	} else {
		if err := s.configureNode(); err != nil {
			return err
		}
	}

	// ensure keypair
	if _, err := s.getOrCreateKeyPair(ctx, s.cfg.ID); err != nil {
		return err
	}

	// ensure node network subnet
	if err := s.ensureNetworkSubnet(ctx, s.cfg.ID); err != nil {
		return err
	}

	// initial node update
	if err := s.updateLocalNodeInfo(ctx); err != nil {
		return err
	}

	// start node heartbeat to update in redis
	go s.updateNodeInfo(ctx)

	// initial peer info update
	if err := s.updatePeerInfo(ctx, s.cfg.ID, s.cfg.Name); err != nil {
		return err
	}

	// initial config update
	node, err := s.getNode(ctx, s.cfg.ID)
	if err != nil {
		return err
	}

	peers, err := s.getPeers(ctx)
	if err != nil {
		return err
	}
	if err := s.updatePeerConfig(ctx, node, peers); err != nil {
		return err
	}

	// reconfigure redis to listen on gateway ip
	nodeIP, _, err := s.getNodeIP(ctx, s.cfg.ID)
	if err != nil {
		return err
	}
	// if no master was joined, configure local redis as master
	if masterRedisURL == "" {
		masterRedisURL = fmt.Sprintf("redis://%s:%d", nodeIP.String(), s.cfg.RedisPort)
	}
	if err := s.reconfigureRedis(ctx, nodeIP.String(), masterRedisURL); err != nil {
		return err
	}

	// start peer config updater to configure wireguard as peers join
	go s.peerUpdater(ctx)

	// start listener for pub/sub
	errCh := make(chan error, 1)
	go func() {
		c := s.rpool.Get()
		defer c.Close()

		psc := redis.PubSubConn{Conn: c}
		psc.Subscribe(nodeEventJoinKey)
		psc.Subscribe(nodeEventRestartTunnelKey)
		for {
			switch v := psc.Receive().(type) {
			case redis.Message:
				if err := s.eventHandler(ctx, v); err != nil {
					logrus.WithError(err).Error("error handling event")
				}
			case redis.Subscription:
			default:
				logrus.Debugf("unknown message type %T: %s", v, v)
			}
		}
	}()

	// authorize initial peers
	for _, peer := range s.cfg.AuthorizedPeers {
		if _, err := s.AuthorizePeer(ctx, &v1.AuthorizePeerRequest{
			ID: peer,
		}); err != nil {
			logrus.WithError(err).Errorf("error authorizing peer %s", peer)
		}
	}

	go s.startDNSServer()

	err = <-errCh
	return err
}

func (s *Server) Stop() error {
	if s.redisCmd != nil {
		if _, err := s.local(context.Background(), "SHUTDOWN"); err != nil {
			if err != io.EOF {
				logrus.WithError(err).Error("error shutting down redis")
			}
		}
	}
	return nil
}

func getPool(redisURL string) (*redis.Pool, error) {
	pool := redis.NewPool(func() (redis.Conn, error) {
		conn, err := redis.DialURL(redisURL)
		if err != nil {
			return nil, errors.Wrap(err, "unable to connect to redis")
		}

		u, err := url.Parse(redisURL)
		if err != nil {
			return nil, err
		}

		auth, ok := u.User.Password()
		if ok {
			if _, err := conn.Do("CONFIG", "SET", "MASTERAUTH", auth); err != nil {
				return nil, errors.Wrap(err, "error authenticating to redis")
			}
		}
		return conn, nil
	}, 10)

	return pool, nil
}

func (s *Server) waitForMaster(ctx context.Context, m *v1.Master) error {
	p, err := ping.New("0.0.0.0", "")
	if err != nil {
		return err
	}
	defer p.Close()

	doneCh := make(chan time.Duration)
	errCh := make(chan error)

	go func() {
		for {
			ip, err := net.ResolveIPAddr("ip4", m.GatewayIP)
			if err != nil {
				errCh <- err
				return
			}
			rtt, err := p.Ping(ip, time.Second*30)
			if err != nil {
				errCh <- err
			}
			doneCh <- rtt
		}
	}()

	select {
	case rtt := <-doneCh:
		logrus.Debugf("rtt master ping: %s", rtt)
		return nil
	case err := <-errCh:
		return err
	}
	return nil
}

func (s *Server) waitForRedisSync(ctx context.Context) error {
	doneCh := make(chan bool)
	errCh := make(chan error)

	go func() {
		for {
			info, err := redis.String(s.local(ctx, "INFO", "REPLICATION"))
			if err != nil {
				logrus.Warn(err)
				continue
			}

			b := bytes.NewBufferString(info)
			s := bufio.NewScanner(b)

			for s.Scan() {
				v := s.Text()
				parts := strings.SplitN(v, ":", 2)
				if parts[0] == "master_link_status" {
					if parts[1] == "up" {
						doneCh <- true
						return
					}
				}
			}
			time.Sleep(time.Second * 1)
		}
	}()

	select {
	case <-doneCh:
		return nil
	case err := <-errCh:
		return err
	case <-time.After(nodeHeartbeatInterval * 2):
		return fmt.Errorf("timeout waiting on sync")
	}
}

func (s *Server) ensureNetworkSubnet(ctx context.Context, id string) error {
	network, err := redis.String(s.local(ctx, "GET", s.getNodeNetworkKey(id)))
	if err != nil {
		if err != redis.ErrNil {
			return err
		}
		// allocate initial node subnet
		r, err := parseSubnetRange(s.cfg.NodeNetwork)
		if err != nil {
			return err
		}
		// iterate node networks to find first free
		nodeNetworkKeys, err := redis.Strings(s.local(ctx, "KEYS", s.getNodeNetworkKey("*")))
		if err != nil {
			return err
		}
		lookup := map[string]struct{}{}
		for _, netKey := range nodeNetworkKeys {
			n, err := redis.String(s.local(ctx, "GET", netKey))
			if err != nil {
				return err
			}
			lookup[n] = struct{}{}
		}

		subnet := r.Subnet
		size, _ := subnet.Mask.Size()

		for {
			n, ok := nextSubnet(subnet, size)
			if !ok {
				return fmt.Errorf("error getting next subnet")
			}
			if _, exists := lookup[n.String()]; exists {
				subnet = n
				continue
			}
			logrus.Debugf("allocated network %s for %s", n.String(), id)
			if err := s.updateNodeNetwork(ctx, id, n.String()); err != nil {
				return err
			}
			break
		}

		return nil
	}
	logrus.Debugf("node network for %s: %s", id, network)
	return nil
}

func (s *Server) getOrCreateKeyPair(ctx context.Context, id string) (*v1.KeyPair, error) {
	key := s.getKeyPairKey(id)
	keyData, err := redis.Bytes(s.master(ctx, "GET", key))
	if err != nil {
		if err != redis.ErrNil {
			return nil, err
		}
		logrus.Debugf("generating new keypair for %s", s.cfg.ID)
		privateKey, publicKey, err := wg.GenerateWireguardKeys(ctx)
		if err != nil {
			return nil, err
		}
		keyPair := &v1.KeyPair{
			PrivateKey: privateKey,
			PublicKey:  publicKey,
		}
		data, err := proto.Marshal(keyPair)
		if err != nil {
			return nil, err
		}
		if _, err := s.master(ctx, "SET", key, data); err != nil {
			return nil, err
		}
		return keyPair, nil
	}

	var keyPair v1.KeyPair
	if err := proto.Unmarshal(keyData, &keyPair); err != nil {
		return nil, err
	}
	return &keyPair, nil
}

func (s *Server) getNodeKey(id string) string {
	return fmt.Sprintf("%s:%s", nodesKey, id)
}

func (s *Server) getRouteKey(network string) string {
	return fmt.Sprintf("%s:%s", routesKey, network)
}

func (s *Server) getPeerKey(id string) string {
	return fmt.Sprintf("%s:%s", peersKey, id)
}

func (s *Server) getKeyPairKey(id string) string {
	return fmt.Sprintf("%s:%s", keypairsKey, id)
}

func (s *Server) getNodeNetworkKey(id string) string {
	return fmt.Sprintf("%s:%s", nodeNetworksKey, id)
}

func (s *Server) getClient(addr string) (*client.Client, error) {
	cfg := &heimdall.Config{
		TLSClientCertificate:  s.cfg.TLSClientCertificate,
		TLSClientKey:          s.cfg.TLSClientKey,
		TLSInsecureSkipVerify: s.cfg.TLSInsecureSkipVerify,
	}

	opts, err := client.DialOptionsFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	opts = append(opts,
		grpc.WithBlock(),
		grpc.WithUserAgent(fmt.Sprintf("%s/%s", version.Name, version.Version)),
	)

	return client.NewClient(s.cfg.ID, addr, opts...)
}

func (s *Server) getClusterKey(ctx context.Context) (string, error) {
	return redis.String(s.local(ctx, "GET", clusterKey))
}

func (s *Server) getWireguardConfigPath() string {
	return filepath.Join(wireguardConfigDir, s.cfg.InterfaceName+".conf")
}

func (s *Server) getTunnelName() string {
	return s.cfg.InterfaceName
}

func (s *Server) local(ctx context.Context, cmd string, args ...interface{}) (interface{}, error) {
	return s.do(ctx, s.rpool, cmd, args...)
}

func (s *Server) master(ctx context.Context, cmd string, args ...interface{}) (interface{}, error) {
	return s.do(ctx, s.wpool, cmd, args...)
}

func (s *Server) do(ctx context.Context, pool *redis.Pool, cmd string, args ...interface{}) (interface{}, error) {
	conn, err := pool.GetContext(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	r, err := conn.Do(cmd, args...)
	return r, err
}

func (s *Server) reconfigureRedis(ctx context.Context, localIP string, masterRedisURL string) error {
	logrus.Debugf("reconfiguring local redis: local=%s master=%s", localIP, masterRedisURL)
	// TODO: mutex lock for server
	if s.redisCmd != nil {
		logrus.Debug("shutting down existing redis...")
		pool, err := getPool(s.redisURL)
		if err != nil {
			return err
		}
		conn, err := pool.GetContext(ctx)
		if err != nil {
			return err
		}
		defer conn.Close()

		if _, err := conn.Do("SHUTDOWN"); err != nil {
			if err != io.EOF {
				return err
			}
		}
		time.Sleep(time.Second * 1)
	}
	// configure replica
	u, err := url.Parse(masterRedisURL)
	if err != nil {
		return err
	}
	hostIP, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		return err
	}
	redisPort, err := strconv.Atoi(port)
	if err != nil {
		return err
	}
	var replicaOf *redisReplica
	if hostIP != localIP {
		replicaOf = &redisReplica{
			Host: hostIP,
			Port: redisPort,
		}
	}

	redisCmd, err := startRedis(ctx, &redisConfig{
		ListenAddr: localIP,
		Port:       s.cfg.RedisPort,
		DataDir:    s.cfg.DataDir,
		ReplicaOf:  replicaOf,
	})
	if err != nil {
		return errors.Wrap(err, "error starting redis on private IP")
	}
	s.redisCmd = redisCmd

	localRedisURL := fmt.Sprintf("redis://%s:%d", localIP, s.cfg.RedisPort)
	pool, err := getPool(localRedisURL)
	if err != nil {
		return err
	}
	s.rpool = pool

	wpool, err := getPool(masterRedisURL)
	if err != nil {
		return err
	}
	s.wpool = wpool

	return nil
}
