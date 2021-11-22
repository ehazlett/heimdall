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
	"crypto/tls"
	"net"
	"net/url"
	"time"

	"github.com/ehazlett/heimdall"
	v1 "github.com/ehazlett/heimdall/api/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// Client is the Atlas client
type Client struct {
	id             string
	conn           *grpc.ClientConn
	heimdallClient v1.HeimdallClient
}

// NewClient returns a new Atlas client configured with the specified address and options
func NewClient(id, addr string, opts ...grpc.DialOption) (*Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()

	if len(opts) == 0 {
		opts = []grpc.DialOption{
			grpc.WithInsecure(),
			grpc.WithBlock(),
		}
	}

	u, err := url.Parse(addr)
	if err != nil {
		return nil, err
	}

	endpoint := u.Host

	if u.Scheme == "unix" {
		endpoint = u.Path
		opts = append(opts, grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}))
	}

	opts = append(opts, grpc.WithDefaultCallOptions(
		grpc.WaitForReady(true),
	))
	c, err := grpc.DialContext(ctx,
		endpoint,
		opts...,
	)
	if err != nil {
		return nil, err
	}

	client := &Client{
		id:             id,
		conn:           c,
		heimdallClient: v1.NewHeimdallClient(c),
	}

	return client, nil
}

// Conn returns the current configured client connection
func (c *Client) Conn() *grpc.ClientConn {
	return c.conn
}

// Close closes the underlying GRPC client
func (c *Client) Close() error {
	return c.conn.Close()
}

// DialOptionsFromConfig returns dial options configured from a Stellar config
func DialOptionsFromConfig(cfg *heimdall.Config) ([]grpc.DialOption, error) {
	opts := []grpc.DialOption{}
	if cfg.TLSClientCertificate != "" {
		var creds credentials.TransportCredentials
		if cfg.TLSClientKey != "" {
			cert, err := tls.LoadX509KeyPair(cfg.TLSClientCertificate, cfg.TLSClientKey)
			if err != nil {
				return nil, err
			}
			creds = credentials.NewTLS(&tls.Config{
				Certificates:       []tls.Certificate{cert},
				InsecureSkipVerify: cfg.TLSInsecureSkipVerify,
			})
		} else {
			c, err := credentials.NewClientTLSFromFile(cfg.TLSClientCertificate, "")
			if err != nil {
				return nil, err
			}
			creds = c
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithInsecure())
	}

	return opts, nil
}
