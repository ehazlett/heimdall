package server

import (
	"context"

	v1 "github.com/ehazlett/heimdall/api/v1"
	"github.com/gogo/protobuf/proto"
	"github.com/gomodule/redigo/redis"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var (
	// ErrInvalidAuth is returned when an invalid cluster key is specified upon connect
	ErrInvalidAuth = errors.New("invalid cluster key specified")
	// ErrNoMaster is returned if there is no configured master yet
	ErrNoMaster = errors.New("no configured master")
)

// Join is called when a peer wants to join the cluster
func (s *Server) Join(ctx context.Context, req *v1.JoinRequest) (*v1.JoinResponse, error) {
	logrus.Debugf("join request from %s", req.ID)
	key, err := s.getClusterKey(ctx)
	if err != nil {
		return nil, err
	}
	if req.ClusterKey != key {
		return nil, ErrInvalidAuth
	}
	data, err := redis.Bytes(s.local(ctx, "GET", masterKey))
	if err != nil {
		if err == redis.ErrNil {
			return nil, ErrNoMaster
		}
		return nil, err
	}
	var master v1.Master
	if err := proto.Unmarshal(data, &master); err != nil {
		return nil, errors.Wrap(err, "error unmarshalling master info")
	}

	peers, err := s.getPeers(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error getting cluster peers")
	}

	if err := s.ensureNetworkSubnet(ctx, req.ID); err != nil {
		return nil, err
	}

	node, err := s.getNode(ctx, req.ID)
	if err != nil {
		if err != redis.ErrNil {
			return nil, errors.Wrapf(err, "error getting node info from redis for %s", req.ID)
		}
		n, err := s.createNode(ctx, req)
		if err != nil {
			return nil, errors.Wrap(err, "error creating node")
		}

		if err := s.updatePeerInfo(ctx, req.ID, req.Name); err != nil {
			return nil, errors.Wrap(err, "error updating peer info")
		}

		node = n
	}

	return &v1.JoinResponse{
		Master: &master,
		Node:   node,
		Peers:  peers,
	}, nil
}
