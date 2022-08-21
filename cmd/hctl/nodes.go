package main

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	humanize "github.com/dustin/go-humanize"
	v1 "github.com/ehazlett/heimdall/api/v1"
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

		ctx := context.Background()

		resp, err := c.Nodes(ctx, &v1.NodesRequest{})
		if err != nil {
			return err
		}

		w := tabwriter.NewWriter(os.Stdout, 20, 1, 3, ' ', 0)
		fmt.Fprintf(w, "ID\tADDR\tENDPOINT\tGATEWAY\tUPDATED\tPUBLIC KEY\n")
		for _, n := range resp.Nodes {
			ep := fmt.Sprintf("%s:%d", n.EndpointIP, n.EndpointPort)
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", n.ID, n.Addr, ep, n.GatewayIP, humanize.Time(n.Updated), n.KeyPair.PublicKey)
		}
		w.Flush()

		return nil
	},
}
