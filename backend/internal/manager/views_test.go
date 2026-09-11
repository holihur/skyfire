package manager

import (
	"encoding/json"
	"errors"
	"log/slog"
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

func TestDuplicatePeerAddress(t *testing.T) {
	conf := filepath.Join(t.TempDir(), "skyfire.json")
	m, err := New(conf, driver.NewMock(), true, nil)
	if err != nil {
		t.Fatal(err)
	}
	iface := &store.Interface{
		Name:       "wg0",
		ListenPort: 51820,
		Addresses:  []string{"10.42.0.1/24"},
		MTU:        1420,
		Up:         true,
	}
	if _, err := m.CreateInterface(iface); err != nil {
		t.Fatal(err)
	}

	boolPtr := func(v bool) *bool { return &v }

	peerA := &PeerInput{
		Name:         "a",
		Address:      "10.42.0.5",
		GenerateKeys: true,
		WithPreshared: boolPtr(false),
		Enabled:      true,
	}
	if _, err := m.CreatePeer("wg0", peerA); err != nil {
		t.Fatalf("create peer A: %v", err)
	}

	peerB := &PeerInput{
		Name:         "b",
		Address:      "10.42.0.5",
		GenerateKeys: true,
		WithPreshared: boolPtr(false),
		Enabled:      true,
	}
	if _, err := m.CreatePeer("wg0", peerB); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate peer address: want ErrConflict, got %v", err)
	}

	peerIface := &PeerInput{
		Name:         "c",
		Address:      "10.42.0.1",
		GenerateKeys: true,
		WithPreshared: boolPtr(false),
		Enabled:      true,
	}
	if _, err := m.CreatePeer("wg0", peerIface); !errors.Is(err, ErrConflict) {
		t.Fatalf("conflict with interface address: want ErrConflict, got %v", err)
	}

	peerAuto := &PeerInput{
		Name:         "d",
		GenerateKeys: true,
		WithPreshared: boolPtr(false),
		Enabled:      true,
	}
	view, err := m.CreatePeer("wg0", peerAuto)
	if err != nil {
		t.Fatalf("create peer with empty address: %v", err)
	}
	autoAddr := view.Address
	if autoAddr == "" {
		t.Fatal("auto-assigned address should not be empty")
	}

	got, _ := m.Get("wg0")
	pubA := got.Peers[0].PublicKey
	updateSame := &PeerInput{
		Name:    "a",
		Address: "10.42.0.5",
		Enabled: true,
	}
	if _, err := m.UpdatePeer("wg0", pubA, updateSame); err != nil {
		t.Fatalf("update to same address should succeed: %v", err)
	}

	updateConflict := &PeerInput{
		Name:    "a",
		Address: autoAddr,
		Enabled: true,
	}
	if _, err := m.UpdatePeer("wg0", pubA, updateConflict); !errors.Is(err, ErrConflict) {
		t.Fatalf("update to duplicate address: want ErrConflict, got %v", err)
	}

	updateIface := &PeerInput{
		Name:    "a",
		Address: "10.42.0.1",
		Enabled: true,
	}
	if _, err := m.UpdatePeer("wg0", pubA, updateIface); !errors.Is(err, ErrConflict) {
		t.Fatalf("update to interface address: want ErrConflict, got %v", err)
	}

	updateEmpty := &PeerInput{
		Name:    "a",
		Address: "",
		Enabled: true,
	}
	if _, err := m.UpdatePeer("wg0", pubA, updateEmpty); err != nil {
		t.Fatalf("update to empty address should succeed: %v", err)
	}
}
