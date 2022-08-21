package heimdall

import (
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"sort"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// NodeID returns a unique local node ID
func NodeID() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		panic(err)
	}

	// sort
	sort.SliceStable(ifaces, func(i, j int) bool { return ifaces[i].Name < ifaces[j].Name })

	var i net.Interface
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagPointToPoint != 0 {
			continue
		}

		i = iface
		break
	}

	s := uuid.NewSHA1(uuid.Nil, i.HardwareAddr)
	return s.String()
}

// GetIP returns the first non-local IP address for the system
func GetIP() string {
	ip := "127.0.0.1"
	ifaces, err := net.Interfaces()
	if err != nil {
		logrus.Warnf("unable to detect network interfaces")
		return ip
	}
	for _, i := range ifaces {
		a := getInterfaceIP(i)
		if a != "" {
			return a
		}
	}

	return ip
}

func getInterfaceIP(iface net.Interface) string {
	addrs, err := iface.Addrs()
	if err != nil {
		return ""
	}
	for _, addr := range addrs {
		var ip net.IP
		switch v := addr.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}
		// skip loopback
		if ip.IsLoopback() {
			return ""
		}
		if ip.To4() == nil {
			return ""
		}
		return ip.To4().String()
	}

	return ""
}

// HashData returns a sha256 sum of the specified data
func HashData(data []byte) string {
	h := sha256.New()
	h.Write(data)
	return fmt.Sprintf("%x", h.Sum(nil))
}

// HashConfig returns the sha256 hash of the specified path
func HashConfig(cfgPath string) (string, error) {
	if _, err := os.Stat(cfgPath); err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	peerData, err := ioutil.ReadFile(cfgPath)
	if err != nil {
		return "", err
	}

	return HashData(peerData), nil
}
