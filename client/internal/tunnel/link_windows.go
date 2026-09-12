//go:build windows

package tunnel

import (
	"fmt"
	"net/netip"
	"os/exec"
	"strings"
)

// tunName is the Wintun adapter name (wintun.dll must ship next to the exe).
func tunName() string { return "Skyfire" }

// netsh runs a netsh command line.
func netsh(args ...string) error {
	out, err := exec.Command("netsh", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("netsh %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// ignoreExists tolerates "object already exists" style netsh errors.
func ignoreExists(err error) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "already") || strings.Contains(msg, "exists") || strings.Contains(msg, "duplicate") {
		return nil
	}
	return err
}

func configureLink(dev string, c *Conf) error {
	if err := addAddresses(dev, c.Addresses); err != nil {
		return err
	}
	if c.MTU > 0 {
		if err := netsh("interface", "ipv4", "set", "subinterface", dev, fmt.Sprintf("mtu=%d", c.MTU)); err != nil {
			return err
		}
	}
	if err := netsh("interface", "set", "interface", dev, "admin=enable"); err != nil {
		return err
	}
	if err := addRoutes(dev, c); err != nil {
		return err
	}
	return nil
}

// configureDNS pushes the tunnel DNS servers onto the Wintun adapter. It is
// called separately from configureLink so a DNS failure never tears the tunnel
// down.
func configureDNS(dev string, dns []string) error {
	if len(dns) == 0 {
		return nil
	}
	return setDNS(dev, dns)
}

// unconfigureDNS restores DHCP-assigned DNS.
func unconfigureDNS(dev string, dns []string) {
	if len(dns) == 0 {
		return
	}
	_ = netsh("interface", "ipv4", "set", "dnsservers", "name="+dev, "source=dhcp")
	_ = netsh("interface", "ipv6", "set", "dnsservers", "name="+dev, "source=dhcp")
}

// unconfigureLink removes the endpoint-exception routes; the adapter's own
// routes are removed when the Wintun device is closed and DNS is restored by
// unconfigureDNS.
func unconfigureLink(_ string, c *Conf) {
	removeEndpointExceptions(c)
}

func addAddresses(dev string, addresses []string) error {
	for _, addr := range addresses {
		p, err := netip.ParsePrefix(addr)
		if err != nil {
			return fmt.Errorf("invalid address %q: %w", addr, err)
		}
		if p.Addr().Is4() {
			mask := p.Masked().Bits()
			var nm [4]byte
			for i := 0; i < 4; i++ {
				bits := mask - 8*i
				if bits > 8 {
					bits = 8
				}
				if bits < 0 {
					bits = 0
				}
				nm[i] = byte((0xff << (8 - bits)) & 0xff)
			}
			err := netsh("interface", "ipv4", "add", "address", dev,
				"address="+p.Addr().String(),
				fmt.Sprintf("mask=%d.%d.%d.%d", nm[0], nm[1], nm[2], nm[3]))
			if err := ignoreExists(err); err != nil {
				return err
			}
		} else {
			err := netsh("interface", "ipv6", "add", "address", dev,
				fmt.Sprintf("address=%s/%d", p.Addr().String(), p.Bits()))
			if err := ignoreExists(err); err != nil {
				return err
			}
		}
	}
	return nil
}

func addRoutes(dev string, c *Conf) error {
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
			if err != nil {
				continue
			}
			if pre.Addr().Is4() && pre.Bits() == 0 {
				continue // handled below as two /1 routes
			}
			if !pre.Addr().Is4() && pre.Bits() == 0 {
				continue
			}
			fam := "ipv6"
			if pre.Addr().Is4() {
				fam = "ipv4"
			}
			if err := ignoreExists(netsh("interface", fam, "add", "route", "prefix="+a, "interface="+dev)); err != nil {
				return err
			}
		}
	}
	// A full tunnel is installed as two /1 routes so the physical default
	// route keeps existing; the endpoint exception added above keeps the
	// tunnel's own transport on that physical route.
	if v4 {
		if err := ignoreExists(netsh("interface", "ipv4", "add", "route", "prefix=0.0.0.0/1", "interface="+dev)); err != nil {
			return err
		}
		if err := ignoreExists(netsh("interface", "ipv4", "add", "route", "prefix=128.0.0.0/1", "interface="+dev)); err != nil {
			return err
		}
	}
	if v6 {
		if err := ignoreExists(netsh("interface", "ipv6", "add", "route", "prefix=::/1", "interface="+dev)); err != nil {
			return err
		}
		if err := ignoreExists(netsh("interface", "ipv6", "add", "route", "prefix=8000::/1", "interface="+dev)); err != nil {
			return err
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
	gw := ""
	if v4 {
		gw = defaultGatewayV4()
		if gw == "" {
			return fmt.Errorf("no IPv4 default gateway found for the endpoint exception")
		}
	}
	for _, ip := range ips {
		if ip.Is4() && gw != "" {
			_ = exec.Command("route", "add", ip.String(), "mask", "255.255.255.255", gw, "metric", "1").Run()
		}
	}
	return nil
}

func removeEndpointExceptions(c *Conf) {
	if c == nil {
		return
	}
	for _, ip := range c.EndpointIPs() {
		if ip.Is4() {
			_ = exec.Command("route", "delete", ip.String()).Run()
		}
	}
}

// defaultGatewayV4 returns the current IPv4 default gateway, preferring
// PowerShell's Get-NetRoute and falling back to parsing `route print`.
func defaultGatewayV4() string {
	out, err := exec.Command("powershell", "-NoProfile", "-Command",
		"(Get-NetRoute -DestinationPrefix '0.0.0.0/0' | Sort-Object RouteMetric | Select-Object -First 1).NextHop").Output()
	if err == nil {
		if gw := strings.TrimSpace(string(out)); gw != "" {
			return gw
		}
	}
	out, err = exec.Command("route", "print", "-4").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) >= 3 && f[0] == "0.0.0.0" && f[1] == "0.0.0.0" {
			return f[2]
		}
	}
	return ""
}

func setDNS(dev string, dns []string) error {
	var v4, v6 []string
	for _, d := range dns {
		ip, err := netip.ParseAddr(d)
		if err != nil {
			continue
		}
		if ip.Is4() {
			v4 = append(v4, d)
		} else {
			v6 = append(v6, d)
		}
	}
	if len(v4) > 0 {
		if err := netsh("interface", "ipv4", "set", "dnsservers", "name="+dev, "static", v4[0], "primary"); err != nil {
			return err
		}
		for i, ip := range v4[1:] {
			if err := netsh("interface", "ipv4", "add", "dnsservers", "name="+dev, ip, fmt.Sprintf("index=%d", i+2)); err != nil {
				return err
			}
		}
	}
	if len(v6) > 0 {
		if err := netsh("interface", "ipv6", "set", "dnsservers", "name="+dev, "static", v6[0], "primary"); err != nil {
			return err
		}
		for i, ip := range v6[1:] {
			if err := netsh("interface", "ipv6", "add", "dnsservers", "name="+dev, ip, fmt.Sprintf("index=%d", i+2)); err != nil {
				return err
			}
		}
	}
	return nil
}
