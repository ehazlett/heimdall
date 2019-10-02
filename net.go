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

package main

import (
	"fmt"
	"net"
	"strings"

	"github.com/gomodule/redigo/redis"
)

type subnetRange struct {
	Start  net.IP
	End    net.IP
	Subnet *net.IPNet
}

func (s *Server) getIPs() (map[string]net.IP, error) {
	c, err := s.getConn()
	if err != nil {
		return nil, err
	}
	defer c.Close()

	values, err := redis.StringMap(c.Do("HGETALL", ipsKey))
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

func (s *Server) getIP(id string) (net.IP, error) {
	allIPs, err := s.getIPs()
	if err != nil {
		return nil, err
	}

	if ip, exists := allIPs[id]; exists {
		return ip, nil
	}
	return nil, nil
}

func (s *Server) getOrAllocateIP(id, subnet string) (net.IP, *net.IPNet, error) {
	r, err := s.parseSubnetRange(subnet)
	if err != nil {
		return nil, nil, err
	}

	ip, err := s.getIP(id)
	if err != nil {
		return nil, nil, err
	}

	if ip != nil {
		return ip, r.Subnet, nil
	}

	ip, err = s.allocateIP(id, r)
	if err != nil {
		return nil, nil, err
	}

	return ip, r.Subnet, nil
}

func (s *Server) allocateIP(id string, r *subnetRange) (net.IP, error) {
	c, err := s.getConn()
	if err != nil {
		return nil, err
	}
	defer c.Close()

	reservedIPs, err := s.getIPs()
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
		if _, err := c.Do("HSET", ipsKey, id, ip.String()); err != nil {
			return nil, err
		}
		return ip, nil
	}

	return nil, fmt.Errorf("no available IPs")
}

func (s *Server) releaseIP(id string) error {
	c, err := s.getConn()
	if err != nil {
		return err
	}
	defer c.Close()

	ip, err := s.getIP(id)
	if err != nil {
		return err
	}

	if ip != nil {
		if _, err := c.Do("HDEL", ipsKey, id); err != nil {
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
func (s *Server) parseSubnetRange(subnet string) (*subnetRange, error) {
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
