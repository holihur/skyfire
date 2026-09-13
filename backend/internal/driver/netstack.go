package driver

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv6"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
	"gvisor.dev/gvisor/pkg/waiter"

	"skyfire/internal/blacklist"
)

// Netstack is a fully userspace driver: the in-process wireguard-go device is
// wired directly into a gVisor network stack, so no OS tunnel interface, no
// kernel ip_forward and no NAT/MASQUERADE rules are needed. Transit traffic
// (peers reaching the internet through the server) is relayed at the transport
// layer: inbound TCP/UDP flows are terminated by netstack and re-originated
// through regular OS sockets, which yields NAT-like source rewriting across
// platforms (Linux, Windows, macOS) from a single code path.
//
// Limitations: ICMP and raw IP protocols are not relayed to transit
// destinations (ICMP to the interface's own address is still answered);
// transit traffic follows the daemon's own OS default route.
type Netstack struct {
	sync.Mutex
	logger *device.Logger
	devs   map[string]*nsDevice
}

type nsDevice struct {
	dev       *device.Device
	tun       *nsTun
	mtu       int
	addr      []string
	last      Config
	shaping   []nsShaping
	blacklist *blacklist.Matcher
}

// nsShaping is the per-peer rate limit applied in the userspace relay path.
type nsShaping struct {
	prefixes []netip.Prefix
	download int64
	upload   int64
}

// NewNetstack creates a netstack driver.
func NewNetstack() *Netstack {
	return &Netstack{
		logger: device.NewLogger(device.LogLevelError, "wireguard-go "),
		devs:   make(map[string]*nsDevice),
	}
}

func (n *Netstack) Name() string { return "netstack" }

func (n *Netstack) UsesOSStack() bool { return false }

func (n *Netstack) Open() error { return nil }

func (n *Netstack) Close() error {
	n.Lock()
	defer n.Unlock()
	for name, d := range n.devs {
		if d.dev != nil {
			d.dev.Close()
		}
		if d.tun != nil {
			_ = d.tun.Close()
		}
		delete(n.devs, name)
	}
	return nil
}

func (n *Netstack) get(name string) *nsDevice {
	n.Lock()
	defer n.Unlock()
	return n.devs[name]
}

func (n *Netstack) set(name string, d *nsDevice) {
	n.Lock()
	defer n.Unlock()
	n.devs[name] = d
}

func (n *Netstack) drop(name string) {
	n.Lock()
	defer n.Unlock()
	delete(n.devs, name)
}

// Create only registers the interface (name + mtu). The live device is built
// lazily at Up() once addresses are known to netstack.
func (n *Netstack) Create(name string, mtu int) error {
	n.Lock()
	defer n.Unlock()
	if d, ok := n.devs[name]; ok {
		if mtu > 0 {
			d.mtu = mtu
		}
		return nil
	}
	if mtu == 0 {
		mtu = 1420
	}
	n.devs[name] = &nsDevice{mtu: mtu}
	return nil
}

func (n *Netstack) Configure(name string, cfg Config) error {
	d := n.get(name)
	if d == nil {
		return fmt.Errorf("interface %s not created", name)
	}
	if d.dev != nil {
		text, err := renderUAPI(cfg)
		if err != nil {
			return err
		}
		if err := d.dev.IpcSet(text); err != nil {
			return fmt.Errorf("configure %s: %w", name, err)
		}
		if d.tun != nil {
			d.tun.forwarding.Store(cfg.Forwarding)
		}
	}
	d.last = cfg
	return nil
}

func (n *Netstack) SetAddresses(name string, addresses []string) error {
	d := n.get(name)
	if d == nil {
		return fmt.Errorf("interface %s not created", name)
	}
	d.addr = addresses
	return nil
}

func (n *Netstack) SetMTU(name string, mtu int) error {
	d := n.get(name)
	if d == nil {
		return fmt.Errorf("interface %s not created", name)
	}
	if mtu > 0 {
		d.mtu = mtu
	}
	return nil
}

