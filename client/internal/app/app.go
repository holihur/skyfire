// Package app wires the persisted client configuration to the WireGuard
// tunnel and exposes a small state machine shared by the tray and CLI UIs.
package app

import (
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/holihur/skyfire/client/internal/config"
	"github.com/holihur/skyfire/client/internal/tunnel"
)

// Status is the tunnel state shown to the user.
type Status int

const (
	Disconnected Status = iota
	Connecting
	Connected
	Error
)

// String implements fmt.Stringer.
func (s Status) String() string {
	switch s {
	case Connecting:
		return "Connecting"
	case Connected:
		return "Connected"
	case Error:
		return "Error"
	default:
		return "Disconnected"
	}
}

// App owns the tunnel and the connection string.
type App struct {
	mu          sync.Mutex
	store       *config.Store
	tun         *tunnel.Tunnel
	status      Status
	lastErr     string
	localConf   string
	dryRun      bool
	autoConnect bool
	onChange    func(Status, string)
	log         *slog.Logger
}

// New creates an app backed by the given store.
func New(store *config.Store, log *slog.Logger) *App {
	if log == nil {
		log = slog.Default()
	}
	return &App{store: store, tun: tunnel.New(log), log: log}
}

// SetLocalConf makes the app use a local WireGuard configuration instead of
// fetching it from the server (useful for testing and offline use).
func (a *App) SetLocalConf(text string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.localConf = text
}

// SetDryRun disables all system changes; Connect only validates the config.
func (a *App) SetDryRun(v bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.dryRun = v
}

// SetOnChange registers a state listener. It is called from the goroutine
// that changed the state.
func (a *App) SetOnChange(fn func(Status, string)) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.onChange = fn
}

// Status returns the current state and the last error message, if any.
func (a *App) Status() (Status, string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.status, a.lastErr
}

// ConnectString returns the saved connection string.
func (a *App) ConnectString() string { return a.store.Connect() }

// SetConnectString validates and saves the connection string.
func (a *App) SetConnectString(v string) error { return a.store.SetConnect(v) }

// Whitelist returns the saved domain/CIDR whitelist.
func (a *App) Whitelist() []string { return a.store.Whitelist() }

// SetWhitelist validates and saves the whitelist.
func (a *App) SetWhitelist(v []string) error { return a.store.SetWhitelist(v) }

// SetAutoConnect requests that the tunnel be brought up automatically once the
// UI is ready (used when the client is launched with -connect).
func (a *App) SetAutoConnect(v bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.autoConnect = v
}

// AutoConnect reports whether the tunnel should be brought up automatically.
func (a *App) AutoConnect() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.autoConnect
}

// Connect fetches (or reuses) the configuration and brings the tunnel up.
func (a *App) Connect() error {
	a.setStatus(Connecting, "")
	text, err := a.loadConfig()
	if err != nil {
		a.setStatus(Error, err.Error())
		return err
	}
	if a.isDryRun() {
		c, err := tunnel.Parse(text)
		if err != nil {
			a.setStatus(Error, err.Error())
			return err
		}
		v4, v6 := c.HasDefaultRoutes()
		a.log.Info("dry-run: configuration is valid",
			"addresses", c.Addresses,
			"dns", c.DNS,
			"mtu", c.MTU,
			"peers", len(c.Peers),
			"fullTunnelV4", v4,
			"fullTunnelV6", v6,
			"whitelist", a.store.Whitelist(),
		)
		for _, p := range c.Peers {
			a.log.Info("dry-run: peer", "endpoint", p.Endpoint, "allowedIPs", p.AllowedIPs, "keepalive", p.Keepalive)
		}
		a.setStatus(Connected, "dry-run")
		return nil
	}
	if err := a.tun.Up(text, a.store.Whitelist()); err != nil {
		a.setStatus(Error, err.Error())
		return err
	}
	a.setStatus(Connected, "")
	return nil
}

// Disconnect tears the tunnel down.
func (a *App) Disconnect() error {
	if err := a.tun.Down(); err != nil {
		a.setStatus(Error, err.Error())
		return err
	}
	a.setStatus(Disconnected, "")
	return nil
}

// Toggle connects when inactive and disconnects otherwise.
func (a *App) Toggle() error {
	s, _ := a.Status()
	if s == Connected || s == Connecting {
		return a.Disconnect()
	}
	return a.Connect()
}

// Describe returns the interface name of the active tunnel (or "").
func (a *App) Describe() string { return a.tun.Interface() }

func (a *App) isDryRun() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.dryRun
}

func (a *App) loadConfig() (string, error) {
	a.mu.Lock()
	local := a.localConf
	a.mu.Unlock()
	if local != "" {
		return local, nil
	}
	conn := a.store.Connect()
	if conn == "" {
		return "", errors.New("no connection string configured")
	}
	text, err := config.Fetch(conn, 15*time.Second)
	if err != nil {
		if cached, cerr := config.LoadCached(); cerr == nil && cached != "" {
			a.log.Warn("using cached configuration", "error", err)
			return cached, nil
		}
		return "", err
	}
	return text, nil
}

func (a *App) setStatus(s Status, msg string) {
	a.mu.Lock()
	a.status = s
	a.lastErr = msg
	fn := a.onChange
	a.mu.Unlock()
	if fn != nil {
		fn(s, msg)
	}
}
