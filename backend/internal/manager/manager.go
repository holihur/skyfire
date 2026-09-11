// Package manager owns the desired-state configuration and applies it to a
// WireGuard Driver. All mutations are persisted before being applied so a
// driver failure never loses user configuration.
package manager

import (
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"sort"
	"strings"
	"sync"
	"time"

	"skyfire/internal/driver"
	"skyfire/internal/store"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// Manager persists and applies WireGuard configuration.
type Manager struct {
	path   string
	mu     sync.Mutex
	store  *store.Config
	driver driver.Driver
	dryRun bool
	log    *slog.Logger
}

// ErrNotFound reports a missing interface/peer.
var ErrNotFound = errors.New("not found")

// ErrConflict reports a configuration conflict.
var ErrConflict = errors.New("conflict")

// New loads the stored configuration and, unless in dry-run mode, reconciles
// the live interfaces against it.
func New(cfgPath string, drv driver.Driver, dryRun bool, log *slog.Logger) (*Manager, error) {
	cfg, err := store.Load(cfgPath)
	if err != nil {
		return nil, err
	}
	m := &Manager{path: cfgPath, store: cfg, driver: drv, dryRun: dryRun, log: log}
	if !dryRun {
		if err := m.Reconcile(); err != nil {
			m.log.Error("initial reconciliation failed", "error", err)
		}
	}
	return m, nil
}

// Reconcile brings live interfaces in line with the desired state.
func (m *Manager) Reconcile() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, iface := range m.store.Interfaces {
		if iface.Up {
			if err := m.applyInterface(iface); err != nil {
				m.log.Error("reconcile apply failed", "iface", iface.Name, "error", err)
			}
		} else {
			if err := m.driver.Remove(iface.Name); err != nil {
				m.log.Warn("reconcile remove failed", "iface", iface.Name, "error", err)
			}
		}
	}
	return nil
}

func (m *Manager) persist() error {
	return store.Save(m.path, m.store)
}

// applyInterface applies the desired state of one interface to the driver.
func (m *Manager) applyInterface(iface *store.Interface) error {
	if m.dryRun {
		m.log.Info("dry-run: skipping live application", "iface", iface.Name)
		return nil
	}
	if err := m.driver.Create(iface.Name, iface.MTU); err != nil {
		return fmt.Errorf("create %s: %w", iface.Name, err)
	}
	if err := m.driver.Configure(iface.Name, toDriverConfig(iface)); err != nil {
		return fmt.Errorf("configure %s: %w", iface.Name, err)
	}
	if iface.MTU > 0 {
		if err := m.driver.SetMTU(iface.Name, iface.MTU); err != nil {
			m.log.Warn("set mtu", "iface", iface.Name, "error", err)
		}
	}
	if len(iface.Addresses) > 0 {
		if err := m.driver.SetAddresses(iface.Name, iface.Addresses); err != nil {
			return fmt.Errorf("assign addresses on %s: %w", iface.Name, err)
		}
	}
	if iface.Up {
		return m.driver.Up(iface.Name)
	}
	return m.driver.Down(iface.Name)
}

func toDriverConfig(iface *store.Interface) driver.Config {
	cfg := driver.Config{
		PrivateKey: iface.PrivateKey,
		ListenPort: iface.ListenPort,
	}
	for _, p := range iface.Peers {
		if !p.Enabled {
			continue
		}
		cfg.Peers = append(cfg.Peers, driver.PeerSpec{
			PublicKey:           p.PublicKey,
			PresharedKey:        p.PresharedKey,
			Endpoint:            p.Endpoint,
			AllowedIPs:          p.AllowedIPs,
			PersistentKeepalive: p.PersistentKeepalive,
		})
	}
	return cfg
}

// ---------------------------------------------------------------------------
// Interface operations
// ---------------------------------------------------------------------------

