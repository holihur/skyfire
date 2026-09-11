package driver

import (
	"fmt"
	"io"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

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

func TestNetstackConcurrentCreateRemove(t *testing.T) {
	d := NewNetstack()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("ns%d", i)
			_ = d.Create(name, 1420)
			_ = d.Remove(name)
		}(i)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = d.Close()
	}()
	wg.Wait()
}

func TestRelayConnsFullDuplex(t *testing.T) {
	c1, c2 := net.Pipe()
	r1, r2 := net.Pipe()
	done := make(chan struct{})
	go func() { relayConns(c1, r1); close(done) }()
	if _, err := c2.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, 4)
	if _, err := io.ReadFull(r2, got); err != nil {
		t.Fatalf("client->remote: %v", err)
	}
	if _, err := r2.Write([]byte("pong")); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(c2, got); err != nil {
		t.Fatalf("remote->client: %v", err)
	}
	_ = c2.Close()
	_ = r2.Close()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("relayConns did not return after both sides closed")
	}
}