// ensure builds the live netstack device (deferred until addresses are known).
func (n *Netstack) ensure(name string) (*nsDevice, error) {
	d := n.get(name)
	if d == nil {
		return nil, fmt.Errorf("interface %s not created", name)
	}
	if d.dev != nil {
		return d, nil
	}
	mtu := d.mtu
	if mtu == 0 {
		mtu = 1420
	}
	t, err := openNSStack(d.addr, mtu, d.last.Forwarding)
	if err != nil {
		return nil, fmt.Errorf("netstack %s: %w", name, err)
	}
	dev := device.NewDevice(t, conn.NewStdNetBind(), n.logger)
	if d.last.PrivateKey != "" {
		text, err := renderUAPI(d.last)
		if err != nil {
			dev.Close()
			_ = t.Close()
			return nil, err
		}
		if err := dev.IpcSet(text); err != nil {
			dev.Close()
			_ = t.Close()
			return nil, fmt.Errorf("configure %s: %w", name, err)
		}
	}
	d.tun = t
	d.dev = dev
	shape := d.shaping
	t.shaping.Store(&shape)
	return d, nil
}

func (n *Netstack) Up(name string) error {
	d, err := n.ensure(name)
	if err != nil {
		return err
	}
	if err := d.dev.Up(); err != nil {
		return err
	}
	d.tun.sendEvent(tun.EventUp)
	return nil
}

func (n *Netstack) Down(name string) error {
	d := n.get(name)
	if d == nil {
		return fmt.Errorf("interface %s not created", name)
	}
	if d.dev == nil {
		return nil
	}
	return d.dev.Down()
}

func (n *Netstack) Remove(name string) error {
	d := n.get(name)
	if d == nil {
		return nil
	}
	if d.dev != nil {
		d.dev.Close()
		d.dev = nil
	}
	if d.tun != nil {
		_ = d.tun.Close()
		d.tun = nil
	}
	n.drop(name)
	return nil
}

// ApplyShaping installs per-peer rate limits for the userspace relay path.
func (n *Netstack) ApplyShaping(name string, peers []PeerShaping) error {
	d := n.get(name)
	if d == nil {
		return fmt.Errorf("interface %s not created", name)
	}
	shape := buildNSShaping(peers)
	d.shaping = shape
	if d.tun != nil {
		d.tun.shaping.Store(&shape)
	}
	return nil
}

// RemoveShaping clears per-peer rate limits.
func (n *Netstack) RemoveShaping(name string) {
	d := n.get(name)
	if d == nil {
		return
	}
	d.shaping = nil
	if d.tun != nil {
		var none []nsShaping
		d.tun.shaping.Store(&none)
	}
}

// ApplyBlacklist installs the block list for the userspace relay path. An
// empty list clears it.
func (n *Netstack) ApplyBlacklist(name string, entries []string) error {
	d := n.get(name)
	if d == nil {
		return fmt.Errorf("interface %s not created", name)
	}
	m := blacklist.Compile(entries)
	d.blacklist = m
	if d.tun != nil {
		d.tun.blacklist.Store(m)
	}
	return nil
}

// buildNSShaping converts driver shaping specs into the userspace form,
// keeping only peers that actually carry a limit.
func buildNSShaping(peers []PeerShaping) []nsShaping {
	var out []nsShaping
	for _, p := range peers {
		if p.DownloadLimit <= 0 && p.UploadLimit <= 0 {
			continue
		}
		var prefixes []netip.Prefix
		for _, s := range p.Prefixes {
			if pre, err := netip.ParsePrefix(s); err == nil && pre.IsValid() {
				prefixes = append(prefixes, pre)
			}
		}
		if len(prefixes) == 0 {
			continue
		}
		out = append(out, nsShaping{
			prefixes: prefixes,
			download: p.DownloadLimit,
			upload:   p.UploadLimit,
		})
	}
	return out
}

