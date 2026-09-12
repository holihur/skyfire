// Package tunnel brings up a userspace WireGuard tunnel from a wg-quick
// configuration file and configures the host interface and routes best-effort.
//
// The tunnel device itself is wireguard-go; only the OS plumbing (address,
// MTU, routes, DNS) is platform specific. Linux support is provided for
// local testing; Windows and macOS are the supported desktop targets.
package tunnel

import (
	"fmt"
	"log/slog"
	"sync"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
)

// defaultMTU matches the server's default and wg-quick.
const defaultMTU = 1420

// routeTable is the policy-routing table used for full tunnels. It shares the
// value of the device firewall mark (fwmark). 51821 avoids clashing with
// wg-quick, which uses 51820 and may already be present on the client.
const routeTable = 51821

// Tunnel owns a wireguard-go device and its TUN interface.
type Tunnel struct {
	mu     sync.Mutex
	dev    *device.Device
	name   string
	conf   *Conf
	router *whitelistRouter
	log    *slog.Logger
}

// New creates an idle tunnel.
func New(log *slog.Logger) *Tunnel {
	if log == nil {
		log = slog.Default()
	}
	return &Tunnel{log: log}
}

// Up parses text, creates the TUN interface and brings WireGuard up. When
// whitelist is non-empty the tunnel enters split-by-domain mode: only the
// listed CIDRs/hostnames are routed through the tunnel and the system resolver
// is pointed at a local split-DNS proxy.
func (t *Tunnel) Up(text string, whitelist []string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.dev != nil {
		return fmt.Errorf("tunnel is already up on %s", t.name)
	}

	c, err := Parse(text)
	if err != nil {
		return err
	}
	c.Whitelist = whitelist
	uapi, err := c.UAPI()
	if err != nil {
		return err
	}
	mtu := c.MTU
	if mtu <= 0 {
		mtu = defaultMTU
	}

	name := tunName()
	tunDev, err := tun.CreateTUN(name, mtu)
	if err != nil {
		return fmt.Errorf("create tun: %w", err)
	}
	realName, err := tunDev.Name()
	if err != nil || realName == "" {
		realName = name
	}

	dev := device.NewDevice(tunDev, conn.NewDefaultBind(), device.NewLogger(device.LogLevelError, "skyfire-client: "))
	if err := dev.IpcSet(uapi); err != nil {
		dev.Close()
		return fmt.Errorf("apply configuration: %w", err)
	}
	if err := dev.Up(); err != nil {
		dev.Close()
		return fmt.Errorf("bring device up: %w", err)
	}
	if err := configureLink(realName, c); err != nil {
		dev.Close()
		return fmt.Errorf("configure %s: %w", realName, err)
	}
	if c.WhitelistMode() {
		router := newWhitelistRouter(realName, c, t.log)
		router.start()
		t.router = router
	}
	if len(c.DNS) > 0 && !c.WhitelistMode() {
		if err := configureDNS(realName, c.DNS); err != nil {
			t.log.Warn("apply DNS failed", "interface", realName, "servers", c.DNS, "error", err)
		} else {
			t.log.Info("dns configured", "interface", realName, "servers", c.DNS)
		}
	}

	t.dev = dev
	t.name = realName
	t.conf = c
	t.log.Info("tunnel up", "interface", realName)
	return nil
}

// Down tears the tunnel down and removes the OS configuration.
func (t *Tunnel) Down() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.dev == nil {
		return nil
	}
	if t.router != nil {
		t.router.stop()
		t.router = nil
	}
	if t.conf != nil {
		if !t.conf.WhitelistMode() {
			unconfigureDNS(t.name, t.conf.DNS)
		}
		unconfigureLink(t.name, t.conf)
	}
	if err := t.dev.Down(); err != nil {
		t.log.Warn("device down", "error", err)
	}
	t.dev.Close()
	t.dev = nil
	t.conf = nil
	t.log.Info("tunnel down", "interface", t.name)
	return nil
}

// IsUp reports whether the tunnel is currently up.
func (t *Tunnel) IsUp() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.dev != nil
}

// Interface returns the OS interface name of the tunnel.
func (t *Tunnel) Interface() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.name
}
