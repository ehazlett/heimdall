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
	"errors"

	ptypes "github.com/gogo/protobuf/types"
	"github.com/gomodule/redigo/redis"
	"github.com/sirupsen/logrus"
	v1 "github.com/stellarproject/heimdall/api/v1"
)

var (
	// ErrAccessDenied is returned when an unauthorized non-node peer attempts to join
	ErrAccessDenied = errors.New("access denied")
)

func (s *Server) AuthorizedPeers(ctx context.Context, req *v1.AuthorizedPeersRequest) (*v1.AuthorizedPeersResponse, error) {
	authorized, err := redis.Strings(s.local(ctx, "SMEMBERS", authorizedPeersKey))
	if err != nil {
		return nil, err
	}
	return &v1.AuthorizedPeersResponse{
		IDs: authorized,
	}, nil
}

// AuthorizePeer authorizes a peer to the cluster
func (s *Server) AuthorizePeer(ctx context.Context, req *v1.AuthorizePeerRequest) (*ptypes.Empty, error) {
	logrus.Debugf("authorizing peer %s", req.ID)
	if _, err := s.master(ctx, "SADD", authorizedPeersKey, req.ID); err != nil {
		return nil, err
	}
	logrus.Infof("authorized peer %s", req.ID)
	return empty, nil
}

// DeauthorizePeer deauthorizes a peer from the cluster
func (s *Server) DeauthorizePeer(ctx context.Context, req *v1.DeauthorizePeerRequest) (*ptypes.Empty, error) {
	logrus.Debugf("deauthorizing peer %s", req.ID)
	if _, err := s.master(ctx, "SREM", authorizedPeersKey, req.ID); err != nil {
		return nil, err
	}
	logrus.Infof("deauthorized peer %s", req.ID)
	return empty, nil
}

// Connect is called when a non-node peer wants to connect to the cluster
func (s *Server) Connect(ctx context.Context, req *v1.ConnectRequest) (*v1.ConnectResponse, error) {
	authorized, err := redis.Bool(s.local(ctx, "SISMEMBER", authorizedPeersKey, req.ID))
	if err != nil {
		return nil, err
	}
	if !authorized {
		logrus.Warnf("unauthorized request attempt from %s", req.ID)
		return nil, ErrAccessDenied
	}
	keyPair, err := s.getOrCreateKeyPair(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	nodes, err := s.getNodes(ctx)
	if err != nil {
		return nil, err
	}
	dnsAddrs := []string{}
	for _, n := range nodes {
		dnsAddrs = append(dnsAddrs, n.GatewayIP)
	}

	peers, err := s.getPeers(ctx)
	if err != nil {
		return nil, err
	}
	ip, _, err := s.getOrAllocatePeerIP(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	if err := s.updatePeerInfo(ctx, req.ID); err != nil {
		return nil, err
	}
	// save peer
	return &v1.ConnectResponse{
		KeyPair: keyPair,
		Address: ip.String() + "/32",
		Peers:   peers,
		DNS:     dnsAddrs,
	}, nil
}
