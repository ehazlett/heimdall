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
	"io/ioutil"
	"os"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/gomodule/redigo/redis"
	"github.com/sirupsen/logrus"
	v1 "github.com/stellarproject/heimdall/api/v1"
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
		peers = append(peers, &peer)
	}
	return peers, nil
}

func (s *Server) peerUpdater(ctx context.Context) {
	logrus.Debugf("starting peer config updater: ttl=%s", peerConfigUpdateInterval)

	t := time.NewTicker(peerConfigUpdateInterval)

	for range t.C {
		uctx, cancel := context.WithTimeout(ctx, peerConfigUpdateInterval)
		if err := s.updatePeerInfo(uctx); err != nil {
			logrus.Errorf("updatePeerInfo: %s", err)
			cancel()
			continue
		}

		if err := s.updatePeerConfig(uctx); err != nil {
			logrus.Errorf("updatePeerConfig: %s", err)
			cancel()
			continue
		}
		cancel()
	}
}

func (s *Server) updatePeerInfo(ctx context.Context) error {
	keypair, err := s.getOrCreateKeyPair(ctx, s.cfg.ID)
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf("%s:%d", s.cfg.EndpointIP, s.cfg.EndpointPort)

	// build allowedIPs from routes and peer network
	allowedIPs := []string{}
	nodes, err := s.getNodes(ctx)
	if err != nil {
		return err
	}

	for _, node := range nodes {
		// only add the route if a peer to prevent route duplicate
		if node.ID != s.cfg.ID {
			continue
		}

		_, gatewayNet, err := s.getNodeIP(ctx, node.ID)
		if err != nil {
			return err
		}

		allowedIPs = append(allowedIPs, gatewayNet.String())
	}

	routes, err := s.getRoutes(ctx)
	if err != nil {
		return err
	}

	for _, route := range routes {
		// only add the route if a peer to prevent route duplicate
		if route.NodeID != s.cfg.ID {
			continue
		}

		logrus.Debugf("adding route to allowed IPs: %s", route.Network)
		allowedIPs = append(allowedIPs, route.Network)
	}

	n := &v1.Peer{
		ID:         s.cfg.ID,
		KeyPair:    keypair,
		AllowedIPs: allowedIPs,
		Endpoint:   endpoint,
	}

	data, err := proto.Marshal(n)
	if err != nil {
		return err
	}
	pHash := hashData(data)

	key := s.getPeerKey(s.cfg.ID)
	peerData, err := redis.Bytes(s.local(ctx, "GET", key))
	if err != nil {
		if err != redis.ErrNil {
			return err
		}
	}

	eHash := hashData(peerData)

	// skip update if same
	if pHash == eHash {
		return nil
	}

	if _, err := s.master(ctx, "SET", key, data); err != nil {
		return err
	}

	logrus.Debugf("peer info: endpoint=%s allowedips=%+v", n.Endpoint, n.Endpoint)

	return nil
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

func (s *Server) updatePeerConfig(ctx context.Context) error {
	peerKeys, err := redis.Strings(s.local(ctx, "KEYS", s.getPeerKey("*")))
	if err != nil {
		return err
	}
	var peers []*v1.Peer
	for _, peerKey := range peerKeys {
		peerData, err := redis.Bytes(s.local(ctx, "GET", peerKey))
		if err != nil {
			return err
		}
		var p v1.Peer
		if err := proto.Unmarshal(peerData, &p); err != nil {
			return err
		}

		// do not add self as a peer
		if p.ID == s.cfg.ID {
			continue
		}

		peers = append(peers, &p)
	}

	keyPair, err := s.getOrCreateKeyPair(ctx, s.cfg.ID)
	if err != nil {
		return err
	}

	gatewayIP, gatewayNet, err := s.getNodeIP(ctx, s.cfg.ID)
	if err != nil {
		return err
	}

	size, _ := gatewayNet.Mask.Size()
	wireguardCfg := &wireguardConfig{
		Iface:      defaultWireguardInterface,
		PrivateKey: keyPair.PrivateKey,
		ListenPort: s.cfg.EndpointPort,
		Address:    fmt.Sprintf("%s/%d", gatewayIP.To4().String(), size),
		Peers:      peers,
	}

	tmpCfg, err := generateNodeWireguardConfig(wireguardCfg)
	if err != nil {
		return err
	}

	h, err := hashConfig(tmpCfg)
	if err != nil {
		return err
	}

	e, err := hashConfig(wireguardConfigPath)
	if err != nil {
		return err
	}

	// if config has not change skip update
	if h == e {
		return nil
	}

	logrus.Debugf("updating peer config to version %s", h)
	// update wireguard config
	if err := os.Rename(tmpCfg, wireguardConfigPath); err != nil {
		return err
	}

	// reload wireguard
	if err := restartWireguardTunnel(ctx); err != nil {
		return err
	}

	return nil
}

func hashData(data []byte) string {
	h := sha256.New()
	h.Write(data)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func hashConfig(cfgPath string) (string, error) {
	if _, err := os.Stat(cfgPath); err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	peerData, err := ioutil.ReadFile(cfgPath)
	if err != nil {
		return "", err
	}

	return hashData(peerData), nil
}
