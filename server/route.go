package server

import (
	"context"

	v1 "github.com/ehazlett/heimdall/api/v1"
	"github.com/gogo/protobuf/proto"
	ptypes "github.com/gogo/protobuf/types"
	"github.com/gomodule/redigo/redis"
	"github.com/pkg/errors"
)

// CreateRoute reserves a new route
func (s *Server) CreateRoute(ctx context.Context, req *v1.CreateRouteRequest) (*ptypes.Empty, error) {
	// check for existing route
	routeKey := s.getRouteKey(req.Network)
	routeData, err := redis.Bytes(s.local(ctx, "GET", routeKey))
	if err != nil {
		if err != redis.ErrNil {
			return nil, err
		}
	}
	if routeData != nil {
		return nil, errors.Wrap(ErrRouteExists, req.Network)
	}

	// check for node id
	if _, err := redis.Bytes(s.local(ctx, "GET", s.getNodeKey(req.NodeID))); err != nil {
		if err == redis.ErrNil {
			return nil, errors.Wrap(ErrNodeDoesNotExist, req.NodeID)
		}
		return nil, err
	}

	// save route
	route := &v1.Route{
		NodeID:  req.NodeID,
		Network: req.Network,
	}

	data, err := proto.Marshal(route)
	if err != nil {
		return nil, err
	}
	if _, err := s.master(ctx, "SET", routeKey, data); err != nil {
		return nil, err
	}

	return empty, nil
}

// Delete deletes a new route
func (s *Server) DeleteRoute(ctx context.Context, req *v1.DeleteRouteRequest) (*ptypes.Empty, error) {
	routeKey := s.getRouteKey(req.Network)
	if _, err := s.master(ctx, "DEL", routeKey); err != nil {
		return nil, err
	}
	return empty, nil
}

// Routes returns a list of known routes
func (s *Server) Routes(ctx context.Context, req *v1.RoutesRequest) (*v1.RoutesResponse, error) {
	routes, err := s.getRoutes(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.RoutesResponse{
		Routes: routes,
	}, nil
}

func (s *Server) getRoutes(ctx context.Context) ([]*v1.Route, error) {
	routeKeys, err := redis.Strings(s.local(ctx, "KEYS", s.getRouteKey("*")))
	if err != nil {
		return nil, err
	}
	var routes []*v1.Route
	for _, routeKey := range routeKeys {
		data, err := redis.Bytes(s.local(ctx, "GET", routeKey))
		if err != nil {
			return nil, err
		}

		var route v1.Route
		if err := proto.Unmarshal(data, &route); err != nil {
			return nil, err
		}
		routes = append(routes, &route)
	}

	return routes, nil
}
