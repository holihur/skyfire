//go:build darwin

package driver

import (
	"fmt"
	"net"
	"net/netip"
	"os/exec"
	"strings"
)

func ifconfig(dev string, args ...string) error {
	cmdArgs := append([]string{dev}, args...)
	out, err := exec.Command("ifconfig", cmdArgs...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("ifconfig %s: %w (%s)", strings.Join(cmdArgs, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func route(args ...string) error {
	out, err := exec.Command("route", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("route %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
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
			err = ifconfig(dev, "inet", local, mask, local, "netmask", mask)
		} else {
			err = ifconfig(dev, "inet6", fmt.Sprintf("%s/%d", local, p.Bits()))
		}
		if err != nil {
			if strings.Contains(err.Error(), "File exists") || strings.Contains(err.Error(), "already") {
				continue
			}
			return err
		}
	}
	return nil
}

func setLinkMTU(dev string, mtu int) error {
	return ifconfig(dev, "mtu", fmt.Sprint(mtu))
}

func setLinkUp(dev string) error {
	return ifconfig(dev, "up")
}

func setLinkDown(dev string) error {
	return ifconfig(dev, "down")
}

func addWGLink(dev, mtu string) error { return nil }

func delWGLink(dev string) error { return nil }

func EnsureForwarding() error { return nil }

func AddRoutes(dev string, allowed []string) error {
	for _, a := range RouteTargets(allowed) {
		if err := route("-add", "-net", a, "-interface", dev); err != nil && !strings.Contains(err.Error(), "File exists") {
			return err
		}
	}
	return nil
}

func RemoveRoutes(dev string, allowed []string) {
	for _, a := range RouteTargets(allowed) {
		_ = route("-delete", "-net", a, "-interface", dev)
	}
}

func AddDefaultRoutes(dev string, v4, v6 bool) error {
	if v4 {
		if err := route("-add", "-net", "0.0.0.0/1", "-interface", dev); err != nil && !strings.Contains(err.Error(), "File exists") {
			return err
		}
		if err := route("-add", "-net", "128.0.0.0/1", "-interface", dev); err != nil && !strings.Contains(err.Error(), "File exists") {
			return err
		}
	}
	if v6 {
		if err := route("-add", "-inet6", "::/1", "-interface", dev); err != nil && !strings.Contains(err.Error(), "File exists") {
			return err
		}
		if err := route("-add", "-inet6", "8000::/1", "-interface", dev); err != nil && !strings.Contains(err.Error(), "File exists") {
			return err
		}
	}
	return nil
}

func RemoveDefaultRoutes(dev string, v4, v6 bool) {
	if v4 {
		_ = route("-delete", "-net", "0.0.0.0/1", "-interface", dev)
		_ = route("-delete", "-net", "128.0.0.0/1", "-interface", dev)
	}
	if v6 {
		_ = route("-delete", "-inet6", "::/1", "-interface", dev)
		_ = route("-delete", "-inet6", "8000::/1", "-interface", dev)
	}
}

func atoiOr(s string, def int) int {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	if err != nil {
		return def
	}
	return n
}
