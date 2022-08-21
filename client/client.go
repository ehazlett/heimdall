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

// Client is the client
type Client struct {
	v1.HeimdallClient
	id   string
	conn *grpc.ClientConn
}

// NewClient returns a new client configured with the specified address and options
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
		v1.NewHeimdallClient(c),
		id,
		c,
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
