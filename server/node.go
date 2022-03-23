/*
	Copyright 2021 Evan Hazlett

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
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	v1 "github.com/ehazlett/heimdall/api/v1"
	"github.com/gogo/protobuf/proto"
	"github.com/gomodule/redigo/redis"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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
	nodes, err := s.getNodes(ctx)
	if err != nil {
		return err
	}
	// attempt to connect to existing
	if len(nodes) > 0 {
		for _, node := range nodes {
			// ignore self
			if node.ID == s.cfg.ID {
				continue
			}

			logrus.Infof("attempting to join existing node %s", node.Addr)
			c, err := s.getClient(node.Addr)
			if err != nil {
				logrus.Warn(err)
				continue
			}
			r, err := c.Join(ctx, &v1.JoinRequest{
				ID:            s.cfg.ID,
				ClusterKey:    s.cfg.ClusterKey,
				GRPCAddress:   s.cfg.GRPCAddress,
				EndpointIP:    s.cfg.EndpointIP,
				EndpointPort:  uint64(s.cfg.EndpointPort),
				InterfaceName: s.cfg.InterfaceName,
			})
			if err != nil {
				c.Close()
				logrus.Warn(err)
				continue
			}

			// start tunnel
			if err := s.updatePeerConfig(ctx, r.Node, r.Peers); err != nil {
				return err
			}
			//  wait for tunnel to come up
			logrus.Infof("waiting for master %s", r.Master.ID)
			if err := s.waitForMaster(ctx, r.Master); err != nil {
				return err
			}

			if err := s.joinMaster(r.Master); err != nil {
				c.Close()
				logrus.Warn(err)
				continue
			}

			// reconfigure local redis on private IP
			if err := s.reconfigureRedis(ctx, r.Node.GatewayIP, r.Master.RedisURL); err != nil {
				return err
			}

			logrus.Infof("waiting for redis sync with %s", r.Master.ID)
			if err := s.waitForRedisSync(ctx); err != nil {
				return err
			}

			go s.replicaMonitor()

			return nil
		}
	}

	// no peer passed; start as master
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

	logrus.Debug("cluster master found; joining existing")

	// join existing master
	var master v1.Master
	if err := proto.Unmarshal(data, &master); err != nil {
		return err
	}

	logrus.Debug("joining cluster master %+v", master)

	if err := s.joinMaster(&master); err != nil {
		return err
	}
	// reconfigure local redis on private IP
	if err := s.reconfigureRedis(ctx, master.GatewayIP, master.RedisURL); err != nil {
		return err
	}

	go s.replicaMonitor()

	return nil
}

func (s *Server) disableReplica() error {
	p, err := getPool(s.redisURL)
	if err != nil {
		return err
	}
	s.wpool = p

	// signal replica monitor to stop if started as a peer
	close(s.replicaCh)

	// unset peer
	s.cfg.GRPCPeerAddress = ""
	return nil
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
					logrus.Debugf("replica monitor: node=%s", n)
					if n == nil || n.ID != s.cfg.ID {
						logrus.Debug("waiting for new master to initialize")
						continue
					}
					logrus.Debugf("replica monitor: configuring node with master %+v", n)
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
	pool, err := getPool(s.redisURL)
	if err != nil {
		return err
	}

	conn := pool.Get()
	defer conn.Close()

	logrus.Debugf("configuring redis as replica of %s", m.ID)
	u, err := url.Parse(m.RedisURL)
	if err != nil {
		return errors.Wrap(err, "error parsing master redis url")
	}
	hostPort := strings.SplitN(u.Host, ":", 2)
	host := hostPort[0]
	port := hostPort[1]
	logrus.Debugf("setting replica to %s:%s", host, port)
	if _, err := conn.Do("REPLICAOF", host, port); err != nil {
		return errors.Wrapf(err, "error setting replica to %s:%s", host, port)
	}

	logrus.Debugf("updating wpool to master on %s", m.RedisURL)
	s.wpool, err = getPool(m.RedisURL)
	if err != nil {
		return err
	}
	return nil
}

func (s *Server) updateMasterInfo(ctx context.Context) error {
	// update master info
	if _, err := s.master(ctx, "SET", clusterKey, s.cfg.ClusterKey); err != nil {
		logrus.Error("updateMasterInfo.setClusterKey")
		return err
	}
	// build redis url with gateway ip
	gatewayIP, _, err := s.getNodeIP(ctx, s.cfg.ID)
	if err != nil {
		if err == redis.ErrNil {
			logrus.Warnf("node does not have an IP assigned yet")
			return nil
		}
		return err
	}
	m := &v1.Master{
		ID:          s.cfg.ID,
		GRPCAddress: s.cfg.AdvertiseGRPCAddress,
		RedisURL:    fmt.Sprintf("redis://%s:%d", gatewayIP.String(), s.cfg.RedisPort),
		GatewayIP:   gatewayIP.String(),
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

func (s *Server) updateNodeInfo(ctx context.Context) {
	logrus.Debugf("starting node heartbeat: ttl=%s", nodeHeartbeatInterval)
	t := time.NewTicker(nodeHeartbeatInterval)
	for range t.C {
		if err := s.updateLocalNodeInfo(ctx); err != nil {
			logrus.Error(err)
			continue
		}
	}
}

func (s *Server) updateLocalNodeInfo(ctx context.Context) error {
	key := s.getNodeKey(s.cfg.ID)
	keyPair, err := s.getOrCreateKeyPair(ctx, s.cfg.ID)
	if err != nil {
		return errors.Wrapf(err, "error getting keypair for %s", s.cfg.ID)
	}
	nodeIP, _, err := s.getNodeIP(ctx, s.cfg.ID)
	if err != nil {
		return errors.Wrapf(err, "error getting node IP for %s", s.cfg.ID)
	}
	node := &v1.Node{
		Updated:       time.Now(),
		ID:            s.cfg.ID,
		Addr:          s.cfg.AdvertiseGRPCAddress,
		KeyPair:       keyPair,
		EndpointIP:    s.cfg.EndpointIP,
		EndpointPort:  uint64(s.cfg.EndpointPort),
		GatewayIP:     nodeIP.String(),
		InterfaceName: s.cfg.InterfaceName,
	}

	logrus.Debugf("local node info: %+v", node)

	data, err := proto.Marshal(node)
	if err != nil {
		return errors.Wrap(err, "error marshalling local node info")
	}

	if _, err := s.master(ctx, "SET", key, data); err != nil {
		return err
	}

	if _, err := s.master(ctx, "EXPIRE", key, nodeHeartbeatExpiry); err != nil {
		return err
	}

	return nil
}

func (s *Server) createNode(ctx context.Context, req *v1.JoinRequest) (*v1.Node, error) {
	key := s.getNodeKey(req.ID)
	keyPair, err := s.getOrCreateKeyPair(ctx, req.ID)
	if err != nil {
		return nil, errors.Wrapf(err, "error getting/creating keypair for %s", req.ID)
	}
	nodeIP, _, err := s.getNodeIP(ctx, req.ID)
	if err != nil {
		return nil, errors.Wrapf(err, "error getting node ip for %s", req.ID)
	}
	node := &v1.Node{
		Updated:       time.Now(),
		ID:            req.ID,
		Addr:          req.GRPCAddress,
		KeyPair:       keyPair,
		EndpointIP:    req.EndpointIP,
		EndpointPort:  uint64(req.EndpointPort),
		GatewayIP:     nodeIP.String(),
		InterfaceName: req.InterfaceName,
	}

	data, err := proto.Marshal(node)
	if err != nil {
		return nil, err
	}

	if _, err := s.master(ctx, "SET", key, data); err != nil {
		return nil, err
	}

	if _, err := s.master(ctx, "EXPIRE", key, nodeHeartbeatExpiry); err != nil {
		return nil, err
	}

	return node, nil
}
