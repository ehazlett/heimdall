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
	var (
		gatewayIP net.IP
		recordIPs []net.IP
	)

	nodes, err := s.getNodes(ctx)
	if err != nil {
		logrus.WithError(err).Error("error getting nodes")
		w.WriteMsg(m)
		return
	}
	for _, n := range nodes {
		if n.Name == name {
			logrus.Debugf("gateway node: %+v", n)
			gatewayIP = net.ParseIP(n.GatewayIP)
			break
		}
	}

	if gatewayIP == nil {
		peers, err := s.getPeers(ctx)
		if err != nil {
			logrus.WithError(err).Error("error getting nodes")
			w.WriteMsg(m)
			return
		}
		for _, p := range peers {
			if p.Name == name {
				recordIPs = append(recordIPs, net.ParseIP(p.PeerIP))
			}
		}
	}

	// forward if empty
	if gatewayIP == nil && len(recordIPs) == 0 {
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

	records := []dns.RR{}
	if gatewayIP != nil {
		records = append(records, &dns.A{
			Hdr: dns.RR_Header{
				Name:   fqdn(name),
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    10,
			},
			A: gatewayIP,
		})
	}

	for _, r := range recordIPs {
		records = append(records, &dns.A{
			Hdr: dns.RR_Header{
				Name:   fqdn(name),
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    10,
			},
			A: r,
		})
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
