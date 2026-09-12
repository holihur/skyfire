package tunnel

import (
	"net/netip"
	"testing"
)

func TestEndpointIPs(t *testing.T) {
	c := &Conf{Peers: []Peer{
		{Endpoint: "203.0.113.7:51820"},   // ipv4 host:port
		{Endpoint: "203.0.113.7:51820"},   // duplicate -> deduped
		{Endpoint: "[2001:db8::1]:51820"}, // ipv6 host:port
		{Endpoint: "198.51.100.9"},        // bare ipv4, no port
	}}
	got := c.EndpointIPs()
	want := map[netip.Addr]bool{
		netip.MustParseAddr("203.0.113.7"):  true,
		netip.MustParseAddr("2001:db8::1"):  true,
		netip.MustParseAddr("198.51.100.9"): true,
	}
	seen := map[netip.Addr]bool{}
	for _, ip := range got {
		if !want[ip] {
			t.Fatalf("unexpected endpoint IP %s", ip)
		}
		if seen[ip] {
			t.Fatalf("duplicate endpoint IP %s", ip)
		}
		seen[ip] = true
	}
	for ip := range want {
		if !seen[ip] {
			t.Fatalf("missing endpoint IP %s (got %v)", ip, got)
		}
	}
}
