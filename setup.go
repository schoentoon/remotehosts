package remotehosts

import (
	"net/url"
	"time"

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

func periodicHostsUpdate(h *RemoteHosts) chan bool {
	parseChan := make(chan bool)

	if h.reload == 0 {
		return parseChan
	}

	go func() {
		ticker := time.NewTicker(h.reload)
		for {
			select {
			case <-parseChan:
				return
			case <-ticker.C:
				h.readHosts()
			}
		}
	}()

	return parseChan
}

func setup(c *caddy.Controller) error {
	h, err := hostsParse(c)
	if err != nil {
		return plugin.Error("remotehosts", err)
	}

	parseChan := periodicHostsUpdate(h.RemoteHosts)

	c.OnStartup(func() error {
		return h.RemoteHosts.readHosts()
	})

	c.OnShutdown(func() error {
		close(parseChan)
		return nil
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
			switch c.Val() {
			case "reload":
				remaining := c.RemainingArgs()
				if len(remaining) != 1 {
					return h, c.Errf("reload needs a duration (zero seconds to disable)")
				}
				reload, err := time.ParseDuration(remaining[0])
				if err != nil {
					return h, c.Errf("invalid duration for reload '%s'", remaining[0])
				}
				if reload < 0 {
					return h, c.Errf("invalid negative duration for reload '%s'", remaining[0])
				}
				h.RemoteHosts.reload = reload
			default:
				uri, err := url.Parse(c.Val())
				if err != nil {
					return h, err
				}
				h.RemoteHosts.URLs = append(h.RemoteHosts.URLs, uri)
			}
		}
	}

	return h, nil
}
