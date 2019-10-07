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

	"github.com/gogo/protobuf/proto"
	"github.com/gomodule/redigo/redis"
	"github.com/sirupsen/logrus"
	v1 "github.com/stellarproject/heimdall/api/v1"
)

var (
	// ErrInvalidAuth is returned when an invalid cluster key is specified upon connect
	ErrInvalidAuth = errors.New("invalid cluster key specified")
	// ErrNoMaster is returned if there is no configured master yet
	ErrNoMaster = errors.New("no configured master")
)

// Connect is called when a peer wants to connect to the node
func (s *Server) Connect(ctx context.Context, req *v1.ConnectRequest) (*v1.ConnectResponse, error) {
	logrus.Debugf("connect request from %s", req.ID)
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
		return nil, err
	}

	return &v1.ConnectResponse{
		Master: &master,
	}, nil
}
