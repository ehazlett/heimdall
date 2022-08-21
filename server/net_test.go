package server

import (
	"context"
	"testing"

	"github.com/ehazlett/heimdall"
)

const (
	testPeerNetwork = "10.51.0.0/16"
	testNodeNetwork = "10.10.0.0/16"
)

func TestNetSuite(t *testing.T) {
	cfg := &heimdall.Config{
		ID:          "test",
		NodeNetwork: testNodeNetwork,
		PeerNetwork: testPeerNetwork,
		DataDir:     "/tmp/heimdall-test",
	}

	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		ctx := context.Background()
		if _, err := srv.master(ctx, "FLUSHDB"); err != nil {
			t.Errorf("error tearing down: %s", err)
		}
	}()

	// run tests
	t.Run("AllocatePeerIP", testNetAllocatePeerIP(srv))
}

func testNetAllocatePeerIP(s *Server) func(t *testing.T) {
	return func(t *testing.T) {
		ctx := context.Background()
		ip, ipnet, err := s.getOrAllocatePeerIP(ctx, "test-node")
		if err != nil {
			t.Fatal(err)
		}

		expectedIP := "10.51.0.2"

		if ip.String() != expectedIP {
			t.Errorf("expected ip %s; received %s", expectedIP, ip.String())
		}

		if ipnet.String() != testPeerNetwork {
			t.Errorf("expected net %s; received %s", testPeerNetwork, ipnet.String())
		}
	}
}
