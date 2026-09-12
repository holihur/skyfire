package tunnel

import (
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

// dnsProxy is a minimal split-DNS proxy. It answers A/AAAA queries whose name
// matches shouldResolve from resolve() (which also installs tunnel routes),
// and relays every other query verbatim to upstream. It listens on UDP and TCP
// so both short and truncated queries work.
type dnsProxy struct {
	upstream      string
	shouldResolve func(name string) bool
	resolve       func(name string) ([]netip.Addr, error)
	log           *slog.Logger

	udp *net.UDPConn
	tcp net.Listener

	stop chan struct{}
	wg   sync.WaitGroup
	once sync.Once
}

func newDNSProxy(upstream string, shouldResolve func(string) bool, resolve func(string) ([]netip.Addr, error), log *slog.Logger) *dnsProxy {
	return &dnsProxy{
		upstream:      upstream,
		shouldResolve: shouldResolve,
		resolve:       resolve,
		log:           log,
		stop:          make(chan struct{}),
	}
}

// start binds the proxy on addr (e.g. 127.0.0.1:53) and launches its loops.
func (p *dnsProxy) start(addr string) error {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return err
	}
	p.udp, err = net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("listen udp %s: %w", addr, err)
	}
	p.tcp, err = net.Listen("tcp", addr)
	if err != nil {
		_ = p.udp.Close()
		p.udp = nil
		return fmt.Errorf("listen tcp %s: %w", addr, err)
	}

	p.wg.Add(2)
	go p.udpLoop()
	go p.tcpLoop()
	return nil
}

// close stops the proxy and waits for its goroutines to exit.
func (p *dnsProxy) close() {
	p.once.Do(func() {
		close(p.stop)
		if p.udp != nil {
			_ = p.udp.Close()
		}
		if p.tcp != nil {
			_ = p.tcp.Close()
		}
	})
	p.wg.Wait()
}

func (p *dnsProxy) udpLoop() {
	defer p.wg.Done()
	buf := make([]byte, 4096)
	for {
		n, addr, err := p.udp.ReadFromUDP(buf)
		if err != nil {
			return
		}
		pkt := make([]byte, n)
		copy(pkt, buf[:n])
		go p.handleUDP(addr, pkt)
	}
}

func (p *dnsProxy) handleUDP(addr *net.UDPAddr, pkt []byte) {
	resp := p.answer(pkt)
	if resp == nil {
		resp = p.relayUDP(pkt)
	}
	if resp != nil {
		_, _ = p.udp.WriteToUDP(resp, addr)
	}
}

func (p *dnsProxy) relayUDP(pkt []byte) []byte {
	conn, err := net.Dial("udp", p.upstream)
	if err != nil {
		return nil
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write(pkt); err != nil {
		return nil
	}
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return nil
	}
	return buf[:n]
}

func (p *dnsProxy) tcpLoop() {
	defer p.wg.Done()
	for {
		conn, err := p.tcp.Accept()
		if err != nil {
			return
		}
		go p.handleTCP(conn)
	}
}

func (p *dnsProxy) handleTCP(conn net.Conn) {
	defer conn.Close()
	for {
		_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
		var lenBuf [2]byte
		if _, err := io.ReadFull(conn, lenBuf[:]); err != nil {
			return
		}
		length := binary.BigEndian.Uint16(lenBuf[:])
		if length == 0 || length > 4096 {
			return
		}
		pkt := make([]byte, length)
		if _, err := io.ReadFull(conn, pkt); err != nil {
			return
		}
		resp := p.answer(pkt)
		if resp == nil {
			resp = p.relayTCP(pkt)
		}
		if resp == nil {
			return
		}
		if err := binary.Write(conn, binary.BigEndian, uint16(len(resp))); err != nil {
			return
		}
		if _, err := conn.Write(resp); err != nil {
			return
		}
	}
}

func (p *dnsProxy) relayTCP(pkt []byte) []byte {
	conn, err := net.Dial("tcp", p.upstream)
	if err != nil {
		return nil
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	if err := binary.Write(conn, binary.BigEndian, uint16(len(pkt))); err != nil {
		return nil
	}
	if _, err := conn.Write(pkt); err != nil {
		return nil
	}
	var lenBuf [2]byte
	if _, err := io.ReadFull(conn, lenBuf[:]); err != nil {
		return nil
	}
	length := binary.BigEndian.Uint16(lenBuf[:])
	if length == 0 || length > 4096 {
		return nil
	}
	resp := make([]byte, length)
	if _, err := io.ReadFull(conn, resp); err != nil {
		return nil
	}
	return resp
}

// answer builds a local response for a matched A/AAAA query, or nil when the
// query should be relayed upstream (unmatched name, unsupported type, or a
// name that failed to resolve).
func (p *dnsProxy) answer(pkt []byte) []byte {
	var msg dnsmessage.Message
	if err := msg.Unpack(pkt); err != nil {
		return nil
	}
	if len(msg.Questions) != 1 {
		return nil
	}
	q := msg.Questions[0]
	if q.Type != dnsmessage.TypeA && q.Type != dnsmessage.TypeAAAA {
		return nil
	}
	name := strings.TrimSuffix(strings.ToLower(q.Name.String()), ".")
	if !p.shouldResolve(name) {
		return nil
	}
	addrs, err := p.resolve(name)
	if err != nil || len(addrs) == 0 {
		return nil
	}

	resp := dnsmessage.Message{
		Header: dnsmessage.Header{
			ID:                 msg.ID,
			Response:           true,
			RecursionDesired:   msg.Header.RecursionDesired,
			RecursionAvailable: true,
		},
		Questions: msg.Questions,
	}
	for _, ip := range addrs {
		if q.Type == dnsmessage.TypeA && ip.Is4() {
			a := ip.As4()
			resp.Answers = append(resp.Answers, dnsmessage.Resource{
				Header: dnsmessage.ResourceHeader{Name: q.Name, Type: dnsmessage.TypeA, Class: dnsmessage.ClassINET, TTL: 60},
				Body:   &dnsmessage.AResource{A: a},
			})
		}
		if q.Type == dnsmessage.TypeAAAA && ip.Is6() {
			aaaa := ip.As16()
			resp.Answers = append(resp.Answers, dnsmessage.Resource{
				Header: dnsmessage.ResourceHeader{Name: q.Name, Type: dnsmessage.TypeAAAA, Class: dnsmessage.ClassINET, TTL: 60},
				Body:   &dnsmessage.AAAAResource{AAAA: aaaa},
			})
		}
	}
	if len(resp.Answers) == 0 {
		return nil
	}
	out, err := resp.Pack()
	if err != nil {
		return nil
	}
	return out
}
