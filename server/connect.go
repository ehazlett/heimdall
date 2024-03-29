package server

import (
	"context"
	"fmt"
	"strings"

	v1 "github.com/ehazlett/heimdall/api/v1"
	ptypes "github.com/gogo/protobuf/types"
	"github.com/gomodule/redigo/redis"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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
	if _, err := s.master(ctx, "DEL", s.getPeerKey(req.ID)); err != nil {
		return nil, err
	}
	// notify to restart tunnels
	if _, err := s.master(ctx, "PUBLISH", nodeEventRestartTunnelKey, "1"); err != nil {
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
	if err := s.updatePeerInfo(ctx, req.ID, req.Name); err != nil {
		return nil, err
	}

	subnetParts := strings.Split(s.cfg.PeerNetwork, "/")
	subnetCIDR := subnetParts[1]
	// save peer
	return &v1.ConnectResponse{
		KeyPair: keyPair,
		Address: fmt.Sprintf("%s/%s", ip.String(), subnetCIDR),
		Peers:   peers,
		DNS:     dnsAddrs,
	}, nil
}
