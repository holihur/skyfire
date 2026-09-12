package tunnel

import (
	"context"
	"log/slog"
	"net"
	"net/netip"
	"strings"
	"sync"
	"time"
)

// whitelistRefreshInterval is how often resolved hostnames are re-resolved.
// CDN/DNS answers change frequently, so routes are refreshed periodically
// rather than only at connect time.
const whitelistRefreshInterval = 60 * time.Second

// loopbackDNS is where the split-DNS proxy listens; the system resolver is
// pointed at it while whitelist mode is active.
const loopbackDNS = "127.0.0.1:53"

type ruleKind int

const (
	rulePrefix   ruleKind = iota // CIDR / IP prefix routed directly
	ruleExact                    // exact hostname
	ruleWildcard                 // "*.example.com" matches subdomains
)

type rule struct {
	kind   ruleKind
	prefix netip.Prefix
	suffix string // exact: full hostname; wildcard: "example.com"
}

// parseWhitelist turns user entries into match rules. CIDR notation and bare
// IPs become prefix routes; "*."-prefixed entries become wildcards; everything
// else is an exact hostname.
func parseWhitelist(entries []string) []rule {
	var out []rule
	for _, e := range entries {
		e = strings.ToLower(strings.TrimSpace(e))
		if e == "" {
			continue
		}
		if strings.Contains(e, "/") {
			if pre, err := netip.ParsePrefix(e); err == nil {
				out = append(out, rule{kind: rulePrefix, prefix: pre.Masked()})
				continue
			}
		}
		if ip, err := netip.ParseAddr(e); err == nil {
			out = append(out, rule{kind: rulePrefix, prefix: netip.PrefixFrom(ip, ip.BitLen())})
			continue
		}
		if strings.HasPrefix(e, "*.") {
			suffix := strings.TrimPrefix(e, "*.")
			if suffix != "" {
				out = append(out, rule{kind: ruleWildcard, suffix: suffix})
			}
			continue
		}
		out = append(out, rule{kind: ruleExact, suffix: e})
	}
	return out
}

// matchDomain reports whether name matches any exact/wildcard domain rule.
func matchDomain(rules []rule, name string) bool {
	name = strings.ToLower(strings.TrimSuffix(name, "."))
	for _, r := range rules {
		switch r.kind {
		case ruleExact:
			if name == r.suffix {
				return true
			}
		case ruleWildcard:
			if name == r.suffix || strings.HasSuffix(name, "."+r.suffix) {
				return true
			}
		}
	}
	return false
}

// whitelistRouter owns split-by-domain routing: static CIDR routes, host
// routes for resolved domain IPs, and the split-DNS proxy that resolves
// matched names through the tunnel on demand. stop() restores DNS and removes
// every route before the device is torn down.
type whitelistRouter struct {
	dev   string
	dns   []string
	rules []rule
	log   *slog.Logger

	tunnelRes   *net.Resolver
	upstreamRes *net.Resolver
	proxy       *dnsProxy

	mu           sync.Mutex
	prefixRoutes map[netip.Prefix]struct{}
	hostRoutes   map[netip.Addr]struct{}
	seen         map[string]struct{}
	dnsApplied   bool

	stopCh chan struct{}
	doneCh chan struct{}
}

func newWhitelistRouter(dev string, c *Conf, log *slog.Logger) *whitelistRouter {
	r := &whitelistRouter{
		dev:          dev,
		dns:          c.DNS,
		rules:        parseWhitelist(c.Whitelist),
		log:          log,
		prefixRoutes: make(map[netip.Prefix]struct{}),
		hostRoutes:   make(map[netip.Addr]struct{}),
		seen:         make(map[string]struct{}),
	}
	r.tunnelRes = resolverFor(c.DNS)
	if upstream := upstreamServer(); upstream != "" {
		r.upstreamRes = resolverFor([]string{upstream})
	}
	return r
}

