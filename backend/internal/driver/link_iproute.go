//go:build !windows

package driver

import (
	"fmt"
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

func atoiOr(s string, def int) int {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	if err != nil {
		return def
	}
	return n
}
