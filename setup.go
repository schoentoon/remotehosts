package remotehosts

import (
	"net/url"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	clog "github.com/coredns/coredns/plugin/pkg/log"

	"github.com/caddyserver/caddy"
)

var log = clog.NewWithPlugin("remotehosts")

func init() {
	caddy.RegisterPlugin("remotehosts", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	h, err := hostsParse(c)
	if err != nil {
		return plugin.Error("remotehosts", err)
	}

	c.OnStartup(func() error {
		return h.RemoteHosts.readHosts()
	})

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		h.Next = next
		return h
	})

	return nil
}

func hostsParse(c *caddy.Controller) (RemoteHostsPlugin, error) {
	h := RemoteHostsPlugin{
		RemoteHosts: &RemoteHosts{
			URLs:      []*url.URL{},
			BlackHole: make(map[string]struct{}),
		},
	}

	if c.Next() {
		_ = c.RemainingArgs()
		for c.NextBlock() {
			uri, err := url.Parse(c.Val())
			if err != nil {
				return h, err
			}
			h.RemoteHosts.URLs = append(h.RemoteHosts.URLs, uri)
		}
	}

	return h, nil
}
