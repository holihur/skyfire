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
	if err := setDNS(dev, c.DNS); err != nil {
		return err
	}
	return nil
}

// unconfigureLink restores DHCP DNS; routes and the adapter itself are
// removed when the Wintun device is closed.
func unconfigureLink(dev string, _ *Conf) {
	_ = netsh("interface", "ipv4", "set", "dnsservers", "name="+dev, "source=dhcp")
	_ = netsh("interface", "ipv6", "set", "dnsservers", "name="+dev, "source=dhcp")
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
	// route (needed by the WireGuard endpoint) keeps existing; this mirrors
	// wg-quick. NOTE: the endpoint exception is not implemented yet, so a
	// full tunnel may not carry its own transport on all configurations.
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
