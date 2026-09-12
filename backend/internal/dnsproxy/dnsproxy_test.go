package dnsproxy

import (
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"
)

func TestRelayUDP(t *testing.T) {
	const marker = "dns-proxy-udp-response"

	up, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatal(err)
	}
	defer up.Close()
	go func() {
		buf := make([]byte, 512)
		for {
			_, addr, err := up.ReadFromUDP(buf)
			if err != nil {
				return
			}
			_, _ = up.WriteToUDP([]byte(marker), addr)
		}
	}()

	s, err := New([]string{up.LocalAddr().String()}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Start("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	proxyAddr := s.udpConns[0].LocalAddr().(*net.UDPAddr)
	client, err := net.DialUDP("udp", nil, proxyAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if err := client.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Write([]byte("query")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 512)
	n, err := client.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(buf[:n]); got != marker {
		t.Fatalf("got %q, want %q", got, marker)
	}
}

func TestRelayTCP(t *testing.T) {
	const marker = "dns-proxy-tcp-response"

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		var l uint16
		if err := binary.Read(conn, binary.BigEndian, &l); err != nil {
			return
		}
		q := make([]byte, l)
		if _, err := io.ReadFull(conn, q); err != nil {
			return
		}
		_ = binary.Write(conn, binary.BigEndian, uint16(len(marker)))
		_, _ = conn.Write([]byte(marker))
	}()

	s, err := New([]string{ln.Addr().String()}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Start("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	client, err := net.Dial("tcp", s.tcpLn.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if err := client.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := binary.Write(client, binary.BigEndian, uint16(5)); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Write([]byte("query")); err != nil {
		t.Fatal(err)
	}
	var rl uint16
	if err := binary.Read(client, binary.BigEndian, &rl); err != nil {
		t.Fatal(err)
	}
	resp := make([]byte, rl)
	if _, err := io.ReadFull(client, resp); err != nil {
		t.Fatal(err)
	}
	if got := string(resp); got != marker {
		t.Fatalf("got %q, want %q", got, marker)
	}
}