// shapingFor returns the download/upload limits matching the given tunnel
// source address, using longest-prefix match. 0 means unlimited.
func (t *nsTun) shapingFor(src netip.Addr) (download, upload int64) {
	list := t.shaping.Load()
	if list == nil || !src.IsValid() {
		return 0, 0
	}
	best, bestBits := -1, -1
	for i := range *list {
		for _, pre := range (*list)[i].prefixes {
			if pre.Contains(src) && pre.Bits() > bestBits {
				best, bestBits = i, pre.Bits()
			}
		}
	}
	if best >= 0 {
		return (*list)[best].download, (*list)[best].upload
	}
	return 0, 0
}

func (n *Netstack) Status(name string) (DeviceStatus, error) {
	d := n.get(name)
	if d == nil {
		return DeviceStatus{Running: false}, nil
	}
	if d.dev == nil {
		return DeviceStatus{Running: false}, nil
	}
	st := DeviceStatus{Running: true, ListenPort: d.last.ListenPort}
	if priv, err := wgtypes.ParseKey(d.last.PrivateKey); err == nil {
		st.PublicKey = priv.PublicKey().String()
	}
	for _, spec := range d.last.Peers {
		ps := PeerStatus{
			PublicKey:           spec.PublicKey,
			Endpoint:            spec.Endpoint,
			AllowedIPs:          spec.AllowedIPs,
			PersistentKeepalive: spec.PersistentKeepalive,
		}
		ps.LatestHandshake, ps.TransferRx, ps.TransferTx, ps.Connected = liveStats(d.dev, spec.PublicKey, spec.PersistentKeepalive)
		st.Peers = append(st.Peers, ps)
	}
	return st, nil
}

// nsTun is the gVisor-backed tun.Device handed to wireguard-go, in the spirit
// of wireguard-go's tun/netstack package, extended with TCP/UDP forwarders so
// transit traffic through the tunnel reaches the OS network.
type nsTun struct {
	ep             *channel.Endpoint
	stack          *stack.Stack
	events         chan tun.Event
	notifyHandle   *channel.NotificationHandle
	incomingPacket chan *buffer.View
	localAddrs     []netip.Addr
	peerNets       []netip.Prefix
	forwarding     atomic.Bool
	shaping        atomic.Pointer[[]nsShaping]
	blacklist      atomic.Pointer[blacklist.Matcher]
	closeOnce      sync.Once
	closed         atomic.Bool
}

func toFullAddr(ap netip.AddrPort) (tcpip.FullAddress, tcpip.NetworkProtocolNumber) {
	pn := ipv6.ProtocolNumber
	if ap.Addr().Is4() {
		pn = ipv4.ProtocolNumber
	}
	fa := tcpip.FullAddress{
		NIC:  1,
		Addr: tcpip.AddrFromSlice(ap.Addr().AsSlice()),
		Port: ap.Port(),
	}
	return fa, pn
}

// peerNetsOf turns a peer config into the cryptokey-routing prefixes this
// interface forwards back into the tunnel (peer-to-peer transit), as-is
// including default routes, mirroring kernel WireGuard semantics.
func peerNetsOf(peers []PeerSpec) []netip.Prefix {
	var out []netip.Prefix
	for _, p := range peers {
		for _, a := range p.AllowedIPs {
			pre, err := netip.ParsePrefix(a)
			if err != nil {
				continue
			}
			if !pre.IsValid() {
				continue
			}
			out = append(out, pre)
		}
	}
	return out
}

