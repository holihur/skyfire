//go:build linux

package driver

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// run executes a privileged ip(8) command. The daemon is expected to run as
// root (or with CAP_NET_ADMIN). It never falls back to sudo implicitly.
func iprun(args ...string) error {
	cmd := exec.Command("ip", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ip %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func ipIsLinkPresent(name string) bool {
	out, err := exec.Command("ip", "link", "show", "dev", name).CombinedOutput()
	if err != nil {
		return false
	}
	return len(out) > 0
}

// addAddresses idempotently assigns CIDR addresses to a link.
func addAddresses(dev string, addresses []string) error {
	for _, a := range addresses {
		if err := iprun("addr", "add", a, "dev", dev); err != nil {
			// tolerate EEXIST: the address is already assigned
			if strings.Contains(err.Error(), "File exists") {
				continue
			}
			return err
		}
	}
	return nil
}

func setLinkMTU(dev string, mtu int) error {
	return iprun("link", "set", "dev", dev, "mtu", fmt.Sprint(mtu))
}

func setLinkUp(dev string) error {
	return iprun("link", "set", "dev", dev, "up")
}

func setLinkDown(dev string) error {
	return iprun("link", "set", "dev", dev, "down")
}

func addWGLink(dev, mtu string) error {
	if ipIsLinkPresent(dev) {
		if mtu != "" {
			return setLinkMTU(dev, atoiOr(mtu, 0))
		}
		return nil
	}
	if err := iprun("link", "add", "dev", dev, "type", "wireguard"); err != nil {
		return err
	}
	if mtu != "" {
		if err := setLinkMTU(dev, atoiOr(mtu, 0)); err != nil {
			return err
		}
	}
	return nil
}

func delWGLink(dev string) error {
	if !ipIsLinkPresent(dev) {
		return nil
	}
	return iprun("link", "delete", "dev", dev)
}

// EnsureForwarding enables IPv4 and IPv6 packet forwarding, the same thing
// wg-quick does before bringing up a routed tunnel. Best-effort per family:
// a missing IPv6 stack (read error) is tolerated, but a failing write is
// reported. Requires root / CAP_NET_ADMIN.
func EnsureForwarding() error {
	for _, p := range []string{
		"/proc/sys/net/ipv4/ip_forward",
		"/proc/sys/net/ipv6/conf/all/forwarding",
	} {
		cur, err := os.ReadFile(p)
		if err != nil {
			continue // family unavailable (e.g. IPv6 disabled); skip it
		}
		if strings.TrimSpace(string(cur)) == "1" {
			continue
		}
		if err := os.WriteFile(p, []byte("1\n"), 0o644); err != nil {
			return fmt.Errorf("enable forwarding via %s: %w", p, err)
		}
	}
	return nil
}

// AddRoutes installs one route per AllowedIP onto the tunnel device, like
// wg-quick does. Idempotent: an existing route is not an error.
func AddRoutes(dev string, allowed []string) error {
	for _, a := range RouteTargets(allowed) {
		if err := iprun("route", "add", a, "dev", dev); err != nil && !strings.Contains(err.Error(), "File exists") {
			return err
		}
	}
	return nil
}

// RemoveRoutes deletes the routes previously installed by AddRoutes. Routes
// are removed best-effort; a missing route is not an error.
func RemoveRoutes(dev string, allowed []string) {
	for _, a := range RouteTargets(allowed) {
		_ = iprun("route", "del", a, "dev", dev)
	}
}

// routeTable is the dedicated policy-routing table used for full-tunnel
// default routes. Same number wg-quick picks (51820).
const routeTable = "51820"

// AddDefaultRoutes sets up policy routing so a default route through the
// tunnel can coexist with the system's real default route (which stays
// needed for the tunnel endpoint's own UDP traffic). Mirrors wg-quick:
//
//	ip route add 0.0.0.0/0 dev wg0 table 51820
//	ip rule add not fwmark 51820 table 51820
//	ip rule add table main suppress_prefixlength 0
//
// The device must carry fwmark 51820 (set via driver.Config.FirewallMark) so
// its own endpoint packets skip the tunnel table and avoid a routing loop.
// Only the requested families are touched; IPv6 is skipped when the host has
// no IPv6 support. Idempotent.
func AddDefaultRoutes(dev string, v4, v6 bool) error {
	if v4 {
		if err := iprun("route", "add", "0.0.0.0/0", "dev", dev, "table", routeTable); err != nil && !strings.Contains(err.Error(), "File exists") {
			return err
		}
		if err := iprun("rule", "add", "not", "fwmark", routeTable, "table", routeTable); err != nil && !strings.Contains(err.Error(), "File exists") {
			return err
		}
		if err := iprun("rule", "add", "table", "main", "suppress_prefixlength", "0"); err != nil && !strings.Contains(err.Error(), "File exists") {
			return err
		}
	}
	if v6 {
		if err := iprun("-6", "route", "add", "::/0", "dev", dev, "table", routeTable); err != nil && !strings.Contains(err.Error(), "File exists") {
			return err
		}
		if err := iprun("-6", "rule", "add", "not", "fwmark", routeTable, "table", routeTable); err != nil && !strings.Contains(err.Error(), "File exists") {
			return err
		}
		if err := iprun("-6", "rule", "add", "table", "main", "suppress_prefixlength", "0"); err != nil && !strings.Contains(err.Error(), "File exists") {
			return err
		}
	}
	return nil
}

// RemoveDefaultRoutes tears down the policy routing installed by
// AddDefaultRoutes. Best-effort: errors are ignored.
func RemoveDefaultRoutes(dev string, v4, v6 bool) {
	if v4 {
		_ = iprun("route", "del", "0.0.0.0/0", "dev", dev, "table", routeTable)
		_ = iprun("rule", "del", "not", "fwmark", routeTable, "table", routeTable)
		_ = iprun("rule", "del", "table", "main", "suppress_prefixlength", "0")
	}
	if v6 {
		_ = iprun("-6", "route", "del", "::/0", "dev", dev, "table", routeTable)
		_ = iprun("-6", "rule", "del", "not", "fwmark", routeTable, "table", routeTable)
		_ = iprun("-6", "rule", "del", "table", "main", "suppress_prefixlength", "0")
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
