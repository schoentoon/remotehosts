# Your remote hosts file manager for coredns

Rather than having to maintain a local copy of various hosts lists to block, this plugin will retrieve this lists automatically and periodically.

In order to use this plugin you'll have to clone [coredns](https://github.com/coredns/coredns) and modify the plugin.cfg file to include the following line:
`remotehosts:github.com/schoentoon/remotehosts`
It is important to realize that CoreDNS will determine the order that it uses the plugins is based on the order in this list and not in the order you specify in your `Corefile`,
so you may want to put this line before the `forward` plugin or whatever plugin you use to reach your upstream dns.
After this you can just build coredns the way you usually build it which is simply calling `make`. After this confirm that the plugin was build correctly into coredns using the following command.
```bash
$ ./coredns -plugins | grep remotehosts
  dns.remotehosts
```

# Configuration

Now to actually configure the plugin have a look at the following Corefile example
```
. {
  remotehosts . {
    http://example.org/some/text/file.txt
    https://example.org/you/can/add/more/urls/here
    reload 5m
  }
  forward . 8.8.8.8 8.8.4.4
}
```

In this case it'll retrieve all the listed urls every 5 minutes and add the results to the internal block list. Format of these files should be the same as a regular hosts file, just like in the [hosts](https://coredns.io/plugins/hosts/) plugin, although ip addresses are ignored and 127.0.0.1 will always be returned. Do note that in this example afterwards a `forward . 8.8.8.8 8.8.4.4` is listed, this is exactly why the order of your `plugin.cfg` is important during build time. Because if the forward plugin would have been higher than the `remotehosts` plugin it would be called first and it obviously wouldn't actually 'block' anything.

# Metrics

If monitoring is enabled (via the prometheus directive) then the following metrics are exported:

* `coredns_remotehosts_size` - Total amount blocked domains currently in memory
* `coredns_remotehosts_blocklist_hits` - Counter of hits for blocked domains