// start installs CIDR routes, resolves exact hostnames, starts the split-DNS
// proxy and points the system resolver at it. A proxy failure degrades to
// CIDR + exact-hostname routing only (wildcards stop working) and is logged.
func (r *whitelistRouter) start() {
	for _, pre := range r.prefixRules() {
		if err := addTunnelPrefix(r.dev, pre); err != nil {
			r.log.Warn("add CIDR route", "prefix", pre.String(), "error", err)
			continue
		}
		r.prefixRoutes[pre] = struct{}{}
	}
	for _, d := range r.exactDomains() {
		r.resolveAndRoute(d)
	}

	if r.hasDomainRules() {
		upstream := net.JoinHostPort(upstreamServer(), "53")
		r.proxy = newDNSProxy(upstream, r.matchDomain, func(name string) ([]netip.Addr, error) {
			return r.resolveAndRoute(name), nil
		}, r.log)
		if err := r.proxy.start(loopbackDNS); err != nil {
			r.log.Warn("split-DNS proxy failed to start; wildcard domains disabled", "error", err)
			r.proxy = nil
		} else if err := configureDNS(r.dev, []string{"127.0.0.1"}); err != nil {
			r.log.Warn("apply split DNS failed; disabling proxy", "error", err)
			unconfigureDNS(r.dev, []string{"127.0.0.1"})
			r.proxy.close()
			r.proxy = nil
		} else {
			r.dnsApplied = true
		}
	}

	if r.hasDomainRules() {
		r.stopCh = make(chan struct{})
		r.doneCh = make(chan struct{})
		go r.loop()
	}
}

