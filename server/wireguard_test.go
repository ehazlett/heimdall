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
	"io/ioutil"
	"os"
	"testing"

	v1 "github.com/stellarproject/heimdall/api/v1"
)

func TestWireguardTemplate(t *testing.T) {
	expectedConf := `# managed by heimdall
[Interface]
PrivateKey = SERVER-PRIVATE-KEY
ListenPort = 10000
Address = 1.2.3.4:10000
PostUp = iptables -A FORWARD -i darknet -j ACCEPT; iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE; ip6tables -A FORWARD -i darknet -j ACCEPT; ip6tables -t nat -A POSTROUTING -o eth0 -j MASQUERADE
PostDown = iptables -D FORWARD -i darknet -j ACCEPT; iptables -t nat -D POSTROUTING -o eth0 -j MASQUERADE; ip6tables -D FORWARD -i darknet -j ACCEPT; ip6tables -t nat -D POSTROUTING -o eth0 -j MASQUERADE

[Peer]
PublicKey = PEER-PUBLIC-KEY
AllowedIPs = 10.100.0.0/24, 10.254.0.0/16
Endpoint = 100.100.100.100:10000

`
	cfg := &wireguardConfig{
		Iface:      defaultWireguardInterface,
		PrivateKey: "SERVER-PRIVATE-KEY",
		ListenPort: 10000,
		Address:    "1.2.3.4:10000",
		Peers: []*v1.Peer{
			{
				KeyPair: &v1.KeyPair{
					PrivateKey: "PEER-PRIVATE-KEY",
					PublicKey:  "PEER-PUBLIC-KEY",
				},
				AllowedIPs: []string{"10.100.0.0/24", "10.254.0.0/16"},
				Endpoint:   "100.100.100.100:10000",
			},
		},
	}
	configPath, err := generateNodeWireguardConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(configPath)
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}

	if string(data) != expectedConf {
		t.Fatalf("config does not match; expected \n %q \n received \n %q", expectedConf, string(data))
	}
}
