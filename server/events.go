package server

import (
	"context"
	"fmt"

	"github.com/ehazlett/heimdall/wg"
	"github.com/gomodule/redigo/redis"
)

func (s *Server) eventHandler(ctx context.Context, m redis.Message) error {
	switch m.Channel {
	case nodeEventRestartTunnelKey:
		if err := wg.RestartTunnel(ctx, s.getTunnelName()); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown event on channel %s", m.Channel)
	}

	return nil
}
