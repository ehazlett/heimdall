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

package server

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	v1 "github.com/stellarproject/heimdall/api/v1"
)

const (
	defaultWireguardInterface = "darknet"
	wireguardTemplate         = `# managed by heimdall
[Interface]
PrivateKey = {{ .PrivateKey }}
ListenPort = {{ .ListenPort }}
Address = {{ .Address }}
PostUp = iptables -A FORWARD -i {{ .Iface }} -j ACCEPT; iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE; ip6tables -A FORWARD -i {{ .Iface }} -j ACCEPT; ip6tables -t nat -A POSTROUTING -o eth0 -j MASQUERADE
PostDown = iptables -D FORWARD -i {{ .Iface }} -j ACCEPT; iptables -t nat -D POSTROUTING -o eth0 -j MASQUERADE; ip6tables -D FORWARD -i {{ .Iface }} -j ACCEPT; ip6tables -t nat -D POSTROUTING -o eth0 -j MASQUERADE
{{ range .Peers }}
[Peer]
PublicKey = {{ .KeyPair.PublicKey }}
AllowedIPs = {{ allowedIPs .AllowedIPs }}
Endpoint = {{ .Endpoint }}
{{ end }}
`
)

func allowedIPs(s []string) string {
	return strings.Join(s, ", ")
}

type wireguardConfig struct {
	Iface      string
	PrivateKey string
	ListenPort int
	Address    string
	Peers      []*v1.Peer
}

func generateNodeWireguardConfig(cfg *wireguardConfig) (string, error) {
	f, err := ioutil.TempFile("", "heimdall-wireguard-")
	if err != nil {
		return "", err
	}
	t, err := template.New("wireguard").Funcs(template.FuncMap{
		"allowedIPs": allowedIPs,
	}).Parse(wireguardTemplate)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(wireguardConfigPath), 0755); err != nil {
		return "", err
	}

	if err := t.Execute(f, cfg); err != nil {
		return "", err
	}
	f.Close()

	return f.Name(), nil
}

func generateWireguardKeys(ctx context.Context) (string, string, error) {
	kData, err := wg(ctx, nil, "genkey")
	if err != nil {
		return "", "", err
	}
	privateKey := strings.TrimSpace(string(kData))
	buf := bytes.NewBufferString(privateKey)
	pubData, err := wg(ctx, buf, "pubkey")
	if err != nil {
		return "", "", err
	}
	publicKey := strings.TrimSpace(string(pubData))

	return privateKey, publicKey, nil
}

func restartWireguardTunnel(ctx context.Context) error {
	tunnelName := strings.Replace(filepath.Base(wireguardConfigPath), filepath.Ext(filepath.Base(wireguardConfigPath)), "", 1)
	logrus.Infof("restarting tunnel %s", tunnelName)
	d, err := wgquick(ctx, "down", tunnelName)
	if err != nil {
		return errors.Wrap(err, string(d))
	}
	u, err := wgquick(ctx, "up", tunnelName)
	if err != nil {
		return errors.Wrap(err, string(u))
	}
	return nil
}

func wg(ctx context.Context, in io.Reader, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "wg", args...)
	if in != nil {
		cmd.Stdin = in
	}
	return cmd.CombinedOutput()
}

func wgquick(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "wg-quick", args...)
	return cmd.CombinedOutput()
}
