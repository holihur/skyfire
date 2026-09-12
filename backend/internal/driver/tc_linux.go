//go:build linux

package driver

import (
	"fmt"
	"hash/fnv"
	"net/netip"
	"os/exec"
	"strings"
)

// Per-peer rate limiting on Linux is implemented with tc(8) traffic control:
//
//   - download (server → peer) is shaped on the egress of the WireGuard/TUN
//     interface with an HTB qdisc plus one leaf class and a flower classifier
//     per limited peer (matching dst_ip = the peer's AllowedIPs).
//   - upload (peer → server) cannot be shaped on ingress directly, so the
//     interface ingress is mirrored into a dedicated IFB device via the
//     mirred action and shaped there on egress, matching src_ip.
//
// Both directions share the same interface: the default HTB class carries
// unlimited traffic, and only peers with a configured limit get a class.

// maxRate is the ceiling for the default (unlimited) HTB class.
const maxRate = "100gbit"

// tcrun executes a privileged tc(8) command.
func tcrun(args ...string) error {
	cmd := exec.Command("tc", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tc %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// ifbFor derives a stable IFB device name from the interface name. IFB names
// are bounded by IFNAMSIZ (15 chars); "ifb" + 8 hex chars of a 32-bit FNV
// hash fits and avoids collisions across differently named interfaces.
func ifbFor(dev string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(dev))
	return fmt.Sprintf("ifb%x", h.Sum32())
}

// rateStr renders bits per second as a tc rate string ("8mbit", "1500000bit").
func rateStr(bps int64) string {
	return fmt.Sprintf("%dbit", bps)
}

// removeTCShaping tears down all shaping previously installed for dev. It is
// best-effort: a missing qdisc or IFB device is not an error.
func removeTCShaping(dev string) {
	_ = tcrun("qdisc", "del", "dev", dev, "root")
	_ = tcrun("qdisc", "del", "dev", dev, "ingress")
	_ = iprun("link", "delete", "dev", ifbFor(dev))
}

// applyTCShaping (re)builds the tc rules for dev from scratch.
func applyTCShaping(dev string, peers []PeerShaping) error {
	removeTCShaping(dev)

	var dl, ul []PeerShaping
	for _, p := range peers {
		if p.DownloadLimit > 0 {
			dl = append(dl, p)
		}
		if p.UploadLimit > 0 {
			ul = append(ul, p)
		}
	}
	if len(dl) == 0 && len(ul) == 0 {
		return nil
	}

	if err := applyEgressShaping(dev, dl); err != nil {
		return err
	}
	return applyIngressShaping(dev, ul)
}

// applyEgressShaping installs the download (server → peer) HTB on dev.
func applyEgressShaping(dev string, peers []PeerShaping) error {
	if len(peers) == 0 {
		return nil
	}
	if err := tcrun("qdisc", "add", "dev", dev, "root", "handle", "1:", "htb", "default", "1"); err != nil {
		return err
	}
	if err := tcrun("class", "add", "dev", dev, "parent", "1:", "classid", "1:1", "htb", "rate", maxRate, "ceil", maxRate); err != nil {
		return err
	}
	for i, p := range peers {
		classid := fmt.Sprintf("1:%d", 100+i)
		if err := tcrun("class", "add", "dev", dev, "parent", "1:", "classid", classid, "htb", "rate", rateStr(p.DownloadLimit), "ceil", rateStr(p.DownloadLimit)); err != nil {
			return err
		}
		for _, pre := range p.Prefixes {
			if err := tcFlowerClassify(dev, "1:", pre, classid, true); err != nil {
				return err
			}
		}
	}
	return nil
}

// applyIngressShaping mirrors dev's ingress into an IFB device and shapes the
// upload (peer → server) direction there.
func applyIngressShaping(dev string, peers []PeerShaping) error {
	if len(peers) == 0 {
		return nil
	}
	ifb := ifbFor(dev)
	if err := iprun("link", "add", "dev", ifb, "type", "ifb"); err != nil {
		return fmt.Errorf("create ifb %s: %w", ifb, err)
	}
	if err := iprun("link", "set", "dev", ifb, "up"); err != nil {
		return err
	}
	if err := tcrun("qdisc", "add", "dev", dev, "handle", "ffff:", "ingress"); err != nil {
		return err
	}
	if err := tcrun("filter", "add", "dev", dev, "parent", "ffff:", "protocol", "all", "u32", "match", "u32", "0", "0", "action", "mirred", "egress", "redirect", "dev", ifb); err != nil {
		return err
	}
	if err := tcrun("qdisc", "add", "dev", ifb, "root", "handle", "1:", "htb", "default", "1"); err != nil {
		return err
	}
	if err := tcrun("class", "add", "dev", ifb, "parent", "1:", "classid", "1:1", "htb", "rate", maxRate, "ceil", maxRate); err != nil {
		return err
	}
	for i, p := range peers {
		classid := fmt.Sprintf("1:%d", 100+i)
		if err := tcrun("class", "add", "dev", ifb, "parent", "1:", "classid", classid, "htb", "rate", rateStr(p.UploadLimit), "ceil", rateStr(p.UploadLimit)); err != nil {
			return err
		}
		for _, pre := range p.Prefixes {
			if err := tcFlowerClassify(ifb, "1:", pre, classid, false); err != nil {
				return err
			}
		}
	}
	return nil
}

// tcFlowerClassify adds a flower classifier for one prefix. dst selects
// dst_ip (download/egress) versus src_ip (upload/ingress). Invalid or
// non-host prefixes are skipped.
func tcFlowerClassify(dev, parent, prefix, classid string, dst bool) error {
	pre, err := netip.ParsePrefix(prefix)
	if err != nil {
		// tolerate bare host addresses (e.g. legacy peer AllowedIPs)
		if a, aerr := netip.ParseAddr(prefix); aerr == nil {
			pre = netip.PrefixFrom(a, a.BitLen())
		} else {
			return fmt.Errorf("rate-limit prefix %q: %w", prefix, err)
		}
	}
	proto := "ip"
	key := "src_ip"
	if dst {
		key = "dst_ip"
	}
	if pre.Addr().Is6() {
		proto = "ipv6"
	}
	return tcrun("filter", "add", "dev", dev, "parent", parent, "protocol", proto, "flower", key, pre.String(), "classid", classid)
}
