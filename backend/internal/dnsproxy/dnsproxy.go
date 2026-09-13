// Package dnsproxy is a minimal transparent DNS forwarder.
//
// It relays raw DNS queries (UDP and TCP) to a list of upstream resolvers
// without parsing them, so every record type — including EDNS0, DNSSEC and
// large TCP responses — passes through untouched. It exists so the Skyfire
// server can offer clean DNS to its tunnel clients: clients point their DNS
// at the server's tunnel address, the netstack driver relays that traffic to
// loopback, and this forwarder resolves it through the configured upstreams.
package dnsproxy

import (
	"encoding/binary"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"skyfire/internal/blacklist"
)

// DefaultTimeout bounds a single upstream exchange.
const DefaultTimeout = 5 * time.Second

// Server forwards DNS traffic to clean upstream resolvers.
type Server struct {
	Upstreams []string      // host:port, e.g. "8.8.8.8:53"
	Timeout   time.Duration // per-exchange deadline (0 -> DefaultTimeout)
	Log       *slog.Logger
	// Blacklist, when non-nil, makes the proxy silently drop queries for
	// blocked domains (matched traffic is discarded, not answered).
	Blacklist *blacklist.Store

	next      atomic.Uint32
	udpConns  []*net.UDPConn
	tcpLn     net.Listener
	wg        sync.WaitGroup
	closeOnce sync.Once
	done      chan struct{}
}

// New builds a server. Upstreams must be non-empty; entries without a port
// get ":53" appended.
func New(upstreams []string, log *slog.Logger) (*Server, error) {
	if len(upstreams) == 0 {
		return nil, errors.New("dnsproxy: at least one upstream is required")
	}
	if log == nil {
		log = slog.Default()
	}
	s := &Server{Log: log, done: make(chan struct{})}
	for _, u := range upstreams {
		host, port, err := net.SplitHostPort(u)
		if err != nil {
			host, port = u, "53"
		}
		if port == "" {
			port = "53"
		}
		s.Upstreams = append(s.Upstreams, net.JoinHostPort(host, port))
	}
	if s.Timeout <= 0 {
		s.Timeout = DefaultTimeout
	}
	return s, nil
}

// Start binds addr (e.g. ":53") for UDP and TCP and begins serving in the
// background. It returns an error only when neither protocol can bind.
func (s *Server) Start(addr string) error {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return err
	}
	udp, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return err
	}
	s.udpConns = append(s.udpConns, udp)

	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		_ = udp.Close()
		return err
	}
	tcp, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		_ = udp.Close()
		return err
	}
	s.tcpLn = tcp

	s.Log.Info("dns proxy listening", "addr", addr, "upstreams", s.Upstreams)
	s.wg.Add(2)
	go func() {
		defer s.wg.Done()
		s.serveUDP(udp)
	}()
	go func() {
		defer s.wg.Done()
		s.serveTCP(tcp)
	}()
	return nil
}

// Addr returns the UDP listen address, which is useful when the server was
// started with port 0. It returns nil before Start.
func (s *Server) Addr() net.Addr {
	if len(s.udpConns) == 0 {
		return nil
	}
	return s.udpConns[0].LocalAddr()
}

// Close stops the listeners and waits for in-flight handlers to finish.
func (s *Server) Close() {
	s.closeOnce.Do(func() {
		close(s.done)
		for _, c := range s.udpConns {
			_ = c.Close()
		}
		if s.tcpLn != nil {
			_ = s.tcpLn.Close()
		}
	})
	s.wg.Wait()
}

// upstream round-robins over the configured resolvers.
func (s *Server) upstream() string {
	n := s.next.Add(1)
	return s.Upstreams[(n-1)%uint32(len(s.Upstreams))]
}

// blocked reports whether the query is for a blacklisted domain. Malformed
// query names are never blocked.
func (s *Server) blocked(q []byte) bool {
	m := s.Blacklist.Load()
	if m.Empty() {
		return false
	}
	name, ok := questionName(q)
	if !ok {
		return false
	}
	return m.MatchDomain(name)
}

// questionName extracts the QNAME from a query. Queries carry an uncompressed
// name in the question section, so no full DNS parser is needed.
func questionName(q []byte) (string, bool) {
	if len(q) < 12 {
		return "", false
	}
	var b []byte
	for i := 12; i < len(q); {
		l := int(q[i])
		i++
		switch {
		case l == 0:
			return string(b), true
		case l&0xc0 != 0: // compression is invalid in a question
			return "", false
		case i+l > len(q):
			return "", false
		}
		if len(b) > 0 {
			b = append(b, '.')
		}
		b = append(b, q[i:i+l]...)
		i += l
	}
	return "", false
}

