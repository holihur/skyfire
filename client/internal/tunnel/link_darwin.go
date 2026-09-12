//go:build darwin

package tunnel

import (
	"fmt"
	"net"
	"net/netip"
	"os/exec"
	"strconv"
	"strings"
)

// tunName lets the kernel pick the next free utunN.
func tunName() string { return "utun" }

func ifconfig(dev string, args ...string) error {
	cmdArgs := append([]string{dev}, args...)
	out, err := exec.Command("ifconfig", cmdArgs...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("ifconfig %s: %w (%s)", strings.Join(cmdArgs, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func routeCmd(args ...string) error {
	out, err := exec.Command("route", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("route %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func configureLink(dev string, c *Conf) error {
	if err := addAddresses(dev, c.Addresses); err != nil {
		return err
	}
	if c.MTU > 0 {
		if err := ifconfig(dev, "mtu", strconv.Itoa(c.MTU)); err != nil {
			return err
		}
	}
	if err := ifconfig(dev, "up"); err != nil {
		return err
	}
	return addRoutes(dev, c)
}

// unconfigureLink removes the endpoint-exception host routes; the interface
// scoped routes disappear with the utun device.
func unconfigureLink(_ string, c *Conf) {
	if c == nil || c.WhitelistMode() {
		return
	}
	removeEndpointExceptions(c)
}

// dnsServiceKey is the scutil resolver slot we own on macOS.
const dnsServiceKey = "State:/Network/Service/skyfire-client/DNS"

// configureDNS installs a scutil resolver pointing at the tunnel DNS. An empty
// SupplementalMatchDomains entry makes it the default resolver for every
// domain. Called separately from configureLink so DNS failure is non-fatal.
func configureDNS(_ string, servers []string) error {
	if len(servers) == 0 {
		return nil
	}
	script := fmt.Sprintf(
		"d.init\nd.add ServerAddresses * %s\nd.add SupplementalMatchDomains * \"\"\nset %s\n",
		strings.Join(servers, " "), dnsServiceKey)
	return runScutil(script)
}

// unconfigureDNS removes the scutil resolver installed by configureDNS.
func unconfigureDNS(_ string, servers []string) {
	if len(servers) == 0 {
		return
	}
	_ = runScutil("remove " + dnsServiceKey + "\n")
}

func runScutil(script string) error {
	cmd := exec.Command("scutil")
	cmd.Stdin = strings.NewReader(script)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("scutil: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func addAddresses(dev string, addresses []string) error {
	for _, addr := range addresses {
		p, err := netip.ParsePrefix(addr)
		if err != nil {
			return fmt.Errorf("invalid address %q: %w", addr, err)
		}
		local := p.Addr().String()
		if p.Addr().Is4() {
			mask := net.IP(net.CIDRMask(p.Bits(), 32)).String()
			err = ifconfig(dev, "inet", local, local, "netmask", mask)
		} else {
			err = ifconfig(dev, "inet6", fmt.Sprintf("%s/%d", local, p.Bits()))
		}
		if err != nil {
			msg := strings.ToLower(err.Error())
			if strings.Contains(msg, "file exists") || strings.Contains(msg, "already") {
				continue
			}
			return err
		}
	}
	return nil
}

func addRoutes(dev string, c *Conf) error {
	// Split-by-domain mode: routes are maintained by the whitelist router
	// instead of the peer's AllowedIPs.
	if c.WhitelistMode() {
		return nil
	}
	// Pin each endpoint to the physical path BEFORE installing the full-tunnel
	// /1 routes; otherwise those routes capture the tunnel's own transport and
	// create a routing loop that cuts the machine off.
	if err := addEndpointExceptions(c); err != nil {
		return err
	}
	v4, v6 := c.HasDefaultRoutes()
	for _, p := range c.Peers {
		for _, a := range p.AllowedIPs {
			pre, err := netip.ParsePrefix(a)
			if err != nil || pre.Bits() == 0 {
				continue // default routes handled below
			}
			if err := routeCmd("-n", "add", "-net", a, "-interface", dev); err != nil {
				if strings.Contains(strings.ToLower(err.Error()), "file exists") {
					continue
				}
				return err
			}
		}
	}
	// Full tunnel as two /1 routes (wg-quick style); the endpoint exception
	// added above keeps the tunnel's own transport on the physical route.
	if v4 {
		for _, p := range []string{"0.0.0.0/1", "128.0.0.0/1"} {
			_ = routeCmd("-n", "add", "-net", p, "-interface", dev)
		}
	}
	if v6 {
		for _, p := range []string{"::/1", "8000::/1"} {
			_ = routeCmd("-n", "add", "-inet6", p, "-interface", dev)
		}
	}
	return nil
}

// addEndpointExceptions pins each peer endpoint to the current default gateway
// so the full-tunnel /1 routes cannot capture the tunnel's own transport. It
// must run before the default routes are installed; returning an error aborts
// the full tunnel rather than risk a loop that cuts the machine off.
func addEndpointExceptions(c *Conf) error {
	v4, v6 := c.HasDefaultRoutes()
	if !v4 && !v6 {
		return nil
	}
	ips := c.EndpointIPs()
	if len(ips) == 0 {
		return fmt.Errorf("cannot resolve peer endpoint; refusing to install full-tunnel routes")
	}
	gw := defaultGateway()
	if gw == "" {
		return fmt.Errorf("no default gateway found for the endpoint exception")
	}
	for _, ip := range ips {
		// Clear any stale route from a previous run before pinning it.
		_ = routeCmd("-n", "delete", "-host", ip.String())
		_ = routeCmd("-n", "add", "-host", ip.String(), gw)
	}
	return nil
}

func removeEndpointExceptions(c *Conf) {
	if c == nil {
		return
	}
	for _, ip := range c.EndpointIPs() {
		_ = routeCmd("-n", "delete", "-host", ip.String())
	}
}

// defaultGateway returns the current IPv4/IPv6 default gateway.
func defaultGateway() string {
	out, err := exec.Command("route", "-n", "get", "default").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "gateway:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "gateway:"))
		}
	}
	return ""
}

// addTunnelPrefix installs a route for pre through the tunnel device.
func addTunnelPrefix(dev string, pre netip.Prefix) error {
	if pre.Addr().Is4() {
		return routeCmd("-n", "add", "-net", pre.String(), "-interface", dev)
	}
	return routeCmd("-n", "add", "-inet6", "-net", pre.String(), "-interface", dev)
}

// delTunnelPrefix removes a route previously installed by addTunnelPrefix.
func delTunnelPrefix(_ string, pre netip.Prefix) {
	if pre.Addr().Is4() {
		_ = routeCmd("-n", "delete", "-net", pre.String())
		return
	}
	_ = routeCmd("-n", "delete", "-inet6", "-net", pre.String())
}

// currentDNSServers returns the system's DNS server IPs, used as the upstream
// for the split-DNS proxy before the system resolver is pointed at it.
func currentDNSServers() []string {
	out, err := exec.Command("scutil", "--dns").Output()
	if err != nil {
		return nil
	}
	var ips []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "nameserver[") {
			continue
		}
		if i := strings.Index(line, ":"); i >= 0 {
			f := strings.TrimSpace(line[i+1:])
			if _, err := netip.ParseAddr(f); err == nil {
				ips = append(ips, f)
			}
		}
	}
	return ips
}
