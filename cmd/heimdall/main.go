package main

import (
	"fmt"
	"os"

	"github.com/ehazlett/heimdall"
	"github.com/ehazlett/heimdall/version"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

const (
	defaultGRPCPort = 9000
)

func main() {
	app := cli.NewApp()
	app.Name = version.Name
	app.Version = version.BuildVersion()
	app.Author = "@ehazlett"
	app.Email = ""
	app.Usage = version.Description
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "debug, D",
			Usage: "enable debug logging",
		},
		cli.StringFlag{
			Name:   "id",
			Usage:  "node id",
			Value:  heimdall.NodeID(),
			EnvVar: "HEIMDALL_NODE_ID",
		},
		cli.StringFlag{
			Name:   "name",
			Usage:  "node name",
			Value:  getHostname(),
			EnvVar: "HEIMDALL_NODE_NAME",
		},
		cli.StringFlag{
			Name:   "data-dir",
			Usage:  "dir for local node state",
			Value:  "/var/lib/heimdall",
			EnvVar: "HEIMDALL_DATA_DIR",
		},
		cli.IntFlag{
			Name:   "redis-port",
			Usage:  "port to use for the managed Redis server",
			Value:  16379,
			EnvVar: "HEIMDALL_REDIS_PORT",
		},
		cli.StringFlag{
			Name:   "addr, a",
			Usage:  "grpc address",
			Value:  fmt.Sprintf("tcp://%s:%d", heimdall.GetIP(), defaultGRPCPort),
			EnvVar: "HEIMDALL_GRPC_ADDR",
		},
		cli.StringFlag{
			Name:   "advertise-grpc-address",
			Usage:  "public advertise grpc address",
			Value:  fmt.Sprintf("tcp://%s:%d", heimdall.GetIP(), defaultGRPCPort),
			EnvVar: "HEIMDALL_ADVERTISE_GRPC_ADDR",
		},
		cli.StringFlag{
			Name:   "peer",
			Usage:  "grpc address to join a peer",
			EnvVar: "HEIMDALL_PEER",
		},
		cli.StringFlag{
			Name:   "cluster-key",
			Usage:  "preshared key for cluster peer joins",
			Value:  generateKey(),
			EnvVar: "HEIMDALL_CLUSTER_KEY",
		},
		cli.StringFlag{
			Name:   "node-interface",
			Usage:  "interface to use for network tunnel",
			Value:  "eth0",
			EnvVar: "HEIMDALL_NODE_INTERFACE",
		},
		cli.StringFlag{
			Name:   "node-network",
			Usage:  "subnet to be used for nodes",
			Value:  "10.10.0.0/16",
			EnvVar: "HEIMDALL_NODE_NETWORK",
		},
		cli.StringFlag{
			Name:   "peer-network",
			Usage:  "subnet to be used for peers",
			Value:  "10.51.0.0/16",
			EnvVar: "HEIMDALL_PEER_NETWORK",
		},
		cli.BoolFlag{
			Name:   "allow-peer-to-peer",
			Usage:  "allow peer to peer communication",
			EnvVar: "HEIMDALL_ALLOW_PEER_TO_PEER",
		},
		cli.StringFlag{
			Name:   "endpoint-ip",
			Usage:  "IP used for peer communication",
			Value:  heimdall.GetIP(),
			EnvVar: "HEIMDALL_ENDPOINT_IP",
		},
		cli.IntFlag{
			Name:   "endpoint-port",
			Usage:  "port for peer communication",
			Value:  10100,
			EnvVar: "HEIMDALL_ENDPOINT_PORT",
		},
		cli.StringFlag{
			Name:  "dns-address",
			Usage: "address for the DNS to listen",
			Value: "0.0.0.0:53",
		},
		cli.StringFlag{
			Name:  "dns-upstream-address",
			Usage: "address for the upstream DNS server",
			Value: "8.8.8.8:53",
		},
		cli.StringFlag{
			Name:   "interface-name",
			Usage:  "interface name to use for peer communication (must not exist)",
			Value:  "darknet",
			EnvVar: "HEIMDALL_INTERFACE_NAME",
		},
		cli.StringSliceFlag{
			Name:  "authorized-peer",
			Usage: "peer to authorize at startup",
			Value: &cli.StringSlice{},
		},
		cli.StringFlag{
			Name:  "cert, c",
			Usage: "heimdall server certificate",
			Value: "",
		},
		cli.StringFlag{
			Name:  "key, k",
			Usage: "heimdall server key",
			Value: "",
		},
		cli.StringFlag{
			Name:  "client-cert",
			Usage: "heimdall client certificate",
			Value: "",
		},
		cli.StringFlag{
			Name:  "client-key",
			Usage: "heimdall client key",
			Value: "",
		},
		cli.BoolFlag{
			Name:  "skip-verify",
			Usage: "skip TLS verification",
		},
	}
	app.Before = func(c *cli.Context) error {
		if c.Bool("debug") {
			log.SetLevel(log.DebugLevel)
		}

		return nil
	}
	app.Action = runServer

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func generateKey() string {
	return uuid.New().String()
}

func getHostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}
