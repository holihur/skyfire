//go:build linux

package tunnel

import (
	"fmt"
	"net/netip"
	"os/exec"
	"strconv"
	"strings"
)

// Linux support is provided for local testing of the client. The supported
// desktop targets are Windows and macOS.
func tunName() string { return "skyfire" }

func iprun(args ...string) error {
	out, err := exec.Command("ip", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("ip %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func ipignore(args ...string) error {
	err := iprun(args...)
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "file exists") || strings.Contains(msg, "already") {
		return nil
	}
	return err
}

func configureLink(dev string, c *Conf) error {
	for _, a := range c.Addresses {
		if err := ipignore("addr", "add", a, "dev", dev); err != nil {
			return err
		}
	}
	if c.MTU > 0 {
		if err := iprun("link", "set", "dev", dev, "mtu", strconv.Itoa(c.MTU)); err != nil {
			return err
		}
	}
	if err := iprun("link", "set", "dev", dev, "up"); err != nil {
		return err
	}

	v4, v6 := c.HasDefaultRoutes()
	for _, p := range c.Peers {
		for _, a := range p.AllowedIPs {
			pre, err := netip.ParsePrefix(a)
			if err != nil || pre.Bits() == 0 {
				continue
			}
			if err := ipignore("route", "add", a, "dev", dev); err != nil {
				return err
			}
		}
	}

	// wg-quick style policy routing: the full-tunnel route lives in its own
	// table and the device's fwmark keeps the endpoint's own packets out of
	// it, so no routing loop occurs even when 0.0.0.0/0 is claimed.
	table := strconv.Itoa(routeTable)
	if v4 {
		if err := ipignore("route", "add", "0.0.0.0/0", "dev", dev, "table", table); err != nil {
			return err
		}
		if err := ipignore("rule", "add", "not", "fwmark", table, "table", table); err != nil {
			return err
		}
		if err := ipignore("rule", "add", "table", "main", "suppress_prefixlength", "0"); err != nil {
			return err
		}
	}
	if v6 {
		if err := ipignore("-6", "route", "add", "::/0", "dev", dev, "table", table); err != nil {
			return err
		}
		if err := ipignore("-6", "rule", "add", "not", "fwmark", table, "table", table); err != nil {
			return err
		}
		if err := ipignore("-6", "rule", "add", "table", "main", "suppress_prefixlength", "0"); err != nil {
			return err
		}
	}
	return nil
}

func unconfigureLink(dev string, c *Conf) {
	if c == nil {
		return
	}
	v4, v6 := c.HasDefaultRoutes()
	table := strconv.Itoa(routeTable)
	if v4 {
		_ = iprun("route", "del", "0.0.0.0/0", "dev", dev, "table", table)
		_ = iprun("rule", "del", "not", "fwmark", table, "table", table)
		_ = iprun("rule", "del", "table", "main", "suppress_prefixlength", "0")
	}
	if v6 {
		_ = iprun("-6", "route", "del", "::/0", "dev", dev, "table", table)
		_ = iprun("-6", "rule", "del", "not", "fwmark", table, "table", table)
		_ = iprun("-6", "rule", "del", "table", "main", "suppress_prefixlength", "0")
	}
	for _, p := range c.Peers {
		for _, a := range p.AllowedIPs {
			pre, err := netip.ParsePrefix(a)
			if err != nil || pre.Bits() == 0 {
				continue
			}
			_ = iprun("route", "del", a, "dev", dev)
		}
	}
}