func (s *Server) serveUDP(conn *net.UDPConn) {
	buf := make([]byte, 65535)
	for {
		n, client, err := conn.ReadFromUDP(buf)
		if err != nil {
			return
		}
		// Copy before handing off: the shared buffer is reused next read.
		q := make([]byte, n)
		copy(q, buf[:n])
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.relayUDP(conn, client, q)
		}()
	}
}

func (s *Server) relayUDP(conn *net.UDPConn, client *net.UDPAddr, q []byte) {
	if s.blocked(q) {
		s.Log.Debug("dns query dropped (blacklist)", "client", client)
		return
	}
	resp, err := s.exchangeUDP(q)
	if err != nil {
		s.Log.Debug("dns udp exchange failed", "error", err, "client", client)
		return
	}
	if _, err := conn.WriteToUDP(resp, client); err != nil {
		s.Log.Debug("dns udp reply failed", "error", err, "client", client)
	}
}

// exchangeUDP tries each upstream in turn until one answers. A fresh
// connected socket per exchange keeps responses matched to their query.
func (s *Server) exchangeUDP(q []byte) ([]byte, error) {
	start := int(s.next.Load()) % len(s.Upstreams)
	var lastErr error
	for i := 0; i < len(s.Upstreams); i++ {
		u := s.Upstreams[(start+i)%len(s.Upstreams)]
		resp, err := s.exchangeUDPOne(u, q)
		if err == nil {
			return resp, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func (s *Server) exchangeUDPOne(upstream string, q []byte) ([]byte, error) {
	ua, err := net.ResolveUDPAddr("udp", upstream)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialUDP("udp", nil, ua)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(s.Timeout)); err != nil {
		return nil, err
	}
	if _, err := conn.Write(q); err != nil {
		return nil, err
	}
	resp := make([]byte, 65535)
	n, err := conn.Read(resp)
	if err != nil {
		return nil, err
	}
	out := make([]byte, n)
	copy(out, resp[:n])
	return out, nil
}

func (s *Server) serveTCP(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer conn.Close()
			s.relayTCP(conn)
		}()
	}
}

// relayTCP handles DNS over TCP (RFC 1035 §4.2.2: two-byte length prefix).
func (s *Server) relayTCP(client net.Conn) {
	for {
		var length uint16
		if err := binary.Read(client, binary.BigEndian, &length); err != nil {
			return
		}
		q := make([]byte, length)
		if _, err := io.ReadFull(client, q); err != nil {
			return
		}
		if s.blocked(q) {
			s.Log.Debug("dns query dropped (blacklist)", "client", client.RemoteAddr())
			return
		}
		// A single upstream connection serves one query here; resolvers
		// rarely pipeline over TCP.
		up, err := net.Dial("tcp", s.upstream())
		if err != nil {
			s.Log.Debug("dns tcp upstream dial failed", "error", err)
			return
		}
		_ = up.SetDeadline(time.Now().Add(s.Timeout))
		// Write the length prefix and query as one segment: some upstreams
		// (and middleboxes) reset the connection when the 2-byte length
		// arrives in a separate TCP segment from the query.
		msg := make([]byte, 2+len(q))
		binary.BigEndian.PutUint16(msg[:2], length)
		copy(msg[2:], q)
		if _, err := up.Write(msg); err != nil {
			s.Log.Debug("dns tcp upstream write failed", "error", err)
			_ = up.Close()
			return
		}
		var rlength uint16
		if err := binary.Read(up, binary.BigEndian, &rlength); err != nil {
			s.Log.Debug("dns tcp upstream read len failed", "error", err)
			_ = up.Close()
			return
		}
		resp := make([]byte, rlength)
		if _, err := io.ReadFull(up, resp); err != nil {
			s.Log.Debug("dns tcp upstream read body failed", "error", err)
			_ = up.Close()
			return
		}
		_ = up.Close()
		out := make([]byte, 2+len(resp))
		binary.BigEndian.PutUint16(out[:2], rlength)
		copy(out[2:], resp)
		if _, err := client.Write(out); err != nil {
			return
		}
	}
}
