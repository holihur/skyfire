//go:build windows

package driver

import (
	"fmt"
	"net/netip"
	"os/exec"
	"strings"
)

// The userspace driver on Windows drives Wintun adapters. Address/MTU/link
// bookkeeping is done with netsh(8), which is always present. The adapter
// itself is destroyed by tun.Device.Close(), so delWGLink is a no-op.

// netsh runs a netsh command line.
func netsh(args ...string) error {
	out, err := exec.Command("netsh", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("netsh %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func addAddresses(dev string, addresses []string) error {
	for _, addr := range addresses {
		p, err := netip.ParsePrefix(addr)
		if err != nil {
			return fmt.Errorf("invalid address %q: %w", addr, err)
		}
		if p.Addr().Is4() {
			mask := netip.PrefixFrom(p.Addr(), p.Bits()).Masked()
			ones := mask.Bits()
			// netsh wants a dotted netmask
			var nm [4]byte
			for i := 0; i < 4; i++ {
				bits := ones - 8*i
				if bits > 8 {
					bits = 8
				}
				if bits < 0 {
					bits = 0
				}
				nm[i] = byte((0xff << (8 - bits)) & 0xff)
			}
			maskStr := fmt.Sprintf("%d.%d.%d.%d", nm[0], nm[1], nm[2], nm[3])
			err := netsh("interface", "ipv4", "add", "address", dev,
				fmt.Sprintf("address=%s", p.Addr().String()),
				fmt.Sprintf("mask=%s", maskStr))
			if err != nil && !strings.Contains(err.Error(), "already") {
				return err
			}
		} else {
			err := netsh("interface", "ipv6", "add", "address", dev,
				fmt.Sprintf("address=%s/%d", p.Addr().String(), p.Bits()))
			if err != nil && !strings.Contains(err.Error(), "already") {
				return err
			}
		}
	}
	return nil
}

func setLinkMTU(dev string, mtu int) error {
	if mtu <= 0 {
		return nil
	}
	if err := netsh("interface", "ipv4", "set", "subinterface", dev, fmt.Sprintf("mtu=%d", mtu)); err != nil {
		return err
	}
	return netsh("interface", "ipv6", "set", "subinterface", dev, fmt.Sprintf("mtu=%d", mtu))
}

func setLinkUp(dev string) error {
	return netsh("interface", "set", "interface", dev, "admin=enable")
}

func setLinkDown(dev string) error {
	return netsh("interface", "set", "interface", dev, "admin=disable")
}

func delWGLink(dev string) error { return nil }

// On Windows, route and forwarding management is not implemented yet: the
// Wintun adapter created by wireguard-go is torn down together with the
// process, which removes its routes anyway.
func EnsureForwarding() error { return nil }

func AddRoutes(dev string, allowed []string) error { return nil }

func RemoveRoutes(dev string, allowed []string) {}

// Full-tunnel policy routing is not implemented on Windows yet; the Wintun
// adapter is process-scoped and its routes disappear with it.
func AddDefaultRoutes(dev string, v4, v6 bool) error { return nil }

func RemoveDefaultRoutes(dev string, v4, v6 bool) {}
