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
	"os"
	"time"

	"github.com/ehazlett/heimdall"
	v1 "github.com/ehazlett/heimdall/api/v1"
	"github.com/ehazlett/heimdall/wg"
	"github.com/gogo/protobuf/proto"
	"github.com/gomodule/redigo/redis"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Peers returns a list of known peers
func (s *Server) Peers(ctx context.Context, req *v1.PeersRequest) (*v1.PeersResponse, error) {
	peers, err := s.getPeers(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.PeersResponse{
		Peers: peers,
	}, nil
}

func (s *Server) getPeers(ctx context.Context) ([]*v1.Peer, error) {
	peerKeys, err := redis.Strings(s.local(ctx, "KEYS", s.getPeerKey("*")))
	if err != nil {
		return nil, err
	}
	var peers []*v1.Peer
	for _, peerKey := range peerKeys {
		data, err := redis.Bytes(s.local(ctx, "GET", peerKey))
		if err != nil {
			return nil, err
		}

		var peer v1.Peer
		if err := proto.Unmarshal(data, &peer); err != nil {
			return nil, err
		}
		peerIP, err := s.getPeerIP(ctx, peer.ID)
		if err != nil {
			return nil, err
		}
		if peerIP != nil {
			peer.PeerIP = peerIP.String()
		}
		peers = append(peers, &peer)
	}
	return peers, nil
}

func (s *Server) peerUpdater(ctx context.Context) {
	logrus.Debugf("starting peer config updater: ttl=%s", peerConfigUpdateInterval)
	t := time.NewTicker(peerConfigUpdateInterval)
	for range t.C {
		uctx, cancel := context.WithTimeout(ctx, peerConfigUpdateInterval)
		if err := s.updatePeerInfo(uctx, s.cfg.ID, s.cfg.Name); err != nil {
			logrus.Errorf("updateLocalPeerInfo: %s", err)
			cancel()
			continue
		}

		peers, err := s.getPeers(ctx)
		if err != nil {
			logrus.Error(err)
			cancel()
			continue
		}

		node, err := s.getNode(ctx, s.cfg.ID)
		if err != nil {
			logrus.Error(err)
			cancel()
			continue
		}

		if err := s.updatePeerConfig(uctx, node, peers); err != nil {
			logrus.Errorf("updatePeerConfig: %s", err)
			cancel()
			continue
		}
		cancel()
	}
}

func (s *Server) updatePeerInfo(ctx context.Context, id, name string) error {
	keypair, err := s.getOrCreateKeyPair(ctx, id)
	if err != nil {
		return errors.Wrap(err, "error getting or creating keypair")
	}

	endpoint, err := s.getPeerEndpoint(ctx, id)
	if err != nil {
		return errors.Wrap(err, "error getting peer endpoint")
	}

	// build allowedIPs from routes and peer network
	allowedIPs := []string{}

	// add peer net
	if endpoint == "" {
		peerIP, err := s.getPeerIP(ctx, id)
		if err != nil {
			return err
		}
		allowedIPs = append(allowedIPs, peerIP.String()+"/32")
	}
	nodes, err := s.getNodes(ctx)
	if err != nil {
		return errors.Wrap(err, "error getting nodes")
	}

	for _, node := range nodes {
		// only add the route if a peer to prevent route duplicate
		if node.ID != id {
			continue
		}

		_, gatewayNet, err := s.getNodeIP(ctx, node.ID)
		if err != nil {
			return errors.Wrapf(err, "error getting node ip for %s", node.ID)
		}

		allowedIPs = append(allowedIPs, gatewayNet.String())
	}

	routes, err := s.getRoutes(ctx)
	if err != nil {
		return errors.Wrap(err, "error getting routes")
	}

	for _, route := range routes {
		// only add the route if a peer to prevent route blackhole
		if route.NodeID != id {
			continue
		}

		allowedIPs = append(allowedIPs, route.Network)
	}

	n := &v1.Peer{
		ID:         id,
		Name:       name,
		KeyPair:    keypair,
		AllowedIPs: allowedIPs,
		Endpoint:   endpoint,
	}

	data, err := proto.Marshal(n)
	if err != nil {
		return errors.Wrap(err, "error marshalling peer info")
	}
	pHash := heimdall.HashData(data)

	key := s.getPeerKey(id)
	peerData, err := redis.Bytes(s.local(ctx, "GET", key))
	if err != nil {
		if err != redis.ErrNil {
			return err
		}
	}

	eHash := heimdall.HashData(peerData)

	// skip update if same
	if pHash == eHash {
		return nil
	}

	if _, err := s.master(ctx, "SET", key, data); err != nil {
		return err
	}

	logrus.Debugf("peer info updated: id=%s", id)

	return nil
}

func (s *Server) getPeerEndpoint(ctx context.Context, id string) (string, error) {
	node, err := s.getNode(ctx, id)
	if err != nil {
		if err == redis.ErrNil {
			return "", nil
		}
		return "", err
	}
	if node == nil {
		return "", nil
	}
	return fmt.Sprintf("%s:%d", node.EndpointIP, node.EndpointPort), nil
}

func (s *Server) getPeerInfo(ctx context.Context, id string) (*v1.Peer, error) {
	key := s.getPeerKey(id)
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

func (s *Server) updatePeerConfig(ctx context.Context, node *v1.Node, peers []*v1.Peer) error {
	var nodePeers []*v1.Peer
	for _, peer := range peers {
		// do not add self as a peer
		if peer.ID == node.ID {
			continue
		}

		nodePeers = append(nodePeers, peer)
	}

	wireguardCfg := &wg.Config{
		Interface:     node.InterfaceName,
		NodeInterface: s.nodeInterface,
		PrivateKey:    node.KeyPair.PrivateKey,
		ListenPort:    int(node.EndpointPort),
		Address:       fmt.Sprintf("%s/%d", node.GatewayIP, 16),
		Peers:         nodePeers,
	}

	wireguardConfigPath := s.getWireguardConfigPath()
	tmpCfg, err := wg.GenerateNodeConfig(wireguardCfg, wireguardConfigPath)
	if err != nil {
		return err
	}

	h, err := heimdall.HashConfig(tmpCfg)
	if err != nil {
		return err
	}

	// if config has not change skip update
	if h == s.currentConfigHash {
		return nil
	}
	// update config hash
	s.currentConfigHash = h

	logrus.Debugf("updating peer config to version %s", h)
	// update wireguard config
	if err := os.Rename(tmpCfg, wireguardConfigPath); err != nil {
		return err
	}

	// reload wireguard
	if err := wg.RestartTunnel(ctx, s.getTunnelName()); err != nil {
		return err
	}

	return nil
}
