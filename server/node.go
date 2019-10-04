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
	"crypto/sha256"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/gomodule/redigo/redis"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/stellarproject/heimdall"
	v1 "github.com/stellarproject/heimdall/api/v1"
)

func (s *Server) configureNode() error {
	ctx := context.Background()
	nodes, err := redis.Strings(s.local(ctx, "KEYS", s.getNodeKey("*")))
	if err != nil {
		return err
	}
	// attempt to connect to existing
	if len(nodes) > 0 {
		for _, node := range nodes {
			addr, err := redis.String(s.local(ctx, "GET", node))
			if err != nil {
				logrus.Warn(err)
				continue
			}
			// ignore self
			if addr == s.cfg.GRPCAddress {
				continue
			}

			logrus.Infof("attempting to join existing node %s", addr)
			c, err := s.getClient(addr)
			if err != nil {
				logrus.Warn(err)
				continue
			}
			m, err := c.Connect(s.cfg.ClusterKey)
			if err != nil {
				c.Close()
				logrus.Warn(err)
				continue
			}

			if err := s.joinMaster(m); err != nil {
				c.Close()
				logrus.Warn(err)
				continue
			}

			return nil
		}
	}

	data, err := redis.Bytes(s.local(context.Background(), "GET", masterKey))
	if err != nil {
		if err != redis.ErrNil {
			return err
		}
		logrus.Infof("starting as master id=%s", s.cfg.ID)
		if _, err := s.local(context.Background(), "REPLICAOF", "NO", "ONE"); err != nil {
			return err
		}

		// start server heartbeat
		logrus.Debug("starting master heartbeat")
		go s.masterHeartbeat()

		// reset replica settings when promoting to master
		logrus.Debug("disabling replica status")
		s.disableReplica()

		return nil
	}

	var master v1.Master
	if err := proto.Unmarshal(data, &master); err != nil {
		return err
	}

	if err := s.joinMaster(&master); err != nil {
		return err
	}

	go s.replicaMonitor()

	return nil
}

func (s *Server) disableReplica() {
	s.wpool = getPool(s.cfg.RedisURL)

	// signal replica monitor to stop if started as a peer
	close(s.replicaCh)

	// unset peer
	s.cfg.GRPCPeerAddress = ""
}

func (s *Server) replicaMonitor() {
	logrus.Debugf("starting replica monitor: ttl=%s", heartbeatInterval)
	s.replicaCh = make(chan struct{}, 1)
	t := time.NewTicker(heartbeatInterval)
	go func() {
		for range t.C {
			if _, err := redis.Bytes(s.local(context.Background(), "GET", masterKey)); err != nil {
				if err == redis.ErrNil {
					if err := s.configureNode(); err != nil {
						logrus.Error(err)
						continue
					}
					return
				}
				logrus.Error(err)
			}
		}
	}()

	<-s.replicaCh
	logrus.Debug("stopping replica monitor")
	t.Stop()
}

func (s *Server) masterHeartbeat() {
	logrus.Debugf("starting master heartbeat: ttl=%s", heartbeatInterval)
	// initial update
	ctx, cancel := context.WithTimeout(context.Background(), heartbeatInterval)
	defer cancel()

	logrus.Infof("cluster master key=%s", s.cfg.ClusterKey)
	if err := s.updateMasterInfo(ctx); err != nil {
		logrus.Error(err)
	}

	t := time.NewTicker(heartbeatInterval)
	for range t.C {
		if err := s.updateMasterInfo(ctx); err != nil {
			logrus.Error(err)
			continue
		}
	}
}