// CreateInterface validates and creates a new interface.
func (m *Manager) CreateInterface(in *store.Interface) (*InterfaceView, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := strings.TrimSpace(in.Name)
	if err := validateName(name); err != nil {
		return nil, err
	}
	for _, existing := range m.store.Interfaces {
		if existing.Name == name {
			return nil, fmt.Errorf("%w: interface %q already exists", ErrConflict, name)
		}
		if in.ListenPort > 0 && existing.ListenPort == in.ListenPort {
			return nil, fmt.Errorf("%w: listen port %d already in use by %q", ErrConflict, in.ListenPort, existing.Name)
		}
	}
	if in.ListenPort > 65535 {
		return nil, fmt.Errorf("listen port out of range")
	}
	if err := validateAddresses(in.Addresses); err != nil {
		return nil, err
	}

	priv := in.PrivateKey
	if priv == "" {
		var err error
		priv, in.PublicKey, err = genKeys()
		if err != nil {
			return nil, err
		}
	} else if in.PublicKey == "" {
		pk, err := wgtypes.ParseKey(priv)
		if err != nil {
			return nil, fmt.Errorf("invalid private key: %w", err)
		}
		in.PublicKey = pk.PublicKey().String()
	}
	mtu := in.MTU
	if mtu == 0 {
		mtu = store.DefaultMTU
	}

	iface := &store.Interface{
		Name:       name,
		PrivateKey: priv,
		PublicKey:  in.PublicKey,
		ListenPort: in.ListenPort,
		Addresses:  in.Addresses,
		MTU:        mtu,
		DNS:        in.DNS,
		Up:         in.Up,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	m.store.Interfaces = append(m.store.Interfaces, iface)
	if err := m.persist(); err != nil {
		return nil, err
	}
	if err := m.applyInterface(iface); err != nil {
		return nil, fmt.Errorf("%w (configuration was saved)", err)
	}
	return m.interfaceViewLocked(iface)
}

// UpdateInterface updates mutable settings on an interface.
func (m *Manager) UpdateInterface(name string, patch *InterfacePatch) (*InterfaceView, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	iface, err := m.findInterfaceLocked(name)
	if err != nil {
		return nil, err
	}
	if patch.ListenPort != nil {
		if *patch.ListenPort < 0 || *patch.ListenPort > 65535 {
			return nil, fmt.Errorf("listen port out of range")
		}
		for _, existing := range m.store.Interfaces {
			if existing.Name != name && existing.ListenPort == *patch.ListenPort {
				return nil, fmt.Errorf("%w: listen port %d already in use by %q", ErrConflict, *patch.ListenPort, existing.Name)
			}
		}
	}
	if patch.Addresses != nil {
		if err := validateAddresses(patch.Addresses); err != nil {
			return nil, err
		}
	}
	if patch.MTU != nil {
		if *patch.MTU < 576 || *patch.MTU > 65535 {
			return nil, fmt.Errorf("mtu out of range")
		}
	}
	if patch.ListenPort != nil {
		iface.ListenPort = *patch.ListenPort
	}
	if patch.Addresses != nil {
		iface.Addresses = patch.Addresses
	}
	if patch.MTU != nil {
		iface.MTU = *patch.MTU
	}
	if patch.DNS != nil {
		iface.DNS = patch.DNS
	}
	if patch.Up != nil {
		iface.Up = *patch.Up
	}
	iface.UpdatedAt = time.Now()
	if err := m.persist(); err != nil {
		return nil, err
	}
	if err := m.applyInterface(iface); err != nil {
		return nil, fmt.Errorf("%w (configuration was saved)", err)
	}
	return m.interfaceViewLocked(iface)
}

// DeleteInterface removes an interface and its live device.
func (m *Manager) DeleteInterface(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	iface, err := m.findInterfaceLocked(name)
	if err != nil {
		return err
	}
	if !m.dryRun {
		if err := m.driver.Remove(iface.Name); err != nil {
			m.log.Warn("driver remove failed", "iface", iface.Name, "error", err)
		}
	}
	for i, existing := range m.store.Interfaces {
		if existing.Name == name {
			m.store.Interfaces = append(m.store.Interfaces[:i], m.store.Interfaces[i+1:]...)
			break
		}
	}
	return m.persist()
}

// SetInterfaceUp enables or disables an interface.
func (m *Manager) SetInterfaceUp(name string, up bool) (*InterfaceView, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	iface, err := m.findInterfaceLocked(name)
	if err != nil {
		return nil, err
	}
	iface.Up = up
	iface.UpdatedAt = time.Now()
	if err := m.persist(); err != nil {
		return nil, err
	}
	if err := m.applyInterface(iface); err != nil {
		iface.Up = !up
		return nil, fmt.Errorf("%w (configuration was saved, state reverted in memory)", err)
	}
	return m.interfaceViewLocked(iface)
}

// ---------------------------------------------------------------------------
// Peer operations
// ---------------------------------------------------------------------------

// CreatePeer adds a peer to an interface.
func (m *Manager) CreatePeer(ifaceName string, in *PeerInput) (*PeerView, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	iface, err := m.findInterfaceLocked(ifaceName)
	if err != nil {
		return nil, err
	}

	peer := &store.Peer{
		Name:                strings.TrimSpace(in.Name),
		Address:             strings.TrimSpace(in.Address),
		Endpoint:            strings.TrimSpace(in.Endpoint),
		PresharedKey:        in.PresharedKey,
		PersistentKeepalive: in.PersistentKeepalive,
		Description:         in.Description,
		ClientRoutes:        in.ClientRoutes,
		DNS:                 in.DNS,
		Enabled:             true,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
	if peer.Name == "" {
		peer.Name = "peer"
	}
	if in.GenerateKeys || in.PublicKey == "" {
		priv, pub, err := genKeys()
		if err != nil {
			return nil, err
		}
		peer.PrivateKey = priv
		peer.PublicKey = pub
	} else {
		pk, err := wgtypes.ParseKey(in.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("invalid public key: %w", err)
		}
		peer.PublicKey = pk.String()
	}
	if in.PresharedKey == "" && in.WithPreshared {
		psk, err := genPreshared()
		if err != nil {
			return nil, err
		}
		peer.PresharedKey = psk
	}
	if len(peer.ClientRoutes) == 0 {
		peer.ClientRoutes = store.DefaultRoutes()
	}

	if peer.Address == "" {
		addr, err := nextFreeAddress(iface)
		if err != nil {
			return nil, fmt.Errorf("cannot auto-assign address: %w", err)
		}
		peer.Address = addr
	}
	if len(peer.AllowedIPs) == 0 {
		peer.AllowedIPs = []string{clientAddressCIDR(peer.Address)}
	}
	for _, existing := range iface.Peers {
		if existing.PublicKey == peer.PublicKey {
			return nil, fmt.Errorf("%w: peer with public key %s already exists", ErrConflict, peer.PublicKey)
		}
	}
	iface.Peers = append(iface.Peers, peer)
	iface.UpdatedAt = time.Now()
	if err := m.persist(); err != nil {
		return nil, err
	}
	if err := m.applyInterface(iface); err != nil {
		return nil, fmt.Errorf("%w (configuration was saved)", err)
	}
	return m.peerViewLocked(iface, peer)
}

// UpdatePeer edits a peer. Setting GenerateKeys rotates the keypair.
func (m *Manager) UpdatePeer(ifaceName, publicKey string, in *PeerInput) (*PeerView, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	iface, err := m.findInterfaceLocked(ifaceName)
	if err != nil {
		return nil, err
	}
	target, err := findPeerLocked(iface, publicKey)
	if err != nil {
		return nil, err
	}
	if in.GenerateKeys {
		priv, pub, err := genKeys()
		if err != nil {
			return nil, err
		}
		target.PrivateKey = priv
		target.PublicKey = pub
	} else if in.PublicKey != "" && in.PublicKey != publicKey {
		pk, err := wgtypes.ParseKey(in.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("invalid public key: %w", err)
		}
		for _, existing := range iface.Peers {
			if existing != target && existing.PublicKey == pk.String() {
				return nil, fmt.Errorf("%w: duplicate public key", ErrConflict)
			}
		}
		target.PublicKey = pk.String()
	}
	target.Name = strings.TrimSpace(in.Name)
	if target.Name == "" {
		target.Name = "peer"
	}
	target.Address = strings.TrimSpace(in.Address)
	target.Endpoint = strings.TrimSpace(in.Endpoint)
	target.PresharedKey = ""
	if in.WithPreshared {
		if in.PresharedKey != "" {
			target.PresharedKey = in.PresharedKey
		} else {
			psk, err := genPreshared()
			if err != nil {
				return nil, err
			}
			target.PresharedKey = psk
		}
	}
	target.PersistentKeepalive = in.PersistentKeepalive
	target.AllowedIPs = in.AllowedIPs
	target.ClientRoutes = in.ClientRoutes
	target.DNS = in.DNS
	target.Description = in.Description
	target.Enabled = in.Enabled
	if len(target.AllowedIPs) == 0 && target.Address != "" {
		target.AllowedIPs = []string{clientAddressCIDR(target.Address)}
	}
	target.UpdatedAt = time.Now()
	iface.UpdatedAt = time.Now()
	if err := m.persist(); err != nil {
		return nil, err
	}
	if err := m.applyInterface(iface); err != nil {
		return nil, fmt.Errorf("%w (configuration was saved)", err)
	}
	return m.peerViewLocked(iface, target)
}

// DeletePeer removes a peer.
func (m *Manager) DeletePeer(ifaceName, publicKey string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	iface, err := m.findInterfaceLocked(ifaceName)
	if err != nil {
		return err
	}
	found := false
	for i, p := range iface.Peers {
		if p.PublicKey == publicKey {
			iface.Peers = append(iface.Peers[:i], iface.Peers[i+1:]...)
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("%w: peer %s", ErrNotFound, publicKey)
	}
	iface.UpdatedAt = time.Now()
	if err := m.persist(); err != nil {
		return err
	}
	if err := m.applyInterface(iface); err != nil {
		return fmt.Errorf("%w (configuration was saved)", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Lookups and views
// ---------------------------------------------------------------------------

func (m *Manager) findInterfaceLocked(name string) (*store.Interface, error) {
	for _, iface := range m.store.Interfaces {
		if iface.Name == name {
			return iface, nil
		}
	}
	return nil, fmt.Errorf("%w: interface %s", ErrNotFound, name)
}

func findPeerLocked(iface *store.Interface, publicKey string) (*store.Peer, error) {
	for _, p := range iface.Peers {
		if p.PublicKey == publicKey {
			return p, nil
		}
	}
	return nil, fmt.Errorf("%w: peer %s", ErrNotFound, publicKey)
}

func (m *Manager) driverStatus(name string) driver.DeviceStatus {
	// the mock driver is purely informational and safe even in dry-run mode,
	// so demo mode still shows simulated live status
	if m.dryRun && m.driver.Name() != "mock" {
		return driver.DeviceStatus{Running: false}
	}
	st, err := m.driver.Status(name)
	if err != nil {
		m.log.Debug("status error", "iface", name, "error", err)
		return driver.DeviceStatus{}
	}
	return st
}

// List returns a view of every interface.
func (m *Manager) List() []*InterfaceView {
	m.mu.Lock()
	defer m.mu.Unlock()
	views := make([]*InterfaceView, 0, len(m.store.Interfaces))
	for _, iface := range m.store.Interfaces {
		if v, err := m.interfaceViewLocked(iface); err == nil {
			views = append(views, v)
		}
	}
	sort.Slice(views, func(i, j int) bool { return views[i].Name < views[j].Name })
	return views
}

// Get returns the view of a single interface.
func (m *Manager) Get(name string) (*InterfaceView, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	iface, err := m.findInterfaceLocked(name)
	if err != nil {
		return nil, err
	}
	return m.interfaceViewLocked(iface)
}

func (m *Manager) interfaceViewLocked(iface *store.Interface) (*InterfaceView, error) {
	st := m.driverStatus(iface.Name)
	v := &InterfaceView{
		Name:       iface.Name,
		PublicKey:  iface.PublicKey,
		ListenPort: iface.ListenPort,
		Addresses:  nonNil(iface.Addresses),
		MTU:        iface.MTU,
		DNS:        nonNil(iface.DNS),
		Up:         iface.Up,
		Running:    st.Running,
		DryRun:     m.dryRun,
		CreatedAt:  iface.CreatedAt,
		UpdatedAt:  iface.UpdatedAt,
		TotalPeers: len(iface.Peers),
		TransferRx: st.TransferAggRx(),
		TransferTx: st.TransferAggTx(),
		Peers:      make([]*PeerView, 0, len(iface.Peers)),
	}
	for _, p := range iface.Peers {
		live := st.Peer(p.PublicKey)
		pv := &PeerView{
			Name:                p.Name,
			PublicKey:           p.PublicKey,
			PresharedKey:        p.PresharedKey,
			Address:             p.Address,
			AllowedIPs:          nonNil(p.AllowedIPs),
			ClientRoutes:        nonNil(p.ClientRoutes),
			DNS:                 nonNil(p.DNS),
			Endpoint:            p.Endpoint,
			PersistentKeepalive: p.PersistentKeepalive,
			Description:         p.Description,
			Enabled:             p.Enabled,
			CreatedAt:           p.CreatedAt,
			UpdatedAt:           p.UpdatedAt,
		}
		if live != nil {
			pv.Endpoint = firstNonEmpty(live.Endpoint, pv.Endpoint)
			pv.TransferRx = live.TransferRx
			pv.TransferTx = live.TransferTx
			pv.LatestHandshake = live.LatestHandshake
			pv.Connected = live.Connected
		}
		if pv.Connected {
			v.ConnectedPeers++
		}
		v.Peers = append(v.Peers, pv)
	}
	return v, nil
}

func (m *Manager) peerViewLocked(iface *store.Interface, p *store.Peer) (*PeerView, error) {
	v, err := m.interfaceViewLocked(iface)
	if err != nil {
		return nil, err
	}
	for _, pv := range v.Peers {
		if pv.PublicKey == p.PublicKey {
			return pv, nil
		}
	}
	return nil, fmt.Errorf("%w: peer %s", ErrNotFound, p.PublicKey)
}

// PrivateKey returns an interface's private key (solely for display).
func (m *Manager) PrivateKey(name string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	iface, err := m.findInterfaceLocked(name)
	if err != nil {
		return "", err
	}
	return iface.PrivateKey, nil
}

// Settings returns the daemon settings.
func (m *Manager) Settings() store.Settings {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.store.Settings
}

// UpdateSettings saves daemon settings.
func (m *Manager) UpdateSettings(s store.Settings) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store.Settings = s
	return m.persist()
}

// ClientConfig produces a client config file for a peer.
func (m *Manager) ClientConfig(ifaceName, publicKey string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	iface, err := m.findInterfaceLocked(ifaceName)
	if err != nil {
		return "", err
	}
	p, err := findPeerLocked(iface, publicKey)
	if err != nil {
		return "", err
	}
	return ClientConfig(m.store.Settings, iface, p), nil
}

// ServerConfig produces a wg-quick export for an interface.
func (m *Manager) ServerConfig(ifaceName string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	iface, err := m.findInterfaceLocked(ifaceName)
	if err != nil {
		return "", err
	}
	return ServerConfig(iface), nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func validateName(name string) error {
	if name == "" {
		return fmt.Errorf("interface name is required")
	}
	if len(name) > 15 {
		return fmt.Errorf("interface name too long (max 15 chars)")
	}
	for _, r := range name {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return fmt.Errorf("interface name may only contain lowercase letters, digits, '-' and '_'")
		}
	}
	return nil
}

func validateAddresses(addrs []string) error {
	for _, a := range addrs {
		if _, err := netip.ParsePrefix(a); err != nil {
			return fmt.Errorf("invalid address %q: %w", a, err)
		}
	}
	return nil
}

// nextFreeAddress picks the next unused host in the interface's first subnet.
func nextFreeAddress(iface *store.Interface) (string, error) {
	var used []string
	for _, p := range iface.Peers {
		if p.Address != "" {
			used = append(used, hostOf(p.Address))
		}
	}
	for _, a := range iface.Addresses {
		used = append(used, hostOf(a))
	}
	for _, raw := range iface.Addresses {
		p, err := netip.ParsePrefix(raw)
		if err != nil || !p.IsValid() {
			continue
		}
		if p.Addr().Is4() && p.Bits() >= 31 {
			continue
		}
		if p.Addr().Is6() && p.Bits() >= 127 {
			continue
		}
		a := p.Addr().Next()
		for p.Contains(a) {
			h := a.String()
			if !containsStr(used, h) {
				return h, nil
			}
			a = a.Next()
			if !a.IsValid() {
				break
			}
		}
	}
	return "", fmt.Errorf("no free address in %v", iface.Addresses)
}

func hostOf(cidr string) string {
	if i := strings.IndexByte(cidr, '/'); i >= 0 {
		return cidr[:i]
	}
	return cidr
}

func clientAddressCIDR(addr string) string {
	if strings.Contains(addr, "/") {
		return addr
	}
	if strings.Contains(addr, ":") {
		return addr + "/128"
	}
	return addr + "/32"
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
