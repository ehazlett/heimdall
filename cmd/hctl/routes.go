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

package main

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	v1 "github.com/ehazlett/heimdall/api/v1"
	"github.com/urfave/cli"
)

var routesCommand = cli.Command{
	Name:  "routes",
	Usage: "route management",
	Subcommands: []cli.Command{
		listRoutesCommand,
		createRouteCommand,
		deleteRouteCommand,
	},
}

var listRoutesCommand = cli.Command{
	Name:  "list",
	Usage: "list routes",
	Action: func(cx *cli.Context) error {
		c, err := getClient(cx)
		if err != nil {
			return err
		}
		defer c.Close()

		ctx := context.Background()

		resp, err := c.Routes(ctx, &v1.RoutesRequest{})
		if err != nil {
			return err
		}

		w := tabwriter.NewWriter(os.Stdout, 20, 1, 3, ' ', 0)
		fmt.Fprintf(w, "NODE\tNETWORK\n")
		for _, r := range resp.Routes {
			fmt.Fprintf(w, "%s\t%s\n", r.NodeID, r.Network)
		}
		w.Flush()

		return nil
	},
}

var createRouteCommand = cli.Command{
	Name:  "create",
	Usage: "create a new route",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "node-id",
			Usage: "node id for route",
		},
		cli.StringFlag{
			Name:  "network",
			Usage: "network for route (i.e. 10.100.0.0/24)",
		},
	},
	Action: func(cx *cli.Context) error {
		c, err := getClient(cx)
		if err != nil {
			return err
		}
		defer c.Close()

		nodeID := cx.String("node-id")
		network := cx.String("network")

		if nodeID == "" || network == "" {
			return fmt.Errorf("node-id and network must be specified")
		}

		ctx := context.Background()

		if _, err := c.CreateRoute(ctx, &v1.CreateRouteRequest{
			NodeID:  nodeID,
			Network: network,
		}); err != nil {
			return err
		}
		return nil
	},
}

var deleteRouteCommand = cli.Command{
	Name:  "delete",
	Usage: "delete a route",
	Action: func(cx *cli.Context) error {
		c, err := getClient(cx)
		if err != nil {
			return err
		}
		defer c.Close()

		ctx := context.Background()

		network := cx.Args().First()
		if network == "" {
			return fmt.Errorf("network must be specified")
		}

		if _, err := c.DeleteRoute(ctx, &v1.DeleteRouteRequest{
			Network: network,
		}); err != nil {
			return err
		}
		return nil
	},
}