func (s *Server) joinMaster(m *v1.Master) error {
	// configure replica
	logrus.Infof("configuring node as replica of %+v", m.ID)
	conn, err := redis.DialURL(s.cfg.RedisURL)
	if err != nil {
		return errors.Wrap(err, "unable to connect to redis")
	}
	defer conn.Close()

	u, err := url.Parse(m.RedisURL)
	if err != nil {
		return errors.Wrap(err, "error parsing master redis url")
	}
	hostPort := strings.SplitN(u.Host, ":", 2)
	host := hostPort[0]
	port := hostPort[1]
	if _, err := conn.Do("REPLICAOF", host, port); err != nil {
		return err
	}
	// auth
	auth, ok := u.User.Password()
	if ok {
		if _, err := conn.Do("CONFIG", "SET", "MASTERAUTH", auth); err != nil {
			return errors.Wrap(err, "error authenticating to redis")
		}
	}

	s.wpool = getPool(m.RedisURL)
	return nil
}

func (s *Server) updateMasterInfo(ctx context.Context) error {
	// update master info
	if _, err := s.master(ctx, "SET", clusterKey, s.cfg.ClusterKey); err != nil {
		return err
	}
	m := &v1.Master{
		ID:          s.cfg.ID,
		GRPCAddress: s.cfg.GRPCAddress,
		RedisURL:    s.cfg.AdvertiseRedisURL,
	}
	data, err := proto.Marshal(m)
	if err != nil {
		return errors.Wrap(err, "error marshalling master info")
	}

	if _, err := s.master(ctx, "SET", masterKey, data); err != nil {
		return errors.Wrap(err, "error setting master info")
	}

	if _, err := s.master(ctx, "EXPIRE", masterKey, int(heartbeatInterval.Seconds())); err != nil {
		return errors.Wrap(err, "error setting expire for master info")
	}
	return nil
}

func (s *Server) updatePeerInfo(ctx context.Context) error {
	// check for existing key
	endpoint := fmt.Sprintf("%s:%d", heimdall.GetIP(), s.cfg.WireguardPort)

	peer, err := s.getPeerInfo(ctx)
	if err != nil {
		return err
	}

	// TODO: build allowedIPs from routes and peer network
	allowedIPs := []string{s.cfg.PeerNetwork}
	ipHash := hashIPs(allowedIPs)

	// check cached info and validate
	if peer != nil {
		peerIPHash := hashIPs(peer.AllowedIPs)
		// if endpoint is the same assume unchanged
		if peer.Endpoint == endpoint && peerIPHash == ipHash {
			logrus.Debugf("peer info: public=%s endpoint=%s", peer.PublicKey, peer.Endpoint)
			return nil
		}
	}

	privateKey, publicKey, err := generateWireguardKeys(ctx)
	if err != nil {
		return err
	}
	// TODO: allowed IPs
	n := &v1.Peer{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
		AllowedIPs: allowedIPs,
		Endpoint:   endpoint,
	}

	logrus.Debugf("peer info: public=%s endpoint=%s", n.PublicKey, n.Endpoint)
	data, err := proto.Marshal(n)
	if err != nil {
		return err
	}
	key := s.getPeerKey(s.cfg.ID)
	if _, err := s.master(ctx, "SET", key, data); err != nil {
		return err
	}
	return nil
}

func (s *Server) getPeerInfo(ctx context.Context) (*v1.Peer, error) {
	key := s.getPeerKey(s.cfg.ID)
	data, err := redis.Bytes(s.local(ctx, "GET", key))
	if err != nil {
		if err == redis.ErrNil {
			return nil, nil
		}
		return nil, err
	}
	var peer v1.Peer
	if err := proto.Unmarshal(data, &peer); err != nil {
		return nil, err
	}

	return &peer, nil
}

func (s *Server) nodeHeartbeat() {
	logrus.Debugf("starting node heartbeat: ttl=%s", nodeHeartbeatInterval)
	ctx := context.Background()
	t := time.NewTicker(nodeHeartbeatInterval)
	key := s.getNodeKey(s.cfg.ID)
	for range t.C {
		if _, err := s.master(ctx, "SET", key, s.cfg.GRPCAddress); err != nil {
			logrus.Error(err)
			continue
		}

		if _, err := s.master(ctx, "EXPIRE", key, nodeHeartbeatExpiry); err != nil {
			logrus.Error(err)
			continue
		}
	}
}

func hashIPs(ips []string) string {
	h := sha256.New()
	for _, ip := range ips {
		h.Write([]byte(ip))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
