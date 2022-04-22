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
	"fmt"
	"os"

	"github.com/ehazlett/heimdall"
	"github.com/ehazlett/heimdall/client"
	"github.com/ehazlett/heimdall/version"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"google.golang.org/grpc"
)

func main() {
	app := cli.NewApp()
	app.Name = "hctl"
	app.Version = version.BuildVersion()
	app.Author = "@ehazlett"
	app.Email = ""
	app.Usage = version.Description
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "debug, D",
			Usage: "Enable debug logging",
		},
		cli.StringFlag{
			Name:   "addr, a",
			Usage:  "heimdall grpc address",
			Value:  "tcp://127.0.0.1:9000",
			EnvVar: "HEIMDALL_ADDR",
		},
		cli.StringFlag{
			Name:   "cert, c",
			Usage:  "heimdall client certificate",
			Value:  "",
			EnvVar: "HEIMDALL_CLIENT_CERT",
		},
		cli.StringFlag{
			Name:   "key, k",
			Usage:  "heimdall client key",
			Value:  "",
			EnvVar: "HEIMDALL_CLIENT_KEY",
		},
		cli.BoolFlag{
			Name:   "skip-verify",
			Usage:  "skip TLS verification",
			EnvVar: "HEIMDALL_SKIP_VERIFY",
		},
	}
	app.Before = func(c *cli.Context) error {
		if c.Bool("debug") {
			logrus.SetLevel(logrus.DebugLevel)
		}
		return nil
	}
	app.Commands = []cli.Command{
		nodesCommand,
		peersCommand,
		routesCommand,
	}

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}

func getClient(c *cli.Context) (*client.Client, error) {
	cert := c.GlobalString("cert")
	key := c.GlobalString("key")
	skipVerification := c.GlobalBool("skip-verify")

	cfg := &heimdall.Config{
		TLSClientCertificate:  cert,
		TLSClientKey:          key,
		TLSInsecureSkipVerify: skipVerification,
	}

	opts, err := client.DialOptionsFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	opts = append(opts,
		grpc.WithBlock(),
		grpc.WithUserAgent(fmt.Sprintf("%s/%s", version.Name, version.Version)),
	)

	addr := c.GlobalString("addr")
	return client.NewClient(heimdall.NodeID(), addr, opts...)
}
