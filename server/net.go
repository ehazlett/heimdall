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

package server

import (
	"context"
	"fmt"
	"math/big"
	"net"
	"strings"

	"github.com/gomodule/redigo/redis"
	"github.com/sirupsen/logrus"
)

type subnetRange struct {
	Start  net.IP
	End    net.IP
	Subnet *net.IPNet
}

func (s *Server) updateNodeNetwork(ctx context.Context, id string, subnet string) error {
	logrus.Debugf("updating node network: id=%s subnet=%s", id, subnet)
	if _, err := s.master(ctx, "SET", s.getNodeNetworkKey(id), subnet); err != nil {
		return err
	}
	return nil
}

func (s *Server) getOrAllocatePeerIP(ctx context.Context, id string) (net.IP, *net.IPNet, error) {
	r, err := parseSubnetRange(s.cfg.PeerNetwork)
	if err != nil {
		return nil, nil, err
	}

	ip, err := s.getPeerIP(ctx, id)
	if err != nil {
		return nil, nil, err
	}

	if ip != nil {
		return ip, r.Subnet, nil
	}

	ip, err = s.allocatePeerIP(ctx, id, r)
	if err != nil {
		return nil, nil, err
	}

	return ip, r.Subnet, nil
}

func (s *Server) getNodeIP(ctx context.Context, id string) (net.IP, *net.IPNet, error) {
	subnet, err := redis.String(s.local(ctx, "GET", s.getNodeNetworkKey(id)))
	if err != nil {
		return nil, nil, err
	}
	r, err := parseSubnetRange(subnet)
	if err != nil {
		return nil, nil, err
	}

	ip := r.Start
	// assign .1 for router
	ip[len(ip)-1] = 1

	return ip, r.Subnet, nil
}

func (s *Server) getPeerIPs(ctx context.Context) (map[string]net.IP, error) {
	values, err := redis.StringMap(s.local(ctx, "HGETALL", peerIPsKey))
	if err != nil {
		return nil, err
	}

	ips := make(map[string]net.IP, len(values))
	for id, val := range values {
		ip := net.ParseIP(string(val))
		ips[id] = ip
	}
	return ips, nil
}

func (s *Server) getNodeIPs(ctx context.Context) (map[string]net.IP, error) {
	values, err := redis.StringMap(s.local(ctx, "HGETALL", nodeIPsKey))
	if err != nil {
		return nil, err
	}

	ips := make(map[string]net.IP, len(values))
	for id, val := range values {
		ip := net.ParseIP(string(val))
		ips[id] = ip
	}
	return ips, nil
}

func (s *Server) getPeerIP(ctx context.Context, id string) (net.IP, error) {
	allIPs, err := s.getPeerIPs(ctx)
	if err != nil {
		return nil, err
	}

	if ip, exists := allIPs[id]; exists {
		return ip, nil
	}
	return nil, nil
}

func (s *Server) allocatePeerIP(ctx context.Context, id string, r *subnetRange) (net.IP, error) {
	reservedIPs, err := s.getPeerIPs(ctx)
	if err != nil {
		return nil, err
	}

	if ip, exists := reservedIPs[id]; exists {
		return ip, nil
	}

	lookup := map[string]string{}
	for id, ip := range reservedIPs {
		lookup[ip.String()] = id
	}
	for ip := r.Start; !ip.Equal(r.End); s.nextIP(ip) {
		// filter out network, gateway and broadcast
		if !s.validIP(ip) {
			continue
		}
		if _, exists := lookup[ip.String()]; exists {
			// ip already reserved
			continue
		}

		// save
		if _, err := s.master(ctx, "HSET", peerIPsKey, id, ip.String()); err != nil {
			return nil, err
		}
		return ip, nil
	}

	return nil, fmt.Errorf("no available IPs")
}

