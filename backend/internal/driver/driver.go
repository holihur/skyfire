// Package driver abstracts the WireGuard tunnel backend so the manager can
// run against either the in-process userspace implementation (wireguard-go),
// the kernel module (via wgctrl), or a safe in-memory mock used for
// development and testing.
package driver

import "time"

// Config is the desired wireguard device configuration.
type Config struct {
	PrivateKey string `json:"privateKey"`
	ListenPort int    `json:"listenPort"`
	// FirewallMark, when > 0, tags the device's own packets (endpoint UDP
	// traffic) so the full-tunnel policy routing rules can exclude them and
	// avoid a routing loop. Required alongside AddDefaultRoutes.
	FirewallMark int        `json:"firewallMark,omitempty"`
	Peers        []PeerSpec `json:"peers"`
}

// PeerSpec is a single peer as applied to the device.
type PeerSpec struct {
	PublicKey           string   `json:"publicKey"`
	PresharedKey        string   `json:"presharedKey"`
	Endpoint            string   `json:"endpoint"`
	AllowedIPs          []string `json:"allowedIPs"`
	PersistentKeepalive int      `json:"persistentKeepalive"`
}

// PeerStatus is live runtime information about a peer.
type PeerStatus struct {
	PublicKey           string    `json:"publicKey"`
	Endpoint            string    `json:"endpoint"`
	AllowedIPs          []string  `json:"allowedIPs"`
	LatestHandshake     time.Time `json:"latestHandshake"`
	Connected           bool      `json:"connected"`
	TransferRx          uint64    `json:"transferRx"`
	TransferTx          uint64    `json:"transferTx"`
	PersistentKeepalive int       `json:"persistentKeepalive"`
}

// DeviceStatus is live runtime information about an interface.
type DeviceStatus struct {
	Running    bool         `json:"running"`
	PublicKey  string       `json:"publicKey"`
	ListenPort int          `json:"listenPort"`
	Peers      []PeerStatus `json:"peers"`
}

// TransferAggRx sums received bytes over all peers.
func (s DeviceStatus) TransferAggRx() uint64 {
	var n uint64
	for _, p := range s.Peers {
		n += p.TransferRx
	}
	return n
}

// TransferAggTx sums transmitted bytes over all peers.
func (s DeviceStatus) TransferAggTx() uint64 {
	var n uint64
	for _, p := range s.Peers {
		n += p.TransferTx
	}
	return n
}

// Peer returns live status for a peer by public key, or nil.
func (s DeviceStatus) Peer(pub string) *PeerStatus {
	for i := range s.Peers {
		if s.Peers[i].PublicKey == pub {
			return &s.Peers[i]
		}
	}
	return nil
}

// RouteTargets filters prefixes skyfired never manages with plain
// per-prefix routes: the default routes ("0.0.0.0/0", "::/0") and empty
// entries. Default routes need policy routing instead — see AddDefaultRoutes.
func RouteTargets(allowed []string) []string {
	var out []string
	for _, a := range allowed {
		if a == "" || a == "0.0.0.0/0" || a == "::/0" {
			continue
		}
		out = append(out, a)
	}
	return out
}

// Driver manages the full lifecycle of a wireguard interface.
type Driver interface {
	Name() string
	// UsesOSStack reports whether packet forwarding relies on the OS network
	// stack (routes on a kernel interface, ip_forward, NAT). When false the
	// driver handles forwarding itself (e.g. the netstack backend) and the
	// manager must not install any OS plumbing.
	UsesOSStack() bool
	// Create creates the wireguard device and its link. Idempotent.
	Create(name string, mtu int) error
	// Configure applies the full device + peer configuration
	// (replace-all semantics).
	Configure(name string, cfg Config) error
	// SetAddresses adds the given CIDR addresses to the interface.
	SetAddresses(name string, addresses []string) error
	// SetMTU updates the interface MTU.
	SetMTU(name string, mtu int) error
	// Up brings the interface up, Down takes it down.
	Up(name string) error
	Down(name string) error
	// Remove tears down and deletes the device.
	Remove(name string) error
	// Status returns live runtime state for the device.
	Status(name string) (DeviceStatus, error)
	Close() error
}
