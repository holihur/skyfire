//go:build linux

package driver

import (
	"fmt"
	"net"
	"net/netip"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// Kernel is the wgctrl backend. It drives the in-kernel WireGuard module via
// netlink and needs CAP_NET_ADMIN (root). Interface links are created with the
// ip(8) utility.
type Kernel struct {
	client *wgctrl.Client
}

// NewKernel opens the netlink control client.
func NewKernel() (*Kernel, error) {
	c, err := wgctrl.New()
	if err != nil {
		return nil, fmt.Errorf("open wgctrl: %w", err)
	}
	return &Kernel{client: c}, nil
}

func (k *Kernel) Name() string { return "kernel" }

func (k *Kernel) Close() error { return k.client.Close() }

func (k *Kernel) Create(name string, mtu int) error {
	mtuStr := ""
	if mtu > 0 {
		mtuStr = fmt.Sprint(mtu)
	}
	return addWGLink(name, mtuStr)
}

func (k *Kernel) Configure(name string, cfg Config) error {
	kc := wgtypes.Config{ReplacePeers: true}
	if cfg.PrivateKey != "" {
		if key, err := wgtypes.ParseKey(cfg.PrivateKey); err == nil {
			kc.PrivateKey = &key
		} else {
			return fmt.Errorf("invalid private key: %w", err)
		}
	}
	if cfg.ListenPort > 0 {
		kc.ListenPort = &cfg.ListenPort
	}
	for _, p := range cfg.Peers {
		pk, err := wgtypes.ParseKey(p.PublicKey)
		if err != nil {
			return fmt.Errorf("invalid peer public key: %w", err)
		}
		pc := wgtypes.PeerConfig{
			PublicKey:         pk,
			ReplaceAllowedIPs: true,
		}
		if p.PresharedKey != "" {
			psk, err := wgtypes.ParseKey(p.PresharedKey)
			if err != nil {
				return fmt.Errorf("invalid preshared key: %w", err)
			}
			pc.PresharedKey = &psk
		}
		if p.Endpoint != "" {
			addrPort, err := netip.ParseAddrPort(p.Endpoint)
			if err != nil {
				// the kernel backend needs a literal IP:port endpoint; host
				// names must already be resolved by the caller
				return fmt.Errorf("invalid endpoint %q (IP literal required): %w", p.Endpoint, err)
			}
			pc.Endpoint = net.UDPAddrFromAddrPort(addrPort)
		}
		ka := time.Duration(p.PersistentKeepalive) * time.Second
		pc.PersistentKeepaliveInterval = &ka
		for _, a := range p.AllowedIPs {
			_, ipNet, err := net.ParseCIDR(a)
			if err != nil {
				return fmt.Errorf("invalid allowed ip %q: %w", a, err)
			}
			pc.AllowedIPs = append(pc.AllowedIPs, *ipNet)
		}
		kc.Peers = append(kc.Peers, pc)
	}
	return k.client.ConfigureDevice(name, kc)
}

func (k *Kernel) SetAddresses(name string, addresses []string) error {
	return addAddresses(name, addresses)
}

func (k *Kernel) SetMTU(name string, mtu int) error {
	return setLinkMTU(name, mtu)
}

func (k *Kernel) Up(name string) error { return setLinkUp(name) }

func (k *Kernel) Down(name string) error { return setLinkDown(name) }

func (k *Kernel) Remove(name string) error {
	_ = k.Down(name)
	return delWGLink(name)
}

func (k *Kernel) Status(name string) (DeviceStatus, error) {
	dev, err := k.client.Device(name)
	if err != nil {
		return DeviceStatus{Running: false}, nil
	}
	st := DeviceStatus{
		Running:    true,
		PublicKey:  dev.PublicKey.String(),
		ListenPort: dev.ListenPort,
	}
	for _, p := range dev.Peers {
		ps := PeerStatus{
			PublicKey:           p.PublicKey.String(),
			TransferRx:          uint64(p.ReceiveBytes),
			TransferTx:          uint64(p.TransmitBytes),
			PersistentKeepalive: int(p.PersistentKeepaliveInterval / time.Second),
		}
		if p.Endpoint != nil {
			ps.Endpoint = p.Endpoint.String()
		}
		for _, a := range p.AllowedIPs {
			if ip4 := a.IP.To4(); ip4 != nil {
				ps.AllowedIPs = append(ps.AllowedIPs, (&net.IPNet{IP: ip4, Mask: a.Mask}).String())
			} else {
				ps.AllowedIPs = append(ps.AllowedIPs, a.String())
			}
		}
		if !p.LastHandshakeTime.IsZero() {
			ps.LatestHandshake = p.LastHandshakeTime
			ps.Connected = time.Since(ps.LatestHandshake) < 2*time.Minute
		}
		st.Peers = append(st.Peers, ps)
	}
	return st, nil
}
