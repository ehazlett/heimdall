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

package wg

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

	v1 "github.com/ehazlett/heimdall/api/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	wireguardNodeTemplate = `# managed by heimdall
[Interface]
PrivateKey = {{ .PrivateKey }}
ListenPort = {{ .ListenPort }}
Address = {{ .Address }}
PostUp = iptables -A FORWARD -i {{ .Interface }} -j ACCEPT; iptables -t nat -A POSTROUTING -o {{ .NodeInterface }} -j MASQUERADE; ip6tables -A FORWARD -i {{ .Interface }} -j ACCEPT; ip6tables -t nat -A POSTROUTING -o {{ .NodeInterface }} -j MASQUERADE
PostDown = iptables -D FORWARD -i {{ .Interface }} -j ACCEPT; iptables -t nat -D POSTROUTING -o {{ .NodeInterface }} -j MASQUERADE; ip6tables -D FORWARD -i {{ .Interface }} -j ACCEPT; ip6tables -t nat -D POSTROUTING -o {{ .NodeInterface }} -j MASQUERADE
{{ range .Peers }}
# {{ .ID }}
[Peer]
PublicKey = {{ .KeyPair.PublicKey }}
{{ if .AllowedIPs }}AllowedIPs = {{ csvList .AllowedIPs }}{{ end }}{{ if ne .Endpoint "" }}
Endpoint = {{ .Endpoint }}{{ end }}
{{ end }}
`
	wireguardPeerTemplate = `# managed by heimdall
[Interface]
PrivateKey = {{ .PrivateKey }}
Address = {{ .Address }}
DNS = {{ csvList .DNS }}
{{ range .Peers }}
# {{ .ID }}
[Peer]
PublicKey = {{ .KeyPair.PublicKey }}
{{ if .AllowedIPs }}AllowedIPs = {{ csvList .AllowedIPs }}{{ end }}{{ if ne .Endpoint "" }}
Endpoint = {{ .Endpoint }}{{ end }}
{{ end }}
`
)

func csvList(s []string) string {
	return strings.Join(s, ", ")
}

// Config is the Wireguard configuration
type Config struct {
	Interface     string
	NodeInterface string
	PrivateKey    string
	ListenPort    int
	Address       string
	Peers         []*v1.Peer
	DNS           []string
}

// GenerateNodeConfig generates the configuration for a node (server)
func GenerateNodeConfig(cfg *Config, cfgPath string) (string, error) {
	f, err := ioutil.TempFile("", "heimdall-wireguard-")
	if err != nil {
		return "", err
	}
	t, err := template.New("wireguard-node").Funcs(template.FuncMap{
		"csvList": csvList,
	}).Parse(wireguardNodeTemplate)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(cfgPath), 0755); err != nil {
		return "", err
	}

	if err := t.Execute(f, cfg); err != nil {
		return "", err
	}
	f.Close()

	return f.Name(), nil
}

// GeneratePeerConfig generates the configuration for a peer
func GeneratePeerConfig(cfg *Config, cfgPath string) (string, error) {
	f, err := ioutil.TempFile("", "heimdall-wireguard-")
	if err != nil {
		return "", err
	}
	t, err := template.New("wireguard-peer").Funcs(template.FuncMap{
		"csvList": csvList,
	}).Parse(wireguardPeerTemplate)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(cfgPath), 0755); err != nil {
		return "", err
	}

	if err := t.Execute(f, cfg); err != nil {
		return "", err
	}
	f.Close()

	return f.Name(), nil
}

// GenerateWireguardKeys generates a new private/public Wireguard keypair
func GenerateWireguardKeys(ctx context.Context) (string, string, error) {
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

// RestartTunnel restarts the named tunnel
func RestartTunnel(ctx context.Context, name string) error {
	logrus.Infof("restarting tunnel %s", name)
	d, err := wg(ctx, nil)
	if err != nil {
		return err
	}
	// only stop if running
	if string(d) != "" {
		d, err := wgquick(ctx, "down", name)
		if err != nil {
			return errors.Wrap(err, string(d))
		}
	}
	u, err := wgquick(ctx, "up", name)
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
