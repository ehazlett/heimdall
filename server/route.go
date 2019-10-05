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

	"github.com/gogo/protobuf/proto"
	ptypes "github.com/gogo/protobuf/types"
	"github.com/gomodule/redigo/redis"
	"github.com/pkg/errors"
	v1 "github.com/stellarproject/heimdall/api/v1"
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
