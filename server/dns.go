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
	"net"
	"strings"

	"github.com/miekg/dns"
	"github.com/sirupsen/logrus"
)

func (s *Server) startDNSServer() error {
	dns.HandleFunc(".", s.dnsQueryHandler)

	for _, proto := range []string{"tcp4", "udp4"} {
		srv := &dns.Server{
			Addr: s.cfg.DNSServerAddress,
			Net:  proto,
		}
		go func() {
			if err := srv.ListenAndServe(); err != nil {
				logrus.Errorf("error starting dns server on 53/%s", srv.Net)
			}
		}()
	}

	return nil
}

func (s *Server) dnsQueryHandler(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)
	m.RecursionAvailable = true

	ctx := context.Background()

	query := m.Question[0].Name
	queryType := m.Question[0].Qtype

	logrus.Debugf("dns: query=%q addr=%q", query, w.RemoteAddr())
	name := getName(query, queryType)

	// resolve by node first then peers
	logrus.Debugf("dns: looking up %s", name)
	var recordIP net.IP

	nodes, err := s.getNodes(ctx)
	if err != nil {
		logrus.WithError(err).Error("error getting nodes")
		w.WriteMsg(m)
		return
	}
	for _, n := range nodes {
		if n.Name == name {
			recordIP = net.ParseIP(n.GatewayIP)
			break
		}
	}

	if recordIP == nil {
		peers, err := s.getPeers(ctx)
		if err != nil {
			logrus.WithError(err).Error("error getting nodes")
			w.WriteMsg(m)
			return
		}
		for _, p := range peers {
			if p.Name == name {
				recordIP = net.ParseIP(p.PeerIP)
				break
			}
		}
	}

	// forward if empty
	if recordIP == nil {
		x, err := dns.Exchange(r, s.cfg.DNSUpstreamAddress)
		if err != nil {
			logrus.Errorf("dns: error forwarding lookup: %+v", err)
			w.WriteMsg(m)
			return
		}
		x.SetReply(r)
		w.WriteMsg(x)
		return
	}

	// defer WriteMsg to ensure a response
	defer w.WriteMsg(m)

	m.Answer = []dns.RR{}
	m.Extra = []dns.RR{}

	records := []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{
				Name:   fqdn(name),
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    0,
			},
			A: recordIP,
		},
	}

	m.Answer = records
}

func getName(query string, queryType uint16) string {
	// adjust lookup for srv
	if queryType == dns.TypeSRV {
		p := strings.Split(query, ".")
		v := strings.Join(p[2:], ".")
		return v[:len(v)-1]
	}
	return query[:len(query)-1]
}

func fqdn(name string) string {
	return name + "."
}
