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
	"fmt"
	"os"
	"text/tabwriter"

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

		routes, err := c.Routes()
		if err != nil {
			return err
		}

		w := tabwriter.NewWriter(os.Stdout, 20, 1, 3, ' ', 0)
		fmt.Fprintf(w, "NODE\tNETWORK\n")
		for _, r := range routes {
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

		if err := c.CreateRoute(cx.String("node-id"), cx.String("network")); err != nil {
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

		network := cx.Args().First()
		if err := c.DeleteRoute(network); err != nil {
			return err
		}
		return nil
	},
}
