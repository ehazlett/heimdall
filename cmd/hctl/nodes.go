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

package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	humanize "github.com/dustin/go-humanize"
	"github.com/urfave/cli"
)

var nodesCommand = cli.Command{
	Name:  "nodes",
	Usage: "node management",
	Subcommands: []cli.Command{
		listNodesCommand,
	},
}

var listNodesCommand = cli.Command{
	Name:  "list",
	Usage: "list nodes",
	Action: func(cx *cli.Context) error {
		c, err := getClient(cx)
		if err != nil {
			return err
		}
		defer c.Close()

		nodes, err := c.Nodes()
		if err != nil {
			return err
		}

		w := tabwriter.NewWriter(os.Stdout, 20, 1, 3, ' ', 0)
		fmt.Fprintf(w, "ID\tADDR\tENDPOINT\tGATEWAY\tUPDATED\tPUBLIC KEY\n")
		for _, n := range nodes {
			ep := fmt.Sprintf("%s:%d", n.EndpointIP, n.EndpointPort)
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", n.ID, n.Addr, ep, n.GatewayIP, humanize.Time(n.Updated), n.KeyPair.PublicKey)
		}
		w.Flush()

		return nil
	},
}
