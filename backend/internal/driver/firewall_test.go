//go:build linux

package driver

import (
	"strings"
	"testing"
)

func TestBLChainName(t *testing.T) {
	if got := blChainName("wg0"); got != "SKY_BL_wg0" {
		t.Errorf("blChainName(wg0) = %q", got)
	}
	if got := blChainName("a/b c-d"); got != "SKY_BL_abcd" {
		t.Errorf("blChainName sanitisation = %q", got)
	}
	if got := blChainName(strings.Repeat("x", 60)); len(got) > 28 {
		t.Errorf("chain name too long: %d", len(got))
	}
}

func TestSplitBlacklist(t *testing.T) {
	v4, v6 := splitBlacklist([]string{
		"10.0.0.0/8",
		"203.0.113.9",
		"2001:db8::/32",
		"ads.example.com", // domain: not a firewall rule
	})
	if len(v4) != 2 {
		t.Errorf("v4 = %v, want 2 entries", v4)
	}
	if len(v6) != 1 {
		t.Errorf("v6 = %v, want 1 entry", v6)
	}
}
