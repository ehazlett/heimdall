package server

import (
	"io/ioutil"
	"os"
	"testing"

	v1 "github.com/ehazlett/heimdall/api/v1"
	"github.com/ehazlett/heimdall/wg"
)

const (
	defaultWireguardInterface = "darknet"
)

func TestWireguardTemplate(t *testing.T) {
	expectedConf := `# managed by heimdall
[Interface]
PrivateKey = SERVER-PRIVATE-KEY
ListenPort = 10000
Address = 1.2.3.4:10000
PostUp = iptables -A FORWARD -i darknet -j ACCEPT; iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE; ip6tables -A FORWARD -i darknet -j ACCEPT; ip6tables -t nat -A POSTROUTING -o eth0 -j MASQUERADE
PostDown = iptables -D FORWARD -i darknet -j ACCEPT; iptables -t nat -D POSTROUTING -o eth0 -j MASQUERADE; ip6tables -D FORWARD -i darknet -j ACCEPT; ip6tables -t nat -D POSTROUTING -o eth0 -j MASQUERADE

# test-peer
[Peer]
PublicKey = PEER-PUBLIC-KEY
PersistentKeepalive = 25
AllowedIPs = 10.100.0.0/24, 10.254.0.0/16
Endpoint = 100.100.100.100:10000

`
	cfg := &wg.Config{
		Interface:     defaultWireguardInterface,
		NodeInterface: "eth0",
		PrivateKey:    "SERVER-PRIVATE-KEY",
		ListenPort:    10000,
		Address:       "1.2.3.4:10000",
		Peers: []*v1.Peer{
			{
				ID: "test-peer",
				KeyPair: &v1.KeyPair{
					PrivateKey: "PEER-PRIVATE-KEY",
					PublicKey:  "PEER-PUBLIC-KEY",
				},
				AllowedIPs: []string{"10.100.0.0/24", "10.254.0.0/16"},
				Endpoint:   "100.100.100.100:10000",
			},
		},
	}
	tmpDir, err := ioutil.TempDir("", "heimdall-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	configPath, err := wg.GenerateNodeConfig(cfg, tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}

	if string(data) != expectedConf {
		t.Fatalf("config does not match; expected \n %q \n received \n %q", expectedConf, string(data))
	}
}
