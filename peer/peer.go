package peer

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/ehazlett/heimdall"
	"github.com/ehazlett/heimdall/client"
	"github.com/ehazlett/heimdall/version"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

const (
	wireguardConfigDir = "/etc/wireguard"
)

// Peer is the non-node peer
type Peer struct {
	cfg            *heimdall.PeerConfig
	currentVersion string
}

// NewPeer returns a new peer
func NewPeer(cfg *heimdall.PeerConfig) (*Peer, error) {
	return &Peer{
		cfg: cfg,
	}, nil
}

// Run starts the peer
func (p *Peer) Run() error {
	// initial sync
	logrus.Infof("connecting to peer %s", p.cfg.Address)
	ctx, cancel := context.WithTimeout(context.Background(), p.cfg.UpdateInterval)
	if err := p.sync(ctx); err != nil {
		cancel()
		return err
	}
	cancel()

	doneCh := make(chan bool)
	errCh := make(chan error)

	t := time.NewTicker(p.cfg.UpdateInterval)
	go func() {
		for range t.C {
			ctx, cancel := context.WithTimeout(context.Background(), p.cfg.UpdateInterval)
			if err := p.sync(ctx); err != nil {
				errCh <- err
				cancel()
				return
			}
			cancel()
		}
	}()
	select {
	case <-doneCh:
	case err := <-errCh:
		return err
	}
	return nil
}

// Stop stops the peer
func (p *Peer) Stop() error {
	// TODO
	return nil
}

func (p *Peer) getWireguardConfigPath() string {
	return filepath.Join(wireguardConfigDir, p.cfg.InterfaceName+".conf")
}

func (p *Peer) getTunnelName() string {
	return p.cfg.InterfaceName
}

func (p *Peer) getClient(addr string) (*client.Client, error) {
	cfg := &heimdall.Config{
		TLSClientCertificate:  p.cfg.TLSClientCertificate,
		TLSClientKey:          p.cfg.TLSClientKey,
		TLSInsecureSkipVerify: p.cfg.TLSInsecureSkipVerify,
	}

	opts, err := client.DialOptionsFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	opts = append(opts,
		grpc.WithBlock(),
		grpc.WithUserAgent(fmt.Sprintf("%s/%s", version.Name, version.Version)),
	)

	return client.NewClient(p.cfg.ID, addr, opts...)
}
