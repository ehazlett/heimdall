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

package client

import (
	"context"

	v1 "github.com/ehazlett/heimdall/api/v1"
)

// AuthorizePeer authorizes a new peer to the cluster
func (c *Client) AuthorizePeer(id string) error {
	ctx := context.Background()
	if _, err := c.heimdallClient.AuthorizePeer(ctx, &v1.AuthorizePeerRequest{
		ID: id,
	}); err != nil {
		return err
	}
	return nil
}

// DeauthorizePeer removes a peer from the cluster
func (c *Client) DeauthorizePeer(id string) error {
	ctx := context.Background()
	if _, err := c.heimdallClient.DeauthorizePeer(ctx, &v1.DeauthorizePeerRequest{
		ID: id,
	}); err != nil {
		return err
	}
	return nil
}

// AuthorizedPeers returns a list of authorized peers
func (c *Client) AuthorizedPeers() ([]string, error) {
	ctx := context.Background()
	resp, err := c.heimdallClient.AuthorizedPeers(ctx, &v1.AuthorizedPeersRequest{})
	if err != nil {
		return nil, err
	}
	return resp.IDs, nil
}

// Connect requests to connect a peer to the cluster
func (c *Client) Connect() (*v1.ConnectResponse, error) {
	ctx := context.Background()
	return c.heimdallClient.Connect(ctx, &v1.ConnectRequest{
		ID: c.id,
	})
}