// stop restores DNS, stops the proxy and removes all installed routes.
func (r *whitelistRouter) stop() {
	if r.dnsApplied {
		unconfigureDNS(r.dev, []string{"127.0.0.1"})
		r.dnsApplied = false
	}
	if r.proxy != nil {
		r.proxy.close()
		r.proxy = nil
	}
	if r.stopCh != nil {
		close(r.stopCh)
		<-r.doneCh
		r.stopCh = nil
		r.doneCh = nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for pre := range r.prefixRoutes {
		delTunnelPrefix(r.dev, pre)
	}
	for ip := range r.hostRoutes {
		delTunnelPrefix(r.dev, hostPrefix(ip))
	}
	r.prefixRoutes = make(map[netip.Prefix]struct{})
	r.hostRoutes = make(map[netip.Addr]struct{})
}

func (r *whitelistRouter) loop() {
	defer close(r.doneCh)
	t := time.NewTicker(whitelistRefreshInterval)
	defer t.Stop()
	for {
		select {
		case <-r.stopCh:
			return
		case <-t.C:
			r.refresh()
		}
	}
}

// refresh re-resolves every hostname seen so far (exact entries plus names
// matched by the proxy) and reconciles host routes, pruning IPs that no longer
// resolve.
func (r *whitelistRouter) refresh() {
	r.mu.Lock()
	names := make([]string, 0, len(r.seen))
	for n := range r.seen {
		names = append(names, n)
	}
	r.mu.Unlock()

	wanted := r.dnsIPs()
	for _, n := range names {
		for ip := range r.resolve(n) {
			wanted[ip] = struct{}{}
		}
	}
	r.applyHostRoutes(wanted)
}

// dnsIPs returns the host addresses of the configured tunnel DNS servers so
// their answers can travel through the tunnel.
func (r *whitelistRouter) dnsIPs() map[netip.Addr]struct{} {
	out := make(map[netip.Addr]struct{})
	for _, d := range r.dns {
		host := d
		if h, _, err := net.SplitHostPort(d); err == nil {
			host = h
		}
		if ip, err := netip.ParseAddr(host); err == nil {
			out[ip.Unmap()] = struct{}{}
		}
	}
	return out
}

// resolve returns the IPs of a hostname, preferring the tunnel DNS and
// falling back to the captured upstream DNS (never the system resolver, which
// may be pointed back at our own proxy).
func (r *whitelistRouter) resolve(name string) map[netip.Addr]struct{} {
	out := make(map[netip.Addr]struct{})
	if r.tunnelRes != nil {
		for _, a := range lookupWith(r.tunnelRes, name) {
			if ip, err := netip.ParseAddr(a); err == nil {
				out[ip.Unmap()] = struct{}{}
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	if r.upstreamRes != nil {
		for _, a := range lookupWith(r.upstreamRes, name) {
			if ip, err := netip.ParseAddr(a); err == nil {
				out[ip.Unmap()] = struct{}{}
			}
		}
	}
	return out
}

// resolveAndRoute resolves a name, installs host routes for its IPs and
// records it for periodic refresh. It returns the resolved IPs.
func (r *whitelistRouter) resolveAndRoute(name string) []netip.Addr {
	ips := r.resolve(name)
	r.mu.Lock()
	r.seen[name] = struct{}{}
	r.mu.Unlock()
	r.addHostRoutes(ips)
	out := make([]netip.Addr, 0, len(ips))
	for ip := range ips {
		out = append(out, ip)
	}
	return out
}

func (r *whitelistRouter) addHostRoutes(ips map[netip.Addr]struct{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for ip := range ips {
		if _, ok := r.hostRoutes[ip]; ok {
			continue
		}
		if err := addTunnelPrefix(r.dev, hostPrefix(ip)); err != nil {
			r.log.Warn("add tunnel route", "ip", ip.String(), "error", err)
			continue
		}
		r.hostRoutes[ip] = struct{}{}
	}
}

func (r *whitelistRouter) applyHostRoutes(wanted map[netip.Addr]struct{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for ip := range wanted {
		if _, ok := r.hostRoutes[ip]; ok {
			continue
		}
		if err := addTunnelPrefix(r.dev, hostPrefix(ip)); err != nil {
			r.log.Warn("add tunnel route", "ip", ip.String(), "error", err)
			continue
		}
		r.hostRoutes[ip] = struct{}{}
	}
	for ip := range r.hostRoutes {
		if _, ok := wanted[ip]; ok {
			continue
		}
		delTunnelPrefix(r.dev, hostPrefix(ip))
		delete(r.hostRoutes, ip)
	}
}

func (r *whitelistRouter) matchDomain(name string) bool {
	return matchDomain(r.rules, name)
}

func (r *whitelistRouter) prefixRules() []netip.Prefix {
	var out []netip.Prefix
	for _, r := range r.rules {
		if r.kind == rulePrefix {
			out = append(out, r.prefix)
		}
	}
	return out
}

func (r *whitelistRouter) exactDomains() []string {
	var out []string
	for _, r := range r.rules {
		if r.kind == ruleExact {
			out = append(out, r.suffix)
		}
	}
	return out
}

func (r *whitelistRouter) hasDomainRules() bool {
	for _, r := range r.rules {
		if r.kind == ruleExact || r.kind == ruleWildcard {
			return true
		}
	}
	return false
}

func hostPrefix(ip netip.Addr) netip.Prefix {
	return netip.PrefixFrom(ip, ip.BitLen())
}

func lookupWith(res *net.Resolver, host string) []string {
	if res == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ips, err := res.LookupHost(ctx, host)
	if err != nil {
		return nil
	}
	return ips
}

// resolverFor builds a Go resolver pinned to the first given DNS server. A
// nil return means "no DNS server configured".
func resolverFor(dns []string) *net.Resolver {
	if len(dns) == 0 {
		return nil
	}
	server := dns[0]
	if _, _, err := net.SplitHostPort(server); err != nil {
		server = net.JoinHostPort(server, "53")
	}
	return &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			d := net.Dialer{Timeout: 5 * time.Second}
			return d.DialContext(ctx, "udp", server)
		},
	}
}

// upstreamServer returns the first non-loopback system DNS server, falling
// back to a public resolver when none can be found. It is captured before the
// system resolver is pointed at the local proxy, so it cannot loop back.
func upstreamServer() string {
	for _, s := range currentDNSServers() {
		if ip := net.ParseIP(s); ip != nil && !ip.IsLoopback() {
			return s
		}
	}
	return "1.1.1.1"
}
