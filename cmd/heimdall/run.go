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
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/ehazlett/heimdall"
	"github.com/ehazlett/heimdall/server"
	"github.com/ehazlett/heimdall/version"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func runServer(clix *cli.Context) error {
	cfg := &heimdall.Config{
		ID:                    clix.String("id"),
		Name:                  clix.String("name"),
		DataDir:               clix.String("data-dir"),
		RedisPort:             clix.Int("redis-port"),
		GRPCAddress:           clix.String("addr"),
		AdvertiseGRPCAddress:  clix.String("advertise-grpc-address"),
		GRPCPeerAddress:       clix.String("peer"),
		ClusterKey:            clix.String("cluster-key"),
		NodeNetwork:           clix.String("node-network"),
		NodeInterface:         clix.String("node-interface"),
		PeerNetwork:           clix.String("peer-network"),
		EndpointIP:            clix.String("endpoint-ip"),
		EndpointPort:          clix.Int("endpoint-port"),
		AllowPeerToPeer:       clix.Bool("allow-peer-to-peer"),
		DNSServerAddress:      clix.String("dns-address"),
		DNSUpstreamAddress:    clix.String("dns-upstream-address"),
		InterfaceName:         clix.String("interface-name"),
		AuthorizedPeers:       clix.StringSlice("authorized-peer"),
		TLSServerCertificate:  clix.String("cert"),
		TLSServerKey:          clix.String("key"),
		TLSClientCertificate:  clix.String("client-cert"),
		TLSClientKey:          clix.String("client-key"),
		TLSInsecureSkipVerify: clix.Bool("skip-verify"),
	}

	errCh := make(chan error, 1)

	srv, err := server.NewServer(cfg)
	if err != nil {
		return err
	}

	// create grpc server
	grpcOpts, err := getGRPCOptions(cfg)
	if err != nil {
		return err
	}
	grpcServer := grpc.NewServer(grpcOpts...)

	// register heimdall server
	if err := srv.Register(grpcServer); err != nil {
		return err
	}

	proto, ep, err := getGRPCEndpoint(cfg.GRPCAddress)
	if err != nil {
		return err
	}
	l, err := net.Listen(proto, ep)
	if err != nil {
		return err
	}
	defer l.Close()

	logrus.WithField("addr", cfg.GRPCAddress).Debug("starting grpc server")
	go grpcServer.Serve(l)

	signals := make(chan os.Signal)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1)
	doneCh := make(chan bool, 1)
	go func() {
		for {
			select {
			case sig := <-signals:
				switch sig {
				case syscall.SIGUSR1:
					logrus.Debug("generating debug profile")
					profilePath, err := srv.GenerateProfile()
					if err != nil {
						logrus.Error(err)
						continue
					}
					logrus.WithFields(logrus.Fields{
						"profile": profilePath,
					}).Info("generated memory profile")
				case syscall.SIGTERM, syscall.SIGINT:
					logrus.Info("shutting down")
					if err := srv.Stop(); err != nil {
						errCh <- err
					}
					doneCh <- true
				default:
					logrus.Warnf("unhandled signal %s", sig)
				}
			}
		}
	}()

	logrus.WithFields(logrus.Fields{
		"version": version.Version,
		"commit":  version.GitCommit,
	}).Infof("starting %s", version.Name)
	go func() {
		if err := srv.Run(); err != nil {
			errCh <- err
		}
	}()

	select {
	case <-doneCh:
		return nil
	case err := <-errCh:
		return err
	}
}

func getGRPCEndpoint(addr string) (string, string, error) {
	u, err := url.Parse(addr)
	if err != nil {
		return "", "", err
	}
	// only tcp/unix are allowed
	var ep string
	switch u.Scheme {
	case "tcp":
		ep = u.Host
	case "unix":
		ep = u.Path
	default:
		return "", "", fmt.Errorf("unsupported grpc listener protocol: %s", u.Scheme)
	}

	return u.Scheme, ep, nil
}

func getGRPCOptions(cfg *heimdall.Config) ([]grpc.ServerOption, error) {
	grpcOpts := []grpc.ServerOption{}
	if cfg.TLSServerCertificate != "" && cfg.TLSServerKey != "" {
		logrus.WithFields(logrus.Fields{
			"cert": cfg.TLSServerCertificate,
			"key":  cfg.TLSServerKey,
		}).Debug("configuring TLS for GRPC")
		cert, err := tls.LoadX509KeyPair(cfg.TLSServerCertificate, cfg.TLSServerKey)
		if err != nil {
			return nil, err

		}
		creds := credentials.NewTLS(&tls.Config{
			Certificates:       []tls.Certificate{cert},
			ClientAuth:         tls.RequestClientCert,
			InsecureSkipVerify: cfg.TLSInsecureSkipVerify,
		})
		grpcOpts = append(grpcOpts, grpc.Creds(creds))
	}
	return grpcOpts, nil
}
