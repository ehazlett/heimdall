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

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/stellarproject/heimdall"
	"github.com/stellarproject/heimdall/version"
	"github.com/urfave/cli"
)

const (
	defaultGRPCPort = 9000
)

func main() {
	app := cli.NewApp()
	app.Name = version.Name
	app.Version = version.BuildVersion()
	app.Author = "@stellarproject"
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
			Name:   "addr, a",
			Usage:  "grpc address",
			Value:  fmt.Sprintf("tcp://%s:%d", heimdall.GetIP(), defaultGRPCPort),
			EnvVar: "HEIMDALL_GRPC_ADDR",
		},
		cli.StringFlag{
			Name:   "redis-url, r",
			Usage:  "uri for datastore backend",
			Value:  "redis://127.0.0.1:6379/0",
			EnvVar: "HEIMDALL_REDIS_URL",
		},
		cli.StringFlag{
			Name:   "advertise-redis-url, p",
			Usage:  "advertise uri for peers",
			Value:  fmt.Sprintf("redis://%s:6379/0", heimdall.GetIP()),
			EnvVar: "HEIMDALL_ADVERTISE_REDIS_URL",
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
			Name:   "peer-network",
			Usage:  "subnet to be used for peers",
			Value:  "10.254.0.0/16",
			EnvVar: "HEIMDALL_PEER_NETWORK",
		},
		cli.StringFlag{
			Name:   "gateway-ip",
			Usage:  "IP used for peer communication",
			Value:  heimdall.GetIP(),
			EnvVar: "HEIMDALL_GATEWAY_IP",
		},
		cli.IntFlag{
			Name:   "gateway-port",
			Usage:  "port for peer communication",
			Value:  10100,
			EnvVar: "HEIMDALL_GATEWAY_PORT",
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
