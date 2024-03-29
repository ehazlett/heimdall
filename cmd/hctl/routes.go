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
