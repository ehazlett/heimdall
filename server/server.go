/*
	Copyright 2019 Stellar Project

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
	"context"
	"io/ioutil"
	"runtime"
	"runtime/pprof"
	"time"

	ptypes "github.com/gogo/protobuf/types"
	"github.com/gomodule/redigo/redis"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/stellarproject/heimdall"
	v1 "github.com/stellarproject/heimdall/api/v1"
	"github.com/stellarproject/heimdall/client"
	"google.golang.org/grpc"
)

const (
	masterKey   = "heimdall:master"
	clusterKey  = "heimdall:key"
	nodesKey    = "heimdall:nodes"
	nodeJoinKey = "heimdall:join"
)

var (
	empty                 = &ptypes.Empty{}
	heartbeatInterval     = time.Second * 5
	nodeHeartbeatInterval = time.Second * 60
	nodeHeartbeatExpiry   = 86400
)

type Server struct {
	cfg       *heimdall.Config
	rpool     *redis.Pool
	wpool     *redis.Pool
	replicaCh chan struct{}
}

func NewServer(cfg *heimdall.Config) (*Server, error) {
	pool := getPool(cfg.RedisURL)
	return &Server{
		cfg:       cfg,
		rpool:     pool,
		wpool:     pool,
		replicaCh: make(chan struct{}, 1),
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
	// check peer address and make a grpc request for master info if present
	if s.cfg.GRPCPeerAddress != "" {
		logrus.Debugf("joining %s", s.cfg.GRPCPeerAddress)
		c, err := s.getClient(s.cfg.GRPCPeerAddress)
		if err != nil {
			return err
		}
		defer c.Close()

		master, err := c.Connect(s.cfg.ClusterKey)
		if err != nil {
			return err
		}

		logrus.Debugf("master info received: %+v", master)
		if err := s.joinMaster(master); err != nil {
			return err
		}

		go s.replicaMonitor()
	} else {
		// starting as master; remove existing key
		if err := s.configureNode(); err != nil {
			return err
		}
	}

	go s.nodeHeartbeat()

	// start listener for pub/sub
	errCh := make(chan error, 1)
	go func() {
		c := s.rpool.Get()
		defer c.Close()

		psc := redis.PubSubConn{Conn: c}
		psc.Subscribe(nodeJoinKey)
		for {
			switch v := psc.Receive().(type) {
			case redis.Message:
				// TODO: handle join notify
				logrus.Debug("join notify")
			case redis.Subscription:
			default:
				logrus.Debugf("unknown message type %T", v)
			}
		}
	}()

	err := <-errCh
	return err
}

func (s *Server) Stop() error {
	s.rpool.Close()
	s.wpool.Close()
	return nil
}

func getPool(u string) *redis.Pool {
	pool := redis.NewPool(func() (redis.Conn, error) {
		conn, err := redis.DialURL(u)
		if err != nil {
			return nil, errors.Wrap(err, "unable to connect to redis")
		}
		return conn, nil
	}, 10)

	return pool
}

func (s *Server) getClient(addr string) (*client.Client, error) {
	return client.NewClient(s.cfg.ID, addr)
}

func (s *Server) getClusterKey(ctx context.Context) (string, error) {
	return redis.String(s.local(ctx, "GET", clusterKey))
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
