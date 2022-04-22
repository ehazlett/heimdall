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

package main

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	v1 "github.com/ehazlett/heimdall/api/v1"
	"github.com/urfave/cli"
)

var peersCommand = cli.Command{
	Name:  "peers",
	Usage: "peer management",
	Subcommands: []cli.Command{
		listPeersCommand,
		authorizedPeersCommand,
		authorizePeerCommand,
		deauthorizePeerCommand,
	},
}

var listPeersCommand = cli.Command{
	Name:  "list",
	Usage: "list peers",
	Action: func(cx *cli.Context) error {
		c, err := getClient(cx)
		if err != nil {
			return err
		}
		defer c.Close()

		ctx := context.Background()

		resp, err := c.Peers(ctx, &v1.PeersRequest{})
		if err != nil {
			return err
		}

		w := tabwriter.NewWriter(os.Stdout, 20, 1, 3, ' ', 0)
		fmt.Fprintf(w, "ID\tPUBLIC KEY\tENDPOINT\tALLOWED\tPEER IP\n")
		for _, p := range resp.Peers {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", p.ID, p.KeyPair.PublicKey, p.Endpoint, p.AllowedIPs, p.PeerIP)
		}
		w.Flush()

		return nil
	},
}

var authorizedPeersCommand = cli.Command{
	Name:  "authorized",
	Usage: "authorized peers in the cluster",
	Action: func(cx *cli.Context) error {
		c, err := getClient(cx)
		if err != nil {
			return err
		}
		defer c.Close()

		ctx := context.Background()

		resp, err := c.AuthorizedPeers(ctx, &v1.AuthorizedPeersRequest{})
		if err != nil {
			return err
		}
		w := tabwriter.NewWriter(os.Stdout, 20, 1, 3, ' ', 0)
		fmt.Fprintf(w, "ID\n")
		for _, id := range resp.IDs {
			fmt.Fprintf(w, "%s\n", id)
		}
		w.Flush()
		return nil
	},
}

var authorizePeerCommand = cli.Command{
	Name:  "authorize",
	Usage: "authorize peer to cluster",
	Action: func(cx *cli.Context) error {
		c, err := getClient(cx)
		if err != nil {
			return err
		}
		defer c.Close()

		ctx := context.Background()

		id := cx.Args().First()
		if id == "" {
			return fmt.Errorf("ID cannot be empty")
		}
		if _, err := c.AuthorizePeer(ctx, &v1.AuthorizePeerRequest{
			ID: id,
		}); err != nil {
			return err
		}
		return nil
	},
}

var deauthorizePeerCommand = cli.Command{
	Name:  "deauthorize",
	Usage: "deauthorize peer from cluster",
	Action: func(cx *cli.Context) error {
		c, err := getClient(cx)
		if err != nil {
			return err
		}
		defer c.Close()

		ctx := context.Background()

		id := cx.Args().First()
		if id == "" {
			return fmt.Errorf("ID cannot be empty")
		}
		if _, err := c.DeauthorizePeer(ctx, &v1.DeauthorizePeerRequest{
			ID: id,
		}); err != nil {
			return err
		}
		return nil
	},
}
