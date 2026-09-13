package dnsproxy

import (
	"bytes"
	"net"
	"testing"
	"time"

	"skyfire/internal/blacklist"
)

// TestServerDropsBlacklisted runs the proxy against a fake upstream: allowed
// queries are relayed, blacklisted ones are dropped without a reply.
func TestServerDropsBlacklisted(t *testing.T) {
	up, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("fake upstream: %v", err)
	}
	defer up.Close()
	go func() {
		b := make([]byte, 2048)
		for {
			n, a, err := up.ReadFromUDP(b)
			if err != nil {
				return
			}
			_, _ = up.WriteToUDP(b[:n], a) // echo
		}
	}()

	srv, err := New([]string{up.LocalAddr().String()}, nil)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	srv.Blacklist = blacklist.NewStore([]string{"ads.example.com"})
	if err := srv.Start("127.0.0.1:0"); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer srv.Close()

	conn, err := net.Dial("udp", srv.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	buf := make([]byte, 2048)

	allowed := query("good.example.com")
	if _, err := conn.Write(allowed); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("allowed query should be relayed, got: %v", err)
	}
	if !bytes.Equal(buf[:n], allowed) {
		t.Fatalf("allowed reply mismatch: % x", buf[:n])
	}

	blocked := query("ads.example.com")
	if _, err := conn.Write(blocked); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(400 * time.Millisecond))
	if _, err := conn.Read(buf); err == nil {
		t.Fatal("blacklisted query must be dropped, but a reply arrived")
	}
}
