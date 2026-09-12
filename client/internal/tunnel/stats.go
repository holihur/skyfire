package tunnel

import (
	"strconv"
	"strings"
	"time"
)

// Stats is a snapshot of WireGuard device state read from the UAPI: link
// liveness plus transfer counters. Counters are from the client's point of
// view, so RxBytes is what the tunnel received (download) and TxBytes is what
// it sent (upload).
type Stats struct {
	// Last is the most recent handshake across all peers (zero when no peer
	// has completed a handshake yet).
	Last time.Time
	// HasKeepalive is true when at least one peer has persistent keepalive
	// enabled, which lets a caller tell an idle-but-healthy tunnel apart from a
	// dropped one.
	HasKeepalive bool
	// HasEndpoint is true when at least one peer has learned a remote endpoint.
	HasEndpoint bool
	// RxBytes and TxBytes are the total bytes received from and sent to all
	// peers since the device was created.
	RxBytes uint64
	TxBytes uint64
}

// Stats reads device state from the running tunnel. ok is false when the
// tunnel is down or the device cannot be queried.
func (t *Tunnel) Stats() (Stats, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.dev == nil {
		return Stats{}, false
	}
	out, err := t.dev.IpcGet()
	if err != nil {
		return Stats{}, false
	}
	return parseStats(out), true
}

// parseStats extracts handshake, keepalive and transfer counters from a
// WireGuard UAPI dump as produced by device.IpcGet.
func parseStats(uapi string) Stats {
	var info Stats
	var sec, nsec int64
	inPeer := false
	flush := func() {
		if inPeer && sec > 0 {
			if ts := time.Unix(sec, nsec); ts.After(info.Last) {
				info.Last = ts
			}
		}
	}
	for _, line := range strings.Split(uapi, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, val, _ := strings.Cut(line, "=")
		switch key {
		case "public_key":
			flush()
			inPeer = true
			sec, nsec = 0, 0
		case "last_handshake_time_sec":
			sec, _ = strconv.ParseInt(val, 10, 64)
		case "last_handshake_time_nsec":
			nsec, _ = strconv.ParseInt(val, 10, 64)
		case "persistent_keepalive_interval":
			if n, _ := strconv.Atoi(val); n > 0 {
				info.HasKeepalive = true
			}
		case "endpoint":
			if val != "" {
				info.HasEndpoint = true
			}
		case "rx_bytes":
			if n, err := strconv.ParseUint(val, 10, 64); err == nil {
				info.RxBytes += n
			}
		case "tx_bytes":
			if n, err := strconv.ParseUint(val, 10, 64); err == nil {
				info.TxBytes += n
			}
		}
	}
	flush()
	return info
}
