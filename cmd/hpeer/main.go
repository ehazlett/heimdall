package main

import (
	"fmt"
	"os"
	"time"

	"github.com/ehazlett/heimdall"
	"github.com/ehazlett/heimdall/version"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func main() {
	cli.VersionPrinter = func(c *cli.Context) {
		fmt.Printf("%v version=%s id=%s\n", c.App.Name, c.App.Version, heimdall.NodeID())
	}
	app := cli.NewApp()
	app.Name = "hpeer"
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
			Name:   "id",
			Usage:  "peer id for cluster",
			Value:  heimdall.NodeID(),
			EnvVar: "HEIMDALL_ID",
		},
		cli.StringFlag{
			Name:   "name",
			Usage:  "peer name",
			Value:  getHostname(),
			EnvVar: "HEIMDALL_NAME",
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

func getHostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}
