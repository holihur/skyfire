//go:build linux

package driver

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"skyfire/internal/blacklist"
)

// applyBlacklistFirewall enforces the block list for OS-stack drivers: client
// transit traffic (entering the interface) to blocked addresses is dropped in
// the FORWARD chain through a dedicated per-interface chain, so only tunnel
// traffic is affected and the rules are easy to remove.
func applyBlacklistFirewall(dev string, entries []string) error {
	v4, v6 := splitBlacklist(entries)
	chain := blChainName(dev)
	return errors.Join(
		syncBLChain("iptables", dev, chain, v4),
		syncBLChain("ip6tables", dev, chain, v6),
	)
}

// splitBlacklist separates address/prefix entries by family. Domain entries
// are enforced by the DNS proxy, not the firewall, and are ignored here.
func splitBlacklist(entries []string) (v4, v6 []string) {
	for _, p := range blacklist.Compile(entries).Prefixes() {
		if p.Addr().Is4() {
			v4 = append(v4, p.String())
		} else {
			v6 = append(v6, p.String())
		}
	}
	return v4, v6
}

func syncBLChain(bin, dev, chain string, cidrs []string) error {
	if len(cidrs) == 0 {
		return removeBLChain(bin, dev, chain)
	}
	if _, err := exec.LookPath(bin); err != nil {
		return ErrBlacklistUnsupported
	}
	_ = runFW(bin, "-N", chain) // ignore "chain already exists"
	if err := runFW(bin, "-F", chain); err != nil {
		return err
	}
	for _, c := range cidrs {
		if err := runFW(bin, "-A", chain, "-d", c, "-j", "DROP"); err != nil {
			return err
		}
	}
	// Ensure exactly one jump from FORWARD for traffic entering the interface.
	if runFW(bin, "-C", "FORWARD", "-i", dev, "-j", chain) != nil {
		if err := runFW(bin, "-I", "FORWARD", "1", "-i", dev, "-j", chain); err != nil {
			return err
		}
	}
	return nil
}

func removeBLChain(bin, dev, chain string) error {
	if _, err := exec.LookPath(bin); err != nil {
		return nil
	}
	for i := 0; i < 8; i++ { // drop duplicate jumps if any
		if runFW(bin, "-D", "FORWARD", "-i", dev, "-j", chain) != nil {
			break
		}
	}
	_ = runFW(bin, "-F", chain)
	_ = runFW(bin, "-X", chain)
	return nil
}

func runFW(bin string, args ...string) error {
	out, err := exec.Command(bin, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w: %s", bin, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// blChainName derives a valid iptables chain name (max 28 chars) from an
// interface name.
func blChainName(dev string) string {
	var b strings.Builder
	b.WriteString("SKY_BL_")
	for _, r := range dev {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		}
	}
	name := b.String()
	if len(name) > 28 {
		name = name[:28]
	}
	return name
}
