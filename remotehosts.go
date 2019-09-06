package remotehosts

import (
	"bufio"
	"bytes"
	"context"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
)

// parseIP calls discards any v6 zone info, before calling net.ParseIP.
func parseIP(addr string) net.IP {
	if i := strings.Index(addr, "%"); i >= 0 {
		// discard ipv6 zone
		addr = addr[0:i]
	}

	return net.ParseIP(addr)
}

type RemoteHostsPlugin struct {
	RemoteHosts *RemoteHosts

	Next plugin.Handler
}

type RemoteHosts struct {
	URLs []*url.URL

	BlackHoleLock sync.RWMutex
	BlackHole     map[string]struct{}
}

func (h RemoteHostsPlugin) Name() string {
	return "remotehosts"
}

func (h RemoteHostsPlugin) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}
	qname := state.Name()

	if !h.RemoteHosts.isBlocked(qname) {
		return plugin.NextOrFailure(h.Name(), h.Next, ctx, w, r)
	}

	answer := new(dns.A)
	answer.Hdr = dns.RR_Header{Name: qname, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 3600}

	switch state.QType() {
	case dns.TypeA:
		answer.A = net.IPv4(127, 0, 0, 1)
	case dns.TypeAAAA:
		answer.Hdr.Rrtype = dns.TypeAAAA
		answer.A = net.IPv6loopback
	}

	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative = true
	m.Answer = []dns.RR{answer}

	w.WriteMsg(m)
	return dns.RcodeSuccess, nil
}

func (h RemoteHosts) readHosts() error {
	for _, uri := range h.URLs {
		err := h.fetchURI(uri)
		if err != nil {
			return err
		}
	}
	return nil
}

func (h RemoteHosts) fetchURI(uri *url.URL) error {
	req := http.Request{
		Method: http.MethodGet,
		URL:    uri,
	}

	resp, err := http.DefaultClient.Do(&req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)

	h.BlackHoleLock.Lock()
	defer h.BlackHoleLock.Unlock()

	for scanner.Scan() {
		line := scanner.Bytes()
		if i := bytes.Index(line, []byte{'#'}); i >= 0 {
			// Discard comments.
			line = line[0:i]
		}
		f := bytes.Fields(line)
		if len(f) < 2 {
			continue
		}
		addr := parseIP(string(f[0]))
		if addr == nil {
			continue
		}

		for i := 1; i < len(f); i++ {
			name := plugin.Name(string(f[i])).Normalize()
			h.BlackHole[name] = struct{}{}
		}
	}

	log.Debugf("blackhole now contains %d entries", len(h.BlackHole))
	return nil
}

func (h RemoteHosts) isBlocked(uri string) bool {
	h.BlackHoleLock.RLock()
	defer h.BlackHoleLock.RUnlock()

	_, ok := h.BlackHole[uri]

	return ok
}
