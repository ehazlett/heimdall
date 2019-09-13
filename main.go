package main

import (
	"os"

	"github.com/ehazlett/gatekeeper/version"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	app.Name = version.Name
	app.Author = "@darknet"
	app.Description = version.Description
	app.Version = version.BuildVersion()
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "debug, D",
			Usage: "enable debug logging",
		},
		cli.IntFlag{
			Name:  "port, p",
			Usage: "listen port",
			Value: 2222,
		},
		cli.StringFlag{
			Name:  "key-dir, d",
			Usage: "path to authorized public keys",
			Value: "",
		},
		cli.StringFlag{
			Name:  "host-key, k",
			Usage: "path to host key",
			Value: "/etc/ssh/ssh_host_rsa_key",
		},
		cli.StringFlag{
			Name:  "subnet, s",
			Usage: "subnet for ip allocation",
			Value: "10.199.254.0/24",
		},
		cli.StringFlag{
			Name:  "redis, r",
			Usage: "redis url",
			Value: "redis://127.0.0.1:6379",
		},
		cli.StringFlag{
			Name:  "guard-addr",
			Usage: "guard server address",
			Value: "10.199.199.1:10100",
		},
		cli.StringFlag{
			Name:  "guard-tunnel",
			Usage: "guard tunnel to use for peers",
			Value: "guard0",
		},
		cli.StringFlag{
			Name:  "guard-dns",
			Usage: "dns to use for peers",
			Value: "10.199.254.1",
		},
	}
	app.Before = func(cx *cli.Context) error {
		if cx.Bool("debug") {
			logrus.SetLevel(logrus.DebugLevel)
		}
		return nil
	}
	app.Action = func(cx *cli.Context) error {
		cfg := &ServerConfig{
			ListenPort:  cx.Int("port"),
			KeysPath:    cx.String("key-dir"),
			HostKeyPath: cx.String("host-key"),
			RedisURL:    cx.String("redis"),
			Subnet:      cx.String("subnet"),
			GuardAddr:   cx.String("guard-addr"),
			GuardTunnel: cx.String("guard-tunnel"),
			GuardDNS:    cx.String("guard-dns"),
		}
		srv, err := NewServer(cfg)
		if err != nil {
			return err
		}
		return srv.Run()
	}

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}
