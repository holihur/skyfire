package tunnel

import (
	"testing"
	"time"
)

func TestParseStats(t *testing.T) {
	uapi := "private_key=aaa\nlisten_port=51820\nfwmark=51821\n" +
		"public_key=peer1\npreshared_key=AAAA\nprotocol_version=1\n" +
		"endpoint=1.2.3.4:51820\n" +
		"last_handshake_time_sec=1000\nlast_handshake_time_nsec=500000000\n" +
		"tx_bytes=10\nrx_bytes=20\npersistent_keepalive_interval=25\n" +
		"allowed_ip=0.0.0.0/0\n" +
		"public_key=peer2\n" +
		"last_handshake_time_sec=2000\nlast_handshake_time_nsec=0\n" +
		"tx_bytes=1\nrx_bytes=2\npersistent_keepalive_interval=0\n"

	h := parseStats(uapi)
	if !h.HasKeepalive {
		t.Error("HasKeepalive = false, want true")
	}
	if !h.HasEndpoint {
		t.Error("HasEndpoint = false, want true")
	}
	if want := time.Unix(2000, 0); !h.Last.Equal(want) {
		t.Errorf("Last = %v, want %v (most recent across peers)", h.Last, want)
	}
	if h.RxBytes != 22 {
		t.Errorf("RxBytes = %d, want 22 (summed across peers)", h.RxBytes)
	}
	if h.TxBytes != 11 {
		t.Errorf("TxBytes = %d, want 11 (summed across peers)", h.TxBytes)
	}
}

func TestParseStatsNoHandshakeNoKeepalive(t *testing.T) {
	uapi := "public_key=peer1\n" +
		"last_handshake_time_sec=0\nlast_handshake_time_nsec=0\n" +
		"tx_bytes=0\nrx_bytes=0\npersistent_keepalive_interval=0\n"

	h := parseStats(uapi)
	if h.HasKeepalive {
		t.Error("HasKeepalive = true, want false")
	}
	if h.HasEndpoint {
		t.Error("HasEndpoint = true, want false")
	}
	if !h.Last.IsZero() {
		t.Errorf("Last = %v, want zero", h.Last)
	}
	if h.RxBytes != 0 || h.TxBytes != 0 {
		t.Errorf("counters = rx %d tx %d, want 0/0", h.RxBytes, h.TxBytes)
	}
}

func TestParseStatsEmpty(t *testing.T) {
	if h := parseStats(""); !h.Last.IsZero() || h.HasKeepalive || h.HasEndpoint || h.RxBytes != 0 || h.TxBytes != 0 {
		t.Errorf("parseStats(empty) = %+v, want zero value", h)
	}
}
