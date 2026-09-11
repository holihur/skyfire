package manager

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skyfire/internal/driver"
	"skyfire/internal/store"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func TestNonNilSlices(t *testing.T) {
	if nonNil(nil) == nil {
		t.Fatal("nonNil(nil) should return an empty slice")
	}
	if s := nonNil([]string{"a"}); len(s) != 1 {
		t.Fatal("nonNil should preserve non-nil slices")
	}
}

func TestInterfaceViewMarshalEmptyPeersAsArray(t *testing.T) {
	conf := filepath.Join(t.TempDir(), "skyfire.json")
	if err := os.WriteFile(conf, []byte(`{
  "version": 1,
  "interfaces": [
    {
      "name": "wg0",
      "publicKey": "k",
      "listenPort": 51820,
      "mtu": 1420
    }
  ]
}`), 0o600); err != nil {
		t.Fatal(err)
	}
	m, err := New(conf, driver.NewMock(), true, nil)
	if err != nil {
		t.Fatal(err)
	}
	v, err := m.Get("wg0")
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{`"peers":[]`, `"addresses":[]`, `"dns":[]`} {
		if !strings.Contains(s, want) {
			t.Fatalf("expected %s in:\n%s", want, s)
		}
	}
	if strings.Contains(s, `:null`) {
		t.Fatalf("JSON must not contain null fields:\n%s", s)
	}
}
func TestValidatePresharedKey(t *testing.T) {
	valid, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	if err := validatePresharedKey(""); err != nil {
		t.Fatalf("empty psk must be allowed: %v", err)
	}
	if err := validatePresharedKey(valid.String()); err != nil {
		t.Fatalf("valid psk rejected: %v", err)
	}
	for _, bad := range []string{"123456", "not-a-key", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"} {
		if err := validatePresharedKey(bad); err == nil {
			t.Fatalf("invalid psk %q accepted", bad)
		}
	}
}

func TestDefaultRouteFamiliesAndFwMark(t *testing.T) {
	if v4, v6 := defaultRouteFamilies([]string{"10.0.0.0/8"}); v4 || v6 {
		t.Fatal("no default route expected")
	}
	if v4, v6 := defaultRouteFamilies([]string{"0.0.0.0/0"}); !v4 || v6 {
		t.Fatalf("want v4 only, got v4=%v v6=%v", v4, v6)
	}
	if v4, v6 := defaultRouteFamilies([]string{"::/0", "0.0.0.0/0"}); !v4 || !v6 {
		t.Fatalf("want both, got v4=%v v6=%v", v4, v6)
	}
	iface := &store.Interface{
		Name: "wg0",
		Peers: []*store.Peer{
			{PublicKey: "k", Enabled: true, AllowedIPs: []string{"0.0.0.0/0"}},
		},
	}
	cfg := toDriverConfig(iface)
	if cfg.FirewallMark != routeFwMark {
		t.Fatalf("full tunnel must set fwmark %d, got %d", routeFwMark, cfg.FirewallMark)
	}
	iface.Peers[0].AllowedIPs = []string{"10.42.0.2/32"}
	if cfg = toDriverConfig(iface); cfg.FirewallMark != 0 {
		t.Fatalf("subnet-only must not set fwmark, got %d", cfg.FirewallMark)
	}
	iface.Peers[0].Enabled = false
	if cfg = toDriverConfig(iface); cfg.FirewallMark != 0 {
		t.Fatalf("disabled peer must not trigger fwmark, got %d", cfg.FirewallMark)
	}
}
