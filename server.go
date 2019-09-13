package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	v1 "github.com/crosbymichael/guard/api/v1"
	"github.com/gliderlabs/ssh"
	"github.com/gomodule/redigo/redis"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

const (
	ipsKey          = "gatekeeper/ips"
	configKeyPrefix = "gatekeeper/config/"
)

type ServerConfig struct {
	ListenPort  int
	DBPath      string
	KeysPath    string
	HostKeyPath string
	RedisURL    string
	Subnet      string
	GuardAddr   string
	GuardTunnel string
	GuardDNS    string
}

type Server struct {
	cfg        *ServerConfig
	publicKeys []ssh.PublicKey
	mu         *sync.Mutex
}

func NewServer(cfg *ServerConfig) (*Server, error) {
	if cfg.ListenPort == 0 {
		cfg.ListenPort = 2222
	}
	return &Server{
		cfg: cfg,
		mu:  &sync.Mutex{},
	}, nil
}

func (s *Server) Run() error {
	if err := s.loadKeys(); err != nil {
		return err
	}

	ssh.Handle(func(session ssh.Session) {
		id := s.getID(session.PublicKey())
		config, err := s.getConfig(id)
		if err != nil {
			logrus.Error(err)
			return
		}
		logrus.Debugf("config: id=%s", id)
		io.WriteString(session, config+"\n")
	})

	pubKeyOption := ssh.PublicKeyAuth(func(ctx ssh.Context, key ssh.PublicKey) bool {
		return s.isAuthorized(ctx, key)
	})

	logrus.Infof("starting ssh server on port %d", s.cfg.ListenPort)
	opts := []ssh.Option{
		pubKeyOption,
	}
	if _, err := os.Stat(s.cfg.HostKeyPath); err == nil {
		opts = append(opts, ssh.HostKeyFile(s.cfg.HostKeyPath))
	}
	return ssh.ListenAndServe(fmt.Sprintf(":%d", s.cfg.ListenPort), nil, opts...)
}

func (s *Server) loadKeys() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cfg.KeysPath == "" {
		return nil
	}

	keys, err := ioutil.ReadDir(s.cfg.KeysPath)
	if err != nil {
		return err
	}

	pubKeys := []ssh.PublicKey{}
	for _, k := range keys {
		logrus.Debugf("loading public key %s", k.Name())
		p := filepath.Join(s.cfg.KeysPath, k.Name())
		data, err := ioutil.ReadFile(p)
		if err != nil {
			return err
		}
		k, _, _, _, err := ssh.ParseAuthorizedKey(data)
		if err != nil {
			return err
		}
		pubKeys = append(pubKeys, k)
	}

	s.publicKeys = pubKeys
	return nil
}

func (s *Server) isAuthorized(ctx ssh.Context, key ssh.PublicKey) bool {
	for _, k := range s.publicKeys {
		if ssh.KeysEqual(key, k) {
			return true
		}
	}
	logrus.WithFields(logrus.Fields{
		"user": ctx.User(),
		"addr": ctx.RemoteAddr(),
	}).Warn("access denied")
	return false
}

func (s *Server) getConfig(id string) (string, error) {
	c, err := s.getConn()
	if err != nil {
		return "", err
	}
	defer c.Close()
	key := path.Join(configKeyPrefix, id)
	cfg, err := redis.String(c.Do("GET", key))
	if err != nil && err != redis.ErrNil {
		return "", err
	}
	if cfg != "" {
		return cfg, nil
	}

	conn, err := grpc.Dial(s.cfg.GuardAddr, grpc.WithInsecure())
	if err != nil {
		return "", errors.Wrap(err, "error connecting to guard server")
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	client := v1.NewWireguardClient(conn)

	ip, ipnet, err := s.getOrAllocateIP(id, s.cfg.Subnet)
	if err != nil {
		return "", err
	}

	r, err := client.NewPeer(ctx, &v1.NewPeerRequest{
		ID:      s.cfg.GuardTunnel,
		PeerID:  id,
		Address: ip.String() + "/32",
	})
	if err != nil {
		return "", err
	}
	// generate peer tunnel config
	t := &v1.Tunnel{
		PrivateKey: r.Peer.PrivateKey,
		Address:    r.Peer.AllowedIPs[0],
		DNS:        s.cfg.GuardDNS,
		Peers: []*v1.Peer{
			{
				ID:         r.Tunnel.ID,
				PublicKey:  r.Tunnel.PublicKey,
				Endpoint:   net.JoinHostPort(r.Tunnel.Endpoint, r.Tunnel.ListenPort),
				AllowedIPs: []string{ipnet.String()},
			},
		},
	}
	b := bytes.NewBuffer(nil)
	t.Render(b)
	if _, err := c.Do("SET", key, b.String()); err != nil {
		return "", err
	}
	return b.String(), nil
}
