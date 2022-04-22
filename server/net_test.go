/*
	Copyright 2022 Evan Hazlett

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
