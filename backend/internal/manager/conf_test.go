package manager

import (
	"strings"
	"testing"

	"skyfire/internal/store"
)

func TestClientConfig(t *testing.T) {
	p := &store.Peer{
		Name:                "phone",
		Address:             "10.42.0.3",
		PrivateKey:          "a",
		PresharedKey:        "b",
		PersistentKeepalive: 25,
		DNS:                 []string{"1.1.1.1"},
	}
	iface := &store.Interface{
		Name:       "wg0",
		PublicKey:  "server-key",
		ListenPort: 51820,
	}
	settings := store.Settings{PublicEndpoint: "vpn.example.com"}

	cfg := ClientConfig(settings, iface, p)
	for _, want := range []string{
		"[Interface]",
		"PrivateKey = a",
		"Address = 10.42.0.3/32",
		"DNS = 1.1.1.1",
		"[Peer]",
		"PublicKey = server-key",
		"PresharedKey = b",
		"AllowedIPs = 0.0.0.0/0, ::/0",
		"Endpoint = vpn.example.com:51820",
		"PersistentKeepalive = 25",
	} {
		if !strings.Contains(cfg, want) {
			t.Fatalf("config missing %q:\n%s", want, cfg)
		}
	}
}

func TestClientEndpointJoinsPort(t *testing.T) {
	ep := clientEndpoint(store.Settings{PublicEndpoint: "vpn.example.com"}, &store.Interface{ListenPort: 51820})
	if ep != "vpn.example.com:51820" {
		t.Fatalf("got %q", ep)
	}
	ep = clientEndpoint(store.Settings{PublicEndpoint: "10.0.0.1:9999"}, &store.Interface{ListenPort: 51820})
	if ep != "10.0.0.1:9999" {
		t.Fatalf("got %q", ep)
	}
}

func TestServerConfigSkipsDisabledPeers(t *testing.T) {
	iface := &store.Interface{
		Name:       "wg0",
		PrivateKey: "priv",
		Addresses:  []string{"10.42.0.1/24"},
		Peers: []*store.Peer{
			{Name: "on", Address: "10.42.0.2", PublicKey: "k1", Enabled: true, AllowedIPs: []string{"10.42.0.2/32"}},
			{Name: "off", Address: "10.42.0.9", PublicKey: "k2", Enabled: false, AllowedIPs: []string{"10.42.0.9/32"}},
		},
	}
	cfg := ServerConfig(iface)
	if strings.Contains(cfg, "# off") || strings.Contains(cfg, "k2") {
		t.Fatalf("disabled peer leaked into config:\n%s", cfg)
	}
	if !strings.Contains(cfg, "# on") {
		t.Fatalf("enabled peer missing:\n%s", cfg)
	}
}

func TestNextFreeAddress(t *testing.T) {
	iface := &store.Interface{
		Name:      "wg0",
		Addresses: []string{"10.42.0.1/24"},
		Peers: []*store.Peer{
			{Address: "10.42.0.1"},
			{Address: "10.42.0.2"},
			{Address: "10.42.0.5"},
			{Address: "10.42.0.7"},
		},
	}
	got, err := nextFreeAddress(iface)
	if err != nil {
		t.Fatalf("nextFreeAddress: %v", err)
	}
	if got != "10.42.0.3" {
		t.Fatalf("got %q, want 10.42.0.3", got)
	}
}

func TestNextFreeAddressFullSubnet(t *testing.T) {
	iface := &store.Interface{Addresses: []string{"192.168.9.1/30"}}
	iface.Peers = []*store.Peer{{Address: "192.168.9.2"}}
	got, err := nextFreeAddress(iface)
	if err != nil {
		t.Fatalf("nextFreeAddress: %v", err)
	}
	if got != "192.168.9.3" {
		t.Fatalf("got %q, want 192.168.9.3", got)
	}
}
