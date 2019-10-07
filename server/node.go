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
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/gomodule/redigo/redis"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	v1 "github.com/stellarproject/heimdall/api/v1"
)

// Nodes returns a list of known nodes
func (s *Server) Nodes(ctx context.Context, req *v1.NodesRequest) (*v1.NodesResponse, error) {
	nodes, err := s.getNodes(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.NodesResponse{
		Nodes: nodes,
	}, nil
}

func (s *Server) getNodes(ctx context.Context) ([]*v1.Node, error) {
	nodeKeys, err := redis.Strings(s.local(ctx, "KEYS", s.getNodeKey("*")))
	if err != nil {
		return nil, err
	}
	var nodes []*v1.Node
	for _, nodeKey := range nodeKeys {
		data, err := redis.Bytes(s.local(ctx, "GET", nodeKey))
		if err != nil {
			return nil, err
		}

		var node v1.Node
		if err := proto.Unmarshal(data, &node); err != nil {
			return nil, err
		}
		nodes = append(nodes, &node)
	}
	return nodes, nil
}

func (s *Server) getNode(ctx context.Context, id string) (*v1.Node, error) {
	data, err := redis.Bytes(s.local(ctx, "GET", s.getNodeKey(id)))
	if err != nil {
		return nil, err
	}

	var node v1.Node
	if err := proto.Unmarshal(data, &node); err != nil {
		return nil, err
	}
	return &node, nil
}

func (s *Server) configureNode() error {
	ctx := context.Background()
	nodeKeys, err := redis.Strings(s.local(ctx, "KEYS", s.getNodeKey("*")))
	if err != nil {
		return err
	}
	// attempt to connect to existing
	if len(nodeKeys) > 0 {
		for _, nodeKey := range nodeKeys {
			nodeData, err := redis.Bytes(s.local(ctx, "GET", nodeKey))
			if err != nil {
				logrus.Warn(err)
				continue
			}
			var node v1.Node
			if err := proto.Unmarshal(nodeData, &node); err != nil {
				return err
			}
			// ignore self
			if node.Addr == s.cfg.GRPCAddress {
				continue
			}

			logrus.Infof("attempting to join existing node %s", node.Addr)
			c, err := s.getClient(node.Addr)
			if err != nil {
				logrus.Warn(err)
				continue
			}
			m, err := c.Join(s.cfg.ClusterKey)
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

			go s.replicaMonitor()

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
	logrus.Debugf("starting replica monitor: ttl=%s", masterHeartbeatInterval)
	s.replicaCh = make(chan struct{}, 1)
	t := time.NewTicker(masterHeartbeatInterval)
	go func() {
		for range t.C {
			if _, err := redis.Bytes(s.local(context.Background(), "GET", masterKey)); err != nil {
				if err == redis.ErrNil {
					// skip configure until new leader election
					n, err := s.getNextMaster(context.Background())
					if err != nil {
						logrus.Error(err)
						continue
					}
					if n.ID != s.cfg.ID {
						logrus.Debugf("waiting for new master to initialize: %s", n.ID)
						continue
					}
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

func (s *Server) getNextMaster(ctx context.Context) (*v1.Node, error) {
	nodes, err := s.getNodes(ctx)
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return nil, nil
	}
	sort.SliceStable(nodes, func(i, j int) bool { return nodes[i].Updated.Before(nodes[j].Updated) })
	return nodes[len(nodes)-1], nil
}

func (s *Server) masterHeartbeat() {
	logrus.Debugf("starting master heartbeat: ttl=%s", masterHeartbeatInterval)
	// initial update
	ctx, cancel := context.WithTimeout(context.Background(), masterHeartbeatInterval)
	defer cancel()

	logrus.Infof("cluster master key=%s", s.cfg.ClusterKey)
	if err := s.updateMasterInfo(ctx); err != nil {
		logrus.Error(err)
	}

	t := time.NewTicker(masterHeartbeatInterval)
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

	if _, err := s.master(ctx, "EXPIRE", masterKey, int(masterHeartbeatInterval.Seconds())); err != nil {
		return errors.Wrap(err, "error setting expire for master info")
	}
	return nil
}

func (s *Server) nodeHeartbeat(ctx context.Context) {
	logrus.Debugf("starting node heartbeat: ttl=%s", nodeHeartbeatInterval)
	t := time.NewTicker(nodeHeartbeatInterval)
	key := s.getNodeKey(s.cfg.ID)
	for range t.C {
		keyPair, err := s.getOrCreateKeyPair(ctx, s.cfg.ID)
		if err != nil {
			logrus.Error(err)
			continue
		}
		nodeIP, _, err := s.getNodeIP(ctx, s.cfg.ID)
		if err != nil {
			logrus.Error(err)
			continue
		}
		node := &v1.Node{
			Updated:      time.Now(),
			ID:           s.cfg.ID,
			Addr:         s.cfg.GRPCAddress,
			KeyPair:      keyPair,
			EndpointIP:   s.cfg.EndpointIP,
			EndpointPort: uint64(s.cfg.EndpointPort),
			GatewayIP:    nodeIP.String(),
		}

		data, err := proto.Marshal(node)
		if err != nil {
			logrus.Error(err)
			continue
		}

		if _, err := s.master(ctx, "SET", key, data); err != nil {
			logrus.Error(err)
			continue
		}

		if _, err := s.master(ctx, "EXPIRE", key, nodeHeartbeatExpiry); err != nil {
			logrus.Error(err)
			continue
		}
	}
}
