package tunnel

import (
	"net/netip"
	"testing"
)

func TestParseWhitelist(t *testing.T) {
	rules := parseWhitelist([]string{
		"10.0.0.0/8",
		"192.168.1.5",
		"2001:db8::/32",
		"api.example.com",
		"*.example.com",
		"*.Example.COM",
		"",
		"  ",
	})
	if len(rules) != 6 {
		t.Fatalf("want 6 rules, got %d: %+v", len(rules), rules)
	}

	if rules[0].kind != rulePrefix || rules[0].prefix.String() != "10.0.0.0/8" {
		t.Fatalf("CIDR rule wrong: %+v", rules[0])
	}
	if rules[1].kind != rulePrefix || rules[1].prefix != netip.PrefixFrom(netip.MustParseAddr("192.168.1.5"), 32) {
		t.Fatalf("bare IP rule wrong: %+v", rules[1])
	}
	if rules[2].kind != rulePrefix || rules[2].prefix.String() != "2001:db8::/32" {
		t.Fatalf("IPv6 CIDR rule wrong: %+v", rules[2])
	}
	if rules[3].kind != ruleExact || rules[3].suffix != "api.example.com" {
		t.Fatalf("exact rule wrong: %+v", rules[3])
	}
	if rules[4].kind != ruleWildcard || rules[4].suffix != "example.com" {
		t.Fatalf("wildcard rule wrong: %+v", rules[4])
	}
	if rules[5].kind != ruleWildcard || rules[5].suffix != "example.com" {
		t.Fatalf("wildcard dedupe/lowercase wrong: %+v", rules[5])
	}
}

func TestMatchDomain(t *testing.T) {
	rules := parseWhitelist([]string{"api.example.com", "*.corp.example"})

	cases := []struct {
		name string
		want bool
	}{
		{"api.example.com.", true},
		{"API.EXAMPLE.COM", true},
		{"other.example.com", false},
		{"corp.example", true},
		{"mail.corp.example", true},
		{"deep.mail.corp.example", true},
		{"example.com", false},
		{"notcorp.example", false},
	}
	for _, tc := range cases {
		if got := matchDomain(rules, tc.name); got != tc.want {
			t.Fatalf("matchDomain(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestPrefixRulesAndExactDomains(t *testing.T) {
	r := &whitelistRouter{rules: parseWhitelist([]string{"10.0.0.0/8", "api.example.com", "*.example.com"})}
	prefixes := r.prefixRules()
	if len(prefixes) != 1 || prefixes[0].String() != "10.0.0.0/8" {
		t.Fatalf("prefixRules wrong: %v", prefixes)
	}
	exact := r.exactDomains()
	if len(exact) != 1 || exact[0] != "api.example.com" {
		t.Fatalf("exactDomains wrong: %v", exact)
	}
	if !r.hasDomainRules() {
		t.Fatal("hasDomainRules should be true for exact + wildcard")
	}
	if !matchDomain(r.rules, "x.example.com") {
		t.Fatal("wildcard should match subdomain")
	}
}
