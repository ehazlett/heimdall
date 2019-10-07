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

package peer

import (
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stellarproject/heimdall"
	"github.com/stellarproject/heimdall/client"
)

const (
	wireguardConfigDir = "/etc/wireguard"
)

// Peer is the non-node peer
type Peer struct {
	cfg *heimdall.PeerConfig
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
	if err := p.sync(); err != nil {
		return err
	}

	doneCh := make(chan bool)
	errCh := make(chan error)

	t := time.NewTicker(p.cfg.UpdateInterval)
	go func() {
		for range t.C {
			if err := p.sync(); err != nil {
				errCh <- err
				return
			}
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
	return client.NewClient(p.cfg.ID, addr)
}