func openNSStack(prefixes []string, mtu int, forwarding bool) (*nsTun, error) {
	addrs := make([]netip.Addr, 0, len(prefixes))
	hasV4, hasV6 := false, false
	for _, p := range prefixes {
		pre, err := netip.ParsePrefix(p)
		if err != nil {
			return nil, fmt.Errorf("parse address %q: %w", p, err)
		}
		ip := pre.Addr()
		if !ip.IsValid() {
			return nil, fmt.Errorf("invalid address %q", p)
		}
		addrs = append(addrs, ip)
		if ip.Is4() {
			hasV4 = true
		} else {
			hasV6 = true
		}
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("no local addresses configured")
	}

	st := stack.New(stack.Options{
		NetworkProtocols:   []stack.NetworkProtocolFactory{ipv4.NewProtocol, ipv6.NewProtocol},
		TransportProtocols: []stack.TransportProtocolFactory{tcp.NewProtocol, udp.NewProtocol},
		// HandleLocal must be off: combined with promiscuous mode (needed so
		// transit packets addressed to arbitrary destinations are accepted)
		// the loopback shortcut otherwise stops answering the interface's
		// own address.
		HandleLocal: false,
	})
	sackEnabled := tcpip.TCPSACKEnabled(true)
	if err := st.SetTransportProtocolOption(tcp.ProtocolNumber, &sackEnabled); err != nil {
		st.Close()
		return nil, fmt.Errorf("enable TCP SACK: %s", err)
	}

	t := &nsTun{
		ep:             channel.New(1024, uint32(mtu), ""),
		stack:          st,
		events:         make(chan tun.Event, 10),
		incomingPacket: make(chan *buffer.View, 512),
		localAddrs:     addrs,
	}
	t.forwarding.Store(forwarding)
	t.notifyHandle = t.ep.AddNotify(t)
	if err := st.CreateNIC(1, t.ep); err != nil {
		st.Close()
		return nil, fmt.Errorf("create nic: %s", err)
	}
	for _, ip := range addrs {
		pn := ipv6.ProtocolNumber
		if ip.Is4() {
			pn = ipv4.ProtocolNumber
		}
		protoAddr := tcpip.ProtocolAddress{
			Protocol:          pn,
			AddressWithPrefix: tcpip.AddrFromSlice(ip.AsSlice()).WithPrefix(),
		}
		if err := st.AddProtocolAddress(1, protoAddr, stack.AddressProperties{}); err != nil {
			st.RemoveNIC(1)
			st.Close()
			return nil, fmt.Errorf("address %s: %s", ip, err)
		}
	}
	if hasV4 {
		st.AddRoute(tcpip.Route{Destination: header.IPv4EmptySubnet, NIC: 1})
	}
	if hasV6 {
		st.AddRoute(tcpip.Route{Destination: header.IPv6EmptySubnet, NIC: 1})
	}

	// Forwarders only run when registered as the transport protocol handler;
	// creating them without registering silently drops TCP/UDP and resets
	// connections destined to the interface address.
	tcpFwd := tcp.NewForwarder(st, 65535, 65535, t.handleTCP)
	st.SetTransportProtocolHandler(tcp.ProtocolNumber, tcpFwd.HandlePacket)
	udpFwd := udp.NewForwarder(st, t.handleUDP)
	st.SetTransportProtocolHandler(udp.ProtocolNumber, udpFwd.HandlePacket)

	// Accept and relay packets addressed to arbitrary destinations (internet
	// transit through the tunnel), not only the interface's own addresses.
	if err := st.SetPromiscuousMode(1, true); err != nil {
		st.Close()
		return nil, fmt.Errorf("enable promiscuous mode: %s", err)
	}
	if err := st.SetSpoofing(1, true); err != nil {
		st.Close()
		return nil, fmt.Errorf("enable spoofing: %s", err)
	}
	return t, nil
}

func (t *nsTun) sendEvent(e tun.Event) {
	// Sending on a closed channel panics (select/default does not save
	// us), so guard with the closed flag plus a recover for the residual
	// Up()-vs-Close() race window.
	defer func() { _ = recover() }()
	if t.closed.Load() {
		return
	}
	select {
	case t.events <- e:
	default:
	}
}

func (t *nsTun) Name() (string, error) { return "go", nil }

func (t *nsTun) File() *os.File { return nil }

func (t *nsTun) Events() <-chan tun.Event { return t.events }

func (t *nsTun) Read(bufs [][]byte, sizes []int, offset int) (int, error) {
	view, ok := <-t.incomingPacket
	if !ok {
		return 0, os.ErrClosed
	}
	n, err := view.Read(bufs[0][offset:])
	if err != nil {
		return 0, err
	}
	sizes[0] = n
	return 1, nil
}

