package driver

import (
	"fmt"
	"sync"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// Mock is a purely in-memory driver. It performs no system calls and is
// intended for development, testing, and the built-in demo mode. Never use it
// for real traffic.
type Mock struct {
	sync.Mutex
	devs map[string]*mockDevice
}

type mockDevice struct {
	mtu int
	cfg Config
	up  bool
}

// NewMock creates a mock driver.
func NewMock() *Mock { return &Mock{devs: make(map[string]*mockDevice)} }

func (m *Mock) Name() string { return "mock" }

func (m *Mock) UsesOSStack() bool { return false }

func (m *Mock) Close() error {
	m.Lock()
	defer m.Unlock()
	m.devs = make(map[string]*mockDevice)
	return nil
}

func (m *Mock) Create(name string, mtu int) error {
	m.Lock()
	defer m.Unlock()
	if m.devs[name] == nil {
		m.devs[name] = &mockDevice{mtu: mtu, up: true}
	}
	return nil
}

func (m *Mock) Configure(name string, cfg Config) error {
	m.Lock()
	defer m.Unlock()
	d, ok := m.devs[name]
	if !ok {
		return fmt.Errorf("interface %s not found", name)
	}
	d.cfg = cfg
	return nil
}

func (m *Mock) SetAddresses(name string, _ []string) error {
	return m.check(name)
}

func (m *Mock) SetMTU(name string, mtu int) error {
	m.Lock()
	defer m.Unlock()
	d, ok := m.devs[name]
	if !ok {
		return fmt.Errorf("interface %s not found", name)
	}
	d.mtu = mtu
	return nil
}

func (m *Mock) Up(name string) error {
	m.Lock()
	defer m.Unlock()
	d, ok := m.devs[name]
	if !ok {
		return fmt.Errorf("interface %s not found", name)
	}
	d.up = true
	return nil
}

func (m *Mock) Down(name string) error {
	m.Lock()
	defer m.Unlock()
	d, ok := m.devs[name]
	if !ok {
		return fmt.Errorf("interface %s not found", name)
	}
	d.up = false
	return nil
}

func (m *Mock) Remove(name string) error {
	m.Lock()
	defer m.Unlock()
	delete(m.devs, name)
	return nil
}

func (m *Mock) check(name string) error {
	m.Lock()
	defer m.Unlock()
	if _, ok := m.devs[name]; !ok {
		return fmt.Errorf("interface %s not found", name)
	}
	return nil
}

func (m *Mock) Status(name string) (DeviceStatus, error) {
	m.Lock()
	defer m.Unlock()
	d, ok := m.devs[name]
	if !ok {
		return DeviceStatus{Running: false}, nil
	}
	st := DeviceStatus{Running: d.up}
	if priv, err := wgtypes.ParseKey(d.cfg.PrivateKey); err == nil {
		st.PublicKey = priv.PublicKey().String()
	}
	st.ListenPort = d.cfg.ListenPort
	for _, spec := range d.cfg.Peers {
		ps := PeerStatus{
			PublicKey:           spec.PublicKey,
			Endpoint:            spec.Endpoint,
			AllowedIPs:          spec.AllowedIPs,
			PersistentKeepalive: spec.PersistentKeepalive,
			TransferRx:          uint64(time.Now().UnixNano()/1e6) % 10000000,
			TransferTx:          uint64(time.Now().UnixNano()/1e6/7) % 1000000,
		}
		if d.up {
			ps.LatestHandshake = time.Now().Add(-30 * time.Second)
			ps.Connected = true
		}
		st.Peers = append(st.Peers, ps)
	}
	return st, nil
}
