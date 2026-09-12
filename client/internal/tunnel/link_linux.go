//go:build linux

package tunnel

import (
	"fmt"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
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

	// Split-by-domain mode: routes are maintained by the whitelist router
	// instead of the peer's AllowedIPs.
	if c.WhitelistMode() {
		return nil
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
	if c == nil || c.WhitelistMode() {
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

// resolvConfPath is the system resolver configuration; resolvBackupPath keeps
// the pre-tunnel content when we have to edit it directly.
const (
	resolvConfPath   = "/etc/resolv.conf"
	resolvBackupPath = "/run/skyfire-client/resolv.conf.bak"
)

// configureDNS points the system resolver at this tunnel's DNS servers,
// preferring the least invasive backend available: systemd-resolved (per-link),
// then resolvconf, then a direct /etc/resolv.conf edit with backup.
func configureDNS(dev string, servers []string) error {
	if len(servers) == 0 {
		return nil
	}
	if haveCommand("resolvectl") && systemdResolvedActive() {
		args := append([]string{"dns", dev}, servers...)
		if err := runCmd("resolvectl", args...); err != nil {
			return err
		}
		// "~." makes this link the default routing domain for all lookups.
		return runCmd("resolvectl", "domain", dev, "~.")
	}
	if resolvconf, err := exec.LookPath("resolvconf"); err == nil {
		cmd := exec.Command(resolvconf, "-a", dev)
		cmd.Stdin = strings.NewReader(resolvconfBody(servers))
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("resolvconf -a %s: %w (%s)", dev, err, strings.TrimSpace(string(out)))
		}
		return nil
	}
	return writeResolvConf(servers)
}

// unconfigureDNS reverses configureDNS by re-detecting the same backend.
func unconfigureDNS(dev string, servers []string) {
	if len(servers) == 0 {
		return
	}
	if haveCommand("resolvectl") && systemdResolvedActive() {
		_ = runCmd("resolvectl", "revert", dev)
		return
	}
	if resolvconf, err := exec.LookPath("resolvconf"); err == nil {
		_ = exec.Command(resolvconf, "-d", dev).Run()
		return
	}
	restoreResolvConf()
}

func resolvconfBody(servers []string) string {
	var b strings.Builder
	for _, s := range servers {
		fmt.Fprintf(&b, "nameserver %s\n", s)
	}
	return b.String()
}

// writeResolvConf edits /etc/resolv.conf directly, saving the original once so
// restoreResolvConf can put it back.
func writeResolvConf(servers []string) error {
	if _, err := os.Stat(resolvBackupPath); err != nil {
		if data, rerr := os.ReadFile(resolvConfPath); rerr == nil {
			if merr := os.MkdirAll(filepath.Dir(resolvBackupPath), 0o755); merr == nil {
				_ = os.WriteFile(resolvBackupPath, data, 0o644)
			}
		}
	}
	return os.WriteFile(resolvConfPath, []byte(resolvconfBody(servers)), 0o644)
}

func restoreResolvConf() {
	data, err := os.ReadFile(resolvBackupPath)
	if err != nil {
		return
	}
	_ = os.WriteFile(resolvConfPath, data, 0o644)
	_ = os.Remove(resolvBackupPath)
}

func haveCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func systemdResolvedActive() bool {
	out, err := exec.Command("systemctl", "is-active", "systemd-resolved").Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "active"
}

func runCmd(name string, args ...string) error {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w (%s)", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// addTunnelPrefix installs a route for pre through the tunnel device.
func addTunnelPrefix(dev string, pre netip.Prefix) error {
	if pre.Addr().Is4() {
		return ipignore("route", "add", pre.String(), "dev", dev)
	}
	return ipignore("-6", "route", "add", pre.String(), "dev", dev)
}

// delTunnelPrefix removes a route previously installed by addTunnelPrefix.
func delTunnelPrefix(dev string, pre netip.Prefix) {
	if pre.Addr().Is4() {
		_ = iprun("route", "del", pre.String(), "dev", dev)
		return
	}
	_ = iprun("-6", "route", "del", pre.String(), "dev", dev)
}

// currentDNSServers returns the system's DNS server IPs, used as the upstream
// for the split-DNS proxy before the system resolver is pointed at it.
func currentDNSServers() []string {
	// resolvectl reports the real per-link upstreams (resolv.conf may just be
	// the 127.0.0.53 stub, which would loop back into our own proxy).
	if out, err := exec.Command("resolvectl", "dns").Output(); err == nil {
		var ips []string
		for _, line := range strings.Split(string(out), "\n") {
			for _, f := range strings.Fields(line) {
				if _, err := netip.ParseAddr(f); err == nil {
					ips = append(ips, f)
				}
			}
		}
		if len(ips) > 0 {
			return ips
		}
	}
	data, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		return nil
	}
	var ips []string
	for _, line := range strings.Split(string(data), "\n") {
		f := strings.Fields(line)
		if len(f) >= 2 && f[0] == "nameserver" {
			if _, err := netip.ParseAddr(f[1]); err == nil {
				ips = append(ips, f[1])
			}
		}
	}
	return ips
}
