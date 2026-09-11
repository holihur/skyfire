package driver

import (
	"net/netip"
	"testing"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func mustAddr(t *testing.T, p string) netip.Addr {
	t.Helper()
	addr, err := netip.ParsePrefix(p)
	if err != nil {
		t.Fatal(err)
	}
	return addr.Addr()
}

func TestNetstackLifecycle(t *testing.T) {
	priv, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	peer, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}

	d := NewNetstack()
	name := "ns0"
	if err := d.Create(name, 1420); err != nil {
		t.Fatal(err)
	}
	if err := d.SetAddresses(name, []string{"10.42.0.1/24", "fd42:42:42::1/64"}); err != nil {
		t.Fatal(err)
	}
	if err := d.Configure(name, Config{
		PrivateKey: priv.String(),
		ListenPort: 0,
		Peers: []PeerSpec{{
			PublicKey:           peer.PublicKey().String(),
			AllowedIPs:          []string{"10.42.0.2/32"},
			PersistentKeepalive: 25,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := d.Up(name); err != nil {
		t.Fatalf("up: %v", err)
	}
	st, err := d.Status(name)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Running || st.PublicKey != priv.PublicKey().String() || len(st.Peers) != 1 {
		t.Fatalf("unexpected status: %+v", st)
	}
	if err := d.Down(name); err != nil {
		t.Fatalf("down: %v", err)
	}
	if err := d.Remove(name); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if err := d.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestNetstackAddressClassification(t *testing.T) {
	tun := &nsTun{localAddrs: []netip.Addr{mustAddr(t, "10.42.0.1/24")}}
	if !tun.isLocal(mustAddr(t, "10.42.0.1/24")) {
		t.Fatal("own address not detected")
	}
	if tun.isLocal(mustAddr(t, "10.42.0.9/32")) {
		t.Fatal("foreign address flagged local")
	}
}
