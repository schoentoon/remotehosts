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
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus"
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
	*RemoteHosts

	Next plugin.Handler
}

type RemoteHosts struct {
	// The time between two reload of the configuration
	reload time.Duration

	// the URLs to fetch
	URLs []*url.URL

	// The actual block list and a mutex protecting access to it
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
	log.Debugf("incoming query %s is blocked", qname)
	blockListHits.WithLabelValues().Inc()

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

func (h *RemoteHosts) readHosts() error {
	log.Infof("Loading hosts")
	for _, uri := range h.URLs {
		err := h.fetchURI(uri)
		if err != nil {
			return err
		}
	}
	return nil
}

func (h *RemoteHosts) fetchURI(uri *url.URL) error {
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

	blockListSize.WithLabelValues().Set(float64(len(h.BlackHole)))

	log.Infof("blackhole now contains %d entries", len(h.BlackHole))
	return nil
}

func (h *RemoteHosts) isBlocked(uri string) bool {
	h.BlackHoleLock.RLock()
	defer h.BlackHoleLock.RUnlock()

	_, ok := h.BlackHole[uri]

	return ok
}

var (
	blockListSize = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: plugin.Namespace,
		Subsystem: "remotehosts",
		Name:      "size",
		Help:      "The number of elements in the blocklist.",
	}, []string{})

	blockListHits = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: plugin.Namespace,
		Subsystem: "remotehosts",
		Name:      "blocklist_hits",
		Help:      "The count of blocklist hits.",
	}, []string{})
)
