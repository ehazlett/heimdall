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

	resp, err := c.Connect(ctx, &v1.ConnectRequest{
		ID:   p.cfg.ID,
		Name: p.cfg.Name,
	})
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
