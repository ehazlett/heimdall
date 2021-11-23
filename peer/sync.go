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

package peer

import (
	"context"
	"os"

	"github.com/ehazlett/heimdall"
	v1 "github.com/ehazlett/heimdall/api/v1"
	"github.com/ehazlett/heimdall/wg"
	"github.com/sirupsen/logrus"
)

func (p *Peer) sync(ctx context.Context) error {
	logrus.Debugf("syncing with node %s", p.cfg.Address)
	c, err := p.getClient(p.cfg.Address)
	if err != nil {
		return err
	}
	defer c.Close()

	resp, err := c.Connect()
	if err != nil {
		return err
	}

	peers := []*v1.Peer{}
	for _, peer := range resp.Peers {
		// don't add self
		if peer.ID == p.cfg.ID {
			continue
		}
		peers = append(peers, peer)
	}

	// generate wireguard config
	wireguardCfg := &wg.Config{
		Interface:  p.cfg.InterfaceName,
		Address:    resp.Address,
		PrivateKey: resp.KeyPair.PrivateKey,
		Peers:      peers,
		DNS:        resp.DNS,
	}

	wireguardConfigPath := p.getWireguardConfigPath()
	tmpCfg, err := wg.GeneratePeerConfig(wireguardCfg, wireguardConfigPath)
	if err != nil {
		return err
	}

	h, err := heimdall.HashConfig(tmpCfg)
	if err != nil {
		return err
	}

	// if config has not change skip update
	if h == p.currentVersion {
		return nil
	}
	p.currentVersion = h

	logrus.Debugf("updating peer config to version %s", h)
	// update wireguard config
	if err := os.Rename(tmpCfg, wireguardConfigPath); err != nil {
		return err
	}

	// reload wireguard
	if err := wg.RestartTunnel(ctx, p.getTunnelName()); err != nil {
		return err
	}

	return nil
}
