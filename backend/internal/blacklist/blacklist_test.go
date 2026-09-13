package blacklist

import (
	"net/netip"
	"testing"
)

func TestMatchIP(t *testing.T) {
	m := Compile([]string{
		"10.0.0.0/8",
		"203.0.113.9",
		"2001:db8::/32",
		"# comment",
		"",
		"ads.example.com", // domain, must not match IPs
	})
	cases := []struct {
		ip   string
		want bool
	}{
		{"10.1.2.3", true},
		{"10.0.0.1", true},
		{"11.0.0.1", false},
		{"203.0.113.9", true},
		{"203.0.113.10", false},
		{"2001:db8::1", true},
		{"2001:db9::1", false},
	}
	for _, c := range cases {
		if got := m.MatchIP(netip.MustParseAddr(c.ip)); got != c.want {
			t.Errorf("MatchIP(%s) = %v, want %v", c.ip, got, c.want)
		}
	}
}

func TestMatchDomain(t *testing.T) {
	m := Compile([]string{
		"ads.example.com",
		"*.tracker.example",
		"Example.COM.",
	})
	cases := []struct {
		name string
		want bool
	}{
		{"ads.example.com", true},
		{"ADS.EXAMPLE.COM", true},
		{"ads.example.com.", true},
		{"sub.ads.example.com", false}, // exact only
		{"a.tracker.example", true},
		{"deep.a.tracker.example", true},
		{"tracker.example", false}, // wildcard requires a subdomain
		{"example.com", true},      // trailing dot normalised
		{"other.com", false},
	}
	for _, c := range cases {
		if got := m.MatchDomain(c.name); got != c.want {
			t.Errorf("MatchDomain(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestEmpty(t *testing.T) {
	if !Compile(nil).Empty() {
		t.Error("Compile(nil) should be empty")
	}
	var nilM *Matcher
	if !nilM.Empty() {
		t.Error("nil matcher should be empty")
	}
	if nilM.MatchIP(netip.MustParseAddr("1.2.3.4")) || nilM.MatchDomain("x.com") {
		t.Error("nil matcher should match nothing")
	}
	if Compile([]string{"10.0.0.0/8"}).Empty() {
		t.Error("matcher with a prefix should not be empty")
	}
}
