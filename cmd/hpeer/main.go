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
	"time"

	"github.com/sirupsen/logrus"
	"github.com/ehazlett/heimdall"
	"github.com/ehazlett/heimdall/version"
	"github.com/urfave/cli"
)

func main() {
	cli.VersionPrinter = func(c *cli.Context) {
		fmt.Printf("%v version=%s id=%s\n", c.App.Name, c.App.Version, heimdall.NodeID())
	}
	app := cli.NewApp()
	app.Name = "hpeer"
	app.Version = version.BuildVersion()
	app.Author = "@stellarproject"
	app.Email = ""
	app.Usage = version.Description
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "debug, D",
			Usage: "Enable debug logging",
		},
		cli.StringFlag{
			Name:   "id",
			Usage:  "peer id for cluster",
			Value:  heimdall.NodeID(),
			EnvVar: "HEIMDALL_ID",
		},
		cli.StringFlag{
			Name:   "addr",
			Usage:  "heimdall peer address to join",
			Value:  "tcp://127.0.0.1:9000",
			EnvVar: "HEIMDALL_ADDR",
		},
		cli.DurationFlag{
			Name:   "update-interval",
			Usage:  "interval in which to update with the cluster",
			Value:  time.Second * 10,
			EnvVar: "HEIMDALL_UPDATE_INTERVAL",
		},
		cli.StringFlag{
			Name:   "interface-name",
			Usage:  "interface name to use for peer communication (must not exist)",
			Value:  "darknet",
			EnvVar: "HEIMDALL_INTERFACE_NAME",
		},
		cli.StringFlag{
			Name:  "cert, c",
			Usage: "heimdall client certificate",
			Value: "",
		},
		cli.StringFlag{
			Name:  "key, k",
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
			logrus.SetLevel(logrus.DebugLevel)
		}
		return nil
	}
	app.Action = run

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}
