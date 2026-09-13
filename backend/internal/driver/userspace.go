package driver

import (
	"fmt"
	"reflect"
	"sync"
	"time"
	"unsafe"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// Userspace is the in-process wireguard-go backend. It creates a real TUN
// device and runs the full WireGuard stack in this process, so it requires the
// same privileges as wireguard-go itself (CAP_NET_ADMIN / running as root).
type Userspace struct {
	sync.Mutex
	logger *device.Logger
	devs   map[string]*wgDevice
}

type wgDevice struct {
	dev  *device.Device
	tun  tun.Device
	last Config
}

// NewUserspace creates a userspace driver.
func NewUserspace() *Userspace {
	return &Userspace{
		logger: device.NewLogger(device.LogLevelError, "wireguard-go "),
		devs:   make(map[string]*wgDevice),
	}
}

func (u *Userspace) Name() string { return "userspace" }

func (u *Userspace) UsesOSStack() bool { return true }

func (u *Userspace) Open() error { return nil }

func (u *Userspace) Close() error {
	u.Lock()
	defer u.Unlock()
	for name, d := range u.devs {
		d.dev.Close()
		_ = d.tun.Close()
		delete(u.devs, name)
		_ = delWGLink(name)
	}
	return nil
}

func (u *Userspace) get(name string) *wgDevice {
	u.Lock()
	defer u.Unlock()
	return u.devs[name]
}

func (u *Userspace) set(name string, d *wgDevice) {
	u.Lock()
	defer u.Unlock()
	u.devs[name] = d
}

func (u *Userspace) drop(name string) {
	u.Lock()
	defer u.Unlock()
	delete(u.devs, name)
}

func (u *Userspace) Create(name string, mtu int) error {
	if u.get(name) != nil {
		_ = u.SetMTU(name, mtu)
		return nil
	}
	if mtu == 0 {
		mtu = 1420
	}
	tunDev, err := tun.CreateTUN(name, mtu)
	if err != nil {
		return fmt.Errorf("create tun %s: %w", name, err)
	}
	dev := device.NewDevice(tunDev, conn.NewStdNetBind(), u.logger)
	u.set(name, &wgDevice{dev: dev, tun: tunDev})
	return nil
}

func (u *Userspace) Configure(name string, cfg Config) error {
	wd := u.get(name)
	if wd == nil {
		return fmt.Errorf("interface %s not created", name)
	}
	text, err := renderUAPI(cfg)
	if err != nil {
		return err
	}
	if err := wd.dev.IpcSet(text); err != nil {
		return fmt.Errorf("configure %s: %w", name, err)
	}
	wd.last = cfg
	return nil
}

func (u *Userspace) SetAddresses(name string, addresses []string) error {
	return addAddresses(name, addresses)
}

func (u *Userspace) SetMTU(name string, mtu int) error {
	return setLinkMTU(name, mtu)
}

func (u *Userspace) Up(name string) error {
	wd := u.get(name)
	if wd == nil {
		return fmt.Errorf("interface %s not created", name)
	}
	if err := wd.dev.Up(); err != nil {
		return err
	}
	return setLinkUp(name)
}

func (u *Userspace) Down(name string) error {
	wd := u.get(name)
	if wd == nil {
		return fmt.Errorf("interface %s not created", name)
	}
	if err := wd.dev.Down(); err != nil {
		return err
	}
	return setLinkDown(name)
}

func (u *Userspace) Remove(name string) error {
	wd := u.get(name)
	if wd == nil {
		_ = delWGLink(name)
		u.drop(name)
		return nil
	}
	wd.dev.Close()
	_ = wd.tun.Close()
	u.drop(name)
	return delWGLink(name)
}

func (u *Userspace) ApplyShaping(name string, peers []PeerShaping) error {
	return applyTCShaping(name, peers)
}

func (u *Userspace) RemoveShaping(name string) {
	removeTCShaping(name)
}

// ApplyBlacklist installs the block list as FORWARD drop rules on the OS path.
func (u *Userspace) ApplyBlacklist(name string, entries []string) error {
	return applyBlacklistFirewall(name, entries)
}

func (u *Userspace) Status(name string) (DeviceStatus, error) {
	wd := u.get(name)
	if wd == nil {
		return DeviceStatus{Running: false}, nil
	}
	st := DeviceStatus{Running: true, ListenPort: wd.last.ListenPort}
	if priv, err := wgtypes.ParseKey(wd.last.PrivateKey); err == nil {
		st.PublicKey = priv.PublicKey().String()
	}
	for _, spec := range wd.last.Peers {
		ps := PeerStatus{
			PublicKey:           spec.PublicKey,
			Endpoint:            spec.Endpoint,
			AllowedIPs:          spec.AllowedIPs,
			PersistentKeepalive: spec.PersistentKeepalive,
		}
		ps.LatestHandshake, ps.TransferRx, ps.TransferTx, ps.Connected = liveStats(wd.dev, spec.PublicKey, spec.PersistentKeepalive)
		st.Peers = append(st.Peers, ps)
	}
	return st, nil
}

// liveStats reads per-peer counters directly from the wireguard-go device
// through reflection. The fields are private to the library, so this is
// intentionally defensive: on any failure it simply reports zeroed stats
// rather than erroring out the request.
func liveStats(dev *device.Device, pubKey string, keepalive int) (time.Time, uint64, uint64, bool) {
	var hs time.Time
	var rx, tx uint64
	pk, err := wgtypes.ParseKey(pubKey)
	if err != nil {
		return hs, 0, 0, false
	}
	p := dev.LookupPeer(device.NoisePublicKey(pk))
	if p == nil {
		return hs, 0, 0, false
	}
	defer func() {
		_ = recover()
	}()
	rv := reflect.ValueOf(p).Elem()
	if f := rv.FieldByName("txBytes"); f.IsValid() && f.CanAddr() {
		tx = *(*uint64)(unsafe.Pointer(f.UnsafeAddr()))
	}
	if f := rv.FieldByName("rxBytes"); f.IsValid() && f.CanAddr() {
		rx = *(*uint64)(unsafe.Pointer(f.UnsafeAddr()))
	}
	if f := rv.FieldByName("lastHandshakeNano"); f.IsValid() && f.CanAddr() {
		if ns := *(*int64)(unsafe.Pointer(f.UnsafeAddr())); ns > 0 {
			hs = time.Unix(0, ns)
		}
	}
	connected := false
	if !hs.IsZero() {
		connected = time.Since(hs) < 2*time.Minute
	}
	if keepalive == 0 && !connected && hs.IsZero() {
		// keepalive off and never connected → not connected, handshake time
		// stays zero
	}
	return hs, rx, tx, connected
}
