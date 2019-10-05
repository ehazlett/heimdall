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

package client

import (
	"context"

	v1 "github.com/stellarproject/heimdall/api/v1"
)

// Routes returns the known routes
func (c *Client) Routes() ([]*v1.Route, error) {
	ctx := context.Background()
	resp, err := c.heimdallClient.Routes(ctx, &v1.RoutesRequest{})
	if err != nil {
		return nil, err
	}

	return resp.Routes, nil
}

// CreateRoute creates a new route via the specified node ID
func (c *Client) CreateRoute(nodeID, network string) error {
	ctx := context.Background()
	if _, err := c.heimdallClient.CreateRoute(ctx, &v1.CreateRouteRequest{
		NodeID:  nodeID,
		Network: network,
	}); err != nil {
		return err
	}

	return nil
}

// DeleteRoute deletes a route
func (c *Client) DeleteRoute(network string) error {
	ctx := context.Background()
	if _, err := c.heimdallClient.DeleteRoute(ctx, &v1.DeleteRouteRequest{
		Network: network,
	}); err != nil {
		return err
	}

	return nil
}
