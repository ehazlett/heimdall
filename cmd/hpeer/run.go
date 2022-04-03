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
	"net"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ehazlett/heimdall"
	"github.com/ehazlett/heimdall/peer"
	"github.com/ehazlett/heimdall/version"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	initConnectTimeout = time.Second * 10
)

func run(cx *cli.Context) error {
	cfg := &heimdall.PeerConfig{
		ID:                    cx.String("id"),
		Name:                  cx.String("name"),
		Address:               cx.String("addr"),
		UpdateInterval:        cx.Duration("update-interval"),
		InterfaceName:         cx.String("interface-name"),
		TLSClientCertificate:  cx.String("cert"),
		TLSClientKey:          cx.String("key"),
		TLSInsecureSkipVerify: cx.Bool("skip-verify"),
	}

	u, err := url.Parse(cfg.Address)
	if err != nil {
		return err
	}
	waitCh := make(chan bool, 1)
	logrus.Debugf("waiting for %s to be reachable", cfg.Address)
	go func(ch chan bool) {
		for {
			if _, err := net.DialTimeout("tcp", u.Host, time.Second*1); err == nil {
				ch <- true
				return
			}
			time.Sleep(time.Second * 1)
		}
	}(waitCh)

	select {
	case <-waitCh:
	case <-time.After(initConnectTimeout):
		return fmt.Errorf("timeout waiting on endpoint %s to be reachable", cfg.Address)
	}

	// wait until address is reachable
	p, err := peer.NewPeer(cfg)
	if err != nil {
		return err
	}

	errCh := make(chan error, 1)

	signals := make(chan os.Signal)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1)
	doneCh := make(chan bool, 1)
	go func() {
		for {
			select {
			case sig := <-signals:
				switch sig {
				case syscall.SIGTERM, syscall.SIGINT:
					logrus.Info("shutting down")
					if err := p.Stop(); err != nil {
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
		if err := p.Run(); err != nil {
			errCh <- err
		}
	}()

	select {
	case <-doneCh:
		return nil
	case err := <-errCh:
		return err
	}

	return nil
}