func (s *Server) releasePeerIP(ctx context.Context, id string) error {
	ip, err := s.getPeerIP(ctx, id)
	if err != nil {
		return err
	}

	if ip != nil {
		if _, err := s.master(ctx, "HDEL", peerIPsKey, id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) nextIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func (s *Server) validIP(ip net.IP) bool {
	v := ip[len(ip)-1]
	switch v {
	case 0, 1, 255:
		return false
	}
	return true
}

// parseSubnetRange parses the subnet range
// format can either be a subnet like 10.0.0.0/8 or range like 10.0.0.100-10.0.0.200/24
func parseSubnetRange(subnet string) (*subnetRange, error) {
	parts := strings.Split(subnet, "-")
	if len(parts) == 1 {
		ip, sub, err := net.ParseCIDR(parts[0])
		if err != nil {
			return nil, err
		}

		end := make(net.IP, len(ip))
		copy(end, ip)
		end[len(end)-1] = 254
		return &subnetRange{
			Start:  ip,
			End:    end,
			Subnet: sub,
		}, nil
	}
	if len(parts) > 2 || !strings.Contains(subnet, "/") {
		return nil, fmt.Errorf("invalid range specified; expect format 10.0.0.100-10.0.0.200/24")
	}
	start := net.ParseIP(parts[0])
	end, sub, err := net.ParseCIDR(parts[1])
	if err != nil {
		return nil, err
	}

	return &subnetRange{
		Start:  start,
		End:    end,
		Subnet: sub,
	}, nil
}

// vendored from
func nextSubnet(n *net.IPNet, prefix int) (*net.IPNet, bool) {
	_, currentLast := addressRange(n)
	mask := net.CIDRMask(prefix, 8*len(currentLast))
	currentSubnet := &net.IPNet{IP: currentLast.Mask(mask), Mask: mask}
	_, last := addressRange(currentSubnet)
	last = inc(last)
	next := &net.IPNet{IP: last.Mask(mask), Mask: mask}
	if last.Equal(net.IPv4zero) || last.Equal(net.IPv6zero) {
		return nil, false
	}
	return next, true
}

func addressRange(network *net.IPNet) (net.IP, net.IP) {
	firstIP := network.IP
	prefixLen, bits := network.Mask.Size()
	if prefixLen == bits {
		lastIP := make([]byte, len(firstIP))
		copy(lastIP, firstIP)
		return firstIP, lastIP
	}

	firstIPInt, bits := ipToInt(firstIP)
	hostLen := uint(bits) - uint(prefixLen)
	lastIPInt := big.NewInt(1)
	lastIPInt.Lsh(lastIPInt, hostLen)
	lastIPInt.Sub(lastIPInt, big.NewInt(1))
	lastIPInt.Or(lastIPInt, firstIPInt)

	return firstIP, intToIP(lastIPInt, bits)

}

func inc(IP net.IP) net.IP {
	IP = checkIPv4(IP)
	incIP := make([]byte, len(IP))
	copy(incIP, IP)
	for j := len(incIP) - 1; j >= 0; j-- {
		incIP[j]++
		if incIP[j] > 0 {
			break
		}
	}
	return incIP
}

func ipToInt(ip net.IP) (*big.Int, int) {
	val := &big.Int{}
	val.SetBytes([]byte(ip))
	if len(ip) == net.IPv4len {
		return val, 32
	} else if len(ip) == net.IPv6len {
		return val, 128
	} else {
		panic(fmt.Errorf("Unsupported address length %d", len(ip)))
	}
}

func intToIP(ipInt *big.Int, bits int) net.IP {
	ipBytes := ipInt.Bytes()
	ret := make([]byte, bits/8)
	for i := 1; i <= len(ipBytes); i++ {
		ret[len(ret)-i] = ipBytes[len(ipBytes)-i]

	}
	return net.IP(ret)
}

func checkIPv4(ip net.IP) net.IP {
	if v4 := ip.To4(); v4 != nil {
		return v4
	}
	return ip
}
