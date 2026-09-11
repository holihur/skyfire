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

// unconfigureLink is a no-op: routes are interface scoped and disappear with
// the utun device. DNS is intentionally left untouched.
func unconfigureLink(string, *Conf) {}

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
	// Full tunnel as two /1 routes (wg-quick style) so the real default route
	// stays available for the endpoint. Endpoint exception not implemented.
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
