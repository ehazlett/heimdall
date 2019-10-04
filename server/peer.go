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

func (s *Server) updatePeerInfo(ctx context.Context) error {
	keypair, err := s.getOrCreateKeyPair(ctx, s.cfg.ID)
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf("%s:%d", s.cfg.GatewayIP, s.cfg.GatewayPort)

	// TODO: build allowedIPs from routes and peer network
	allowedIPs := []string{s.cfg.PeerNetwork}

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
	key := s.getPeerKey(s.cfg.ID)
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

func (s *Server) updatePeerConfig(ctx context.Context) {
	logrus.Debugf("starting peer config updater: ttl=%s", peerConfigUpdateInterval)
	t := time.NewTicker(peerConfigUpdateInterval)

	configHash := ""

	for range t.C {
		uctx, cancel := context.WithTimeout(ctx, peerConfigUpdateInterval)
		peerKeys, err := redis.Strings(s.local(uctx, "KEYS", s.getPeerKey("*")))
		if err != nil {
			logrus.Error(err)
			cancel()
			continue
		}
		var peers []*v1.Peer
		for _, peerKey := range peerKeys {
			peerData, err := redis.Bytes(s.local(uctx, "GET", peerKey))
			if err != nil {
				logrus.Error(err)
				cancel()
				continue
			}
			var p v1.Peer
			if err := proto.Unmarshal(peerData, &p); err != nil {
				logrus.Error(err)
				cancel()
				continue
			}

			// do not add self as a peer
			if p.ID == s.cfg.ID {
				continue
			}

			peers = append(peers, &p)
		}

		keyPair, err := s.getOrCreateKeyPair(ctx, s.cfg.ID)
		if err != nil {
			logrus.Error(err)
			cancel()
			continue
		}

		gatewayIP, _, err := s.getOrAllocateIP(ctx, s.cfg.ID)
		if err != nil {
			logrus.Error(err)
			cancel()
			continue
		}
		wireguardCfg := &wireguardConfig{
			Iface:      defaultWireguardInterface,
			PrivateKey: keyPair.PrivateKey,
			ListenPort: s.cfg.GatewayPort,
			Address:    gatewayIP.String() + "/32",
			Peers:      peers,
		}

		tmpCfg, err := generateNodeWireguardConfig(wireguardCfg)
		if err != nil {
			logrus.Error(err)
			cancel()
			continue
		}

		h, err := hashConfig(tmpCfg)
		if err != nil {
			logrus.Error(err)
			cancel()
			continue
		}

		// if config has not change skip update
		if h == configHash {
			continue
		}

		logrus.Debugf("updating peer config to version %s", h)
		// update wireguard config
		if err := os.Rename(tmpCfg, wireguardConfigPath); err != nil {
			logrus.Error(err)
			cancel()
			continue
		}
		// reload wireguard
		if err := restartWireguardTunnel(ctx); err != nil {
			logrus.Error(err)
			cancel()
			continue
		}
		configHash = h
	}
}

func hashConfig(cfgPath string) (string, error) {
	peerData, err := ioutil.ReadFile(cfgPath)
	if err != nil {
		return "", err
	}

	h := sha256.New()
	h.Write(peerData)
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
