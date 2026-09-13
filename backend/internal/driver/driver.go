// Package driver abstracts the WireGuard tunnel backend so the manager can
// run against either the in-process userspace implementation (wireguard-go),
// the kernel module (via wgctrl), or a safe in-memory mock used for
// development and testing.
package driver

import (
	"errors"
	"time"
)

// Config is the desired wireguard device configuration.
type Config struct {
	PrivateKey string `json:"privateKey"`
	ListenPort int    `json:"listenPort"`
	// FirewallMark, when > 0, tags the device's own packets (endpoint UDP
	// traffic) so the full-tunnel policy routing rules can exclude them and
	// avoid a routing loop. Required alongside AddDefaultRoutes.
	FirewallMark int `json:"firewallMark,omitempty"`
	// Forwarding allows transit traffic (clients reaching external networks
	// through this device). When false the driver drops non-local transit.
	Forwarding bool       `json:"forwarding"`
	Peers      []PeerSpec `json:"peers"`
}

// PeerSpec is a single peer as applied to the device.
type PeerSpec struct {
	PublicKey           string   `json:"publicKey"`
	PresharedKey        string   `json:"presharedKey"`
	Endpoint            string   `json:"endpoint"`
	AllowedIPs          []string `json:"allowedIPs"`
	PersistentKeepalive int      `json:"persistentKeepalive"`
}

// PeerShaping describes per-peer rate limits for one peer.
type PeerShaping struct {
	// Prefixes are the source/destination prefixes used to match this peer's
	// traffic. They are the peer's server-side AllowedIPs (or its assigned
	// tunnel address as a /32 or /128 when AllowedIPs is empty).
	Prefixes []string
	// DownloadLimit caps the peer's download (server → peer) in bits per
	// second. 0 means unlimited.
	DownloadLimit int64
	// UploadLimit caps the peer's upload (peer → server) in bits per second.
	// 0 means unlimited.
	UploadLimit int64
}

// ErrShapingUnsupported is returned by ApplyShaping when the driver cannot
// enforce rate limits. Callers should treat it as non-fatal and log a hint.
var ErrShapingUnsupported = errors.New("rate limiting is not supported by this driver")

// ErrBlacklistUnsupported is returned by ApplyBlacklist when the driver cannot
// enforce the block list. Callers should treat it as non-fatal and log a hint.
var ErrBlacklistUnsupported = errors.New("blacklist is not supported by this driver")

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
	// ApplyShaping installs per-peer rate limits (download/upload in bits
	// per second, 0 = unlimited). Drivers that cannot shape return
	// ErrShapingUnsupported; the configuration is still kept.
	ApplyShaping(name string, peers []PeerShaping) error
	// RemoveShaping removes any per-peer rate limiting for the interface.
	RemoveShaping(name string)
	// ApplyBlacklist installs the operator block list (domains, IP addresses
	// and CIDR prefixes). Matched traffic is dropped: domains are dropped at
	// the DNS proxy, addresses at the data path. An empty list clears any
	// installed rules. Drivers that cannot enforce it return
	// ErrBlacklistUnsupported; the configuration is still kept.
	ApplyBlacklist(name string, entries []string) error
	Close() error
}