func (t *nsTun) Write(bufs [][]byte, offset int) (int, error) {
	for _, b := range bufs {
		packet := b[offset:]
		if len(packet) == 0 {
			continue
		}
		pkt := stack.NewPacketBuffer(stack.PacketBufferOptions{Payload: buffer.MakeWithData(packet)})
		switch packet[0] >> 4 {
		case 4:
			t.ep.InjectInbound(header.IPv4ProtocolNumber, pkt)
		case 6:
			t.ep.InjectInbound(header.IPv6ProtocolNumber, pkt)
		default:
			return 0, fmt.Errorf("unsupported IP version in packet")
		}
	}
	return len(bufs), nil
}

func (t *nsTun) WriteNotify() {
	// Drain all queued packets: channel.Endpoint may batch several
	// packets behind a single notification.
	for {
		pkt := t.ep.Read()
		if pkt == nil {
			return
		}
		view := pkt.ToView()
		pkt.DecRef()
		select {
		case t.incomingPacket <- view:
		default:
			view.Release()
		}
	}
}

func (t *nsTun) Close() error {
	t.closeOnce.Do(func() {
		t.closed.Store(true)
		t.stack.RemoveNIC(1)
		t.stack.Close()
		t.ep.RemoveNotify(t.notifyHandle)
		t.ep.Close()
		close(t.events)
		close(t.incomingPacket)
	})
	return nil
}

func (t *nsTun) MTU() (int, error) { return int(t.ep.MTU()), nil }

func (t *nsTun) BatchSize() int { return 1 }

// isLocal reports whether dst is one of the interface's own addresses. Such
// flows target the server itself (e.g. the web API over the tunnel), so they
// are relayed to the local loopback instead of being dialed out.
func (t *nsTun) isLocal(dst netip.Addr) bool {
	for _, a := range t.localAddrs {
		if a == dst {
			return true
		}
	}
	return false
}

// flowDst converts a transport endpoint id into the OS-side dial address: the
// netstack "local" half of the 4-tuple is the destination internet host.
func tcpipToNetip(a tcpip.Address) (netip.Addr, bool) {
	switch a.Len() {
	case 4:
		return netip.AddrFrom4(a.As4()), true
	case 16:
		return netip.AddrFrom16(a.As16()), true
	}
	return netip.Addr{}, false
}

func idDst(id stack.TransportEndpointID) (netip.AddrPort, bool) {
	ip, ok := tcpipToNetip(id.LocalAddress)
	if !ok {
		return netip.AddrPort{}, false
	}
	return netip.AddrPortFrom(ip, id.LocalPort), true
}

// dialHost resolves the OS-side host for a flow, keeping the hairpin case
// (traffic to the server's own tunnel address) on loopback.
func (t *nsTun) dialHost(dst netip.Addr) string {
	if t.isLocal(dst) {
		if dst.Is6() {
			return "::1"
		}
		return "127.0.0.1"
	}
	return dst.String()
}

// handleTCP relays inbound TCP connections from the tunnel to the network
// through regular OS sockets (no kernel NAT involved).
func (t *nsTun) handleTCP(fr *tcp.ForwarderRequest) {
	go func() {
		dst, ok := idDst(fr.ID())
		if !ok {
			fr.Complete(true)
			return
		}
		// When forwarding is disabled only traffic to the server's own
		// addresses (e.g. the tunnel DNS at the interface address) is
		// allowed; internet transit is rejected.
		if !t.forwarding.Load() && !t.isLocal(dst.Addr()) {
			fr.Complete(true)
			return
		}
		if m := t.blacklist.Load(); m.MatchIP(dst.Addr()) {
			fr.Complete(true)
			return
		}
		src, _ := tcpipToNetip(fr.ID().RemoteAddress)
		dl, ul := t.shapingFor(src)
		raddr := &net.TCPAddr{IP: net.ParseIP(t.dialHost(dst.Addr())), Port: int(dst.Port())}
		remote, err := net.DialTCP("tcp", nil, raddr)
		if err != nil {
			fr.Complete(true)
			return
		}
		var wq waiter.Queue
		ep, terr := fr.CreateEndpoint(&wq)
		if terr != nil {
			fr.Complete(true)
			_ = remote.Close()
			return
		}
		fr.Complete(false)
		client := gonet.NewTCPConn(&wq, ep)
		relayConns(client, remote, ul, dl)
	}()
}

