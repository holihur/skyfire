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
	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
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

// TestNetstackUDPRelay verifies the transport forwarders are actually wired
// into the stack: without SetTransportProtocolHandler the stack resets
// connection attempts instead of relaying them to host sockets.
func TestNetstackUDPRelay(t *testing.T) {
	// A UDP echo service on host loopback stands in for a local service the
	// tunnel should reach.
	pc, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	defer pc.Close()
	dstPort := uint16(pc.LocalAddr().(*net.UDPAddr).Port)

	tun, err := openNSStack([]string{"10.99.0.1/24"}, 1420)
	if err != nil {
		t.Fatalf("open ns stack: %v", err)
	}
	defer tun.stack.Close()

	// Craft an IPv4/UDP datagram from a peer tunnel address to the service.
	payload := []byte("ping")
	src := netip.MustParseAddr("10.99.0.2").As4()
	dst := netip.MustParseAddr("10.99.0.1").As4()
	total := header.IPv4MinimumSize + header.UDPMinimumSize + len(payload)
	buf := make([]byte, total)
	ip := header.IPv4(buf)
	ip.Encode(&header.IPv4Fields{
		TotalLength: uint16(total),
		TTL:         64,
		Protocol:    uint8(header.UDPProtocolNumber),
		SrcAddr:     tcpip.AddrFrom4(src),
		DstAddr:     tcpip.AddrFrom4(dst),
	})
	ip.SetChecksum(^ip.CalculateChecksum())
	udp := header.UDP(ip.Payload())
	udp.Encode(&header.UDPFields{
		SrcPort: 40000,
		DstPort: dstPort,
		Length:  uint16(header.UDPMinimumSize + len(payload)),
	})
	copy(udp.Payload(), payload)

	tun.ep.InjectInbound(header.IPv4ProtocolNumber, stack.NewPacketBuffer(stack.PacketBufferOptions{
		Payload: buffer.MakeWithData(buf),
	}))

	// The relayed datagram must arrive at the loopback service.
	rbuf := make([]byte, 256)
	_ = pc.SetReadDeadline(time.Now().Add(3 * time.Second))
	n, addr, err := pc.ReadFromUDP(rbuf)
	if err != nil {
		t.Fatalf("relay did not reach loopback service: %v", err)
	}
	if string(rbuf[:n]) != "ping" {
		t.Fatalf("relayed payload = %q, want %q", rbuf[:n], "ping")
	}
	if _, err := pc.WriteToUDP([]byte("pong"), addr); err != nil {
		t.Fatalf("write reply: %v", err)
	}

	// The reply must come back through the tunnel device.
	done := make(chan []byte, 1)
	go func() {
		bufs := [][]byte{make([]byte, 2048)}
		sizes := make([]int, 1)
		if _, err := tun.Read(bufs, sizes, 0); err != nil {
			return
		}
		off := header.IPv4MinimumSize + header.UDPMinimumSize
		if sizes[0] < off {
			return
		}
		done <- append([]byte(nil), bufs[0][off:sizes[0]]...)
	}()
	select {
	case reply := <-done:
		if string(reply) != "pong" {
			t.Fatalf("reply payload = %q, want %q", reply, "pong")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no reply through the tunnel device")
	}
}