func udpDial(t *nsTun, dst netip.Addr, port uint16) *net.UDPAddr {
	return &net.UDPAddr{IP: net.ParseIP(t.dialHost(dst)), Port: int(port)}
}

// handleUDP relays inbound UDP datagrams (e.g. DNS, QUIC) from the tunnel to
// the OS network via connected UDP sockets; datagram boundaries are preserved.
func (t *nsTun) handleUDP(r *udp.ForwarderRequest) {
	go func() {
		dst, ok := idDst(r.ID())
		if !ok {
			return
		}
		// Drop transit datagrams when forwarding is disabled; local
		// destinations (tunnel DNS, server API) stay reachable.
		if !t.forwarding.Load() && !t.isLocal(dst.Addr()) {
			return
		}
		if m := t.blacklist.Load(); m.MatchIP(dst.Addr()) {
			return
		}
		sock, err := net.DialUDP("udp", nil, udpDial(t, dst.Addr(), dst.Port()))
		if err != nil {
			return
		}
		src, _ := tcpipToNetip(r.ID().RemoteAddress)
		dl, ul := t.shapingFor(src)
		var wq waiter.Queue
		ep, terr := r.CreateEndpoint(&wq)
		if terr != nil {
			_ = sock.Close()
			return
		}
		client := gonet.NewUDPConn(&wq, ep)
		relayUDP(client, sock, ul, dl)
	}()
}

// closeWrite half-closes the write side where supported so the peer can
// still deliver in-flight data after we stop sending.
func closeWrite(c net.Conn) {
	if tc, ok := c.(*net.TCPConn); ok {
		_ = tc.CloseWrite()
	}
}

func relayConns(client net.Conn, remote net.Conn, uploadLimit, downloadLimit int64) {
	// Wait for both directions before closing: closing either conn early
	// would truncate the opposite direction and break TCP half-close
	// (e.g. HTTP request + FIN followed by the response).
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(remote, limitReader(client, uploadLimit))
		closeWrite(remote)
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(client, limitReader(remote, downloadLimit))
		closeWrite(client)
	}()
	wg.Wait()
	_ = client.Close()
	_ = remote.Close()
}

// udpIdleTimeout bounds relay goroutine lifetime for connectionless UDP
// flows whose peer never answers.
const udpIdleTimeout = 2 * time.Minute

func relayUDP(client *gonet.UDPConn, remote *net.UDPConn, uploadLimit, downloadLimit int64) {
	upLim := newByteLimiter(uploadLimit)
	downLim := newByteLimiter(downloadLimit)
	go func() {
		buf := make([]byte, 65535)
		for {
			_ = client.SetReadDeadline(time.Now().Add(udpIdleTimeout))
			n, err := client.Read(buf)
			if err != nil {
				break
			}
			if upLim != nil {
				if werr := upLim.WaitN(context.Background(), n); werr != nil {
					break
				}
			}
			if _, err := remote.Write(buf[:n]); err != nil {
				break
			}
		}
		_ = client.Close()
		_ = remote.Close()
	}()
	go func() {
		buf := make([]byte, 65535)
		for {
			_ = remote.SetReadDeadline(time.Now().Add(udpIdleTimeout))
			n, err := remote.Read(buf)
			if err != nil {
				break
			}
			if downLim != nil {
				if werr := downLim.WaitN(context.Background(), n); werr != nil {
					break
				}
			}
			if _, err := client.Write(buf[:n]); err != nil {
				break
			}
		}
		_ = client.Close()
		_ = remote.Close()
	}()
}
