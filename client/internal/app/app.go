// Package app wires the persisted client configuration to the WireGuard
// tunnel and exposes a small state machine shared by the tray and CLI UIs.
package app

import (
	"context"
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

// Reconnect tuning. A tunnel that uses persistent keepalive refreshes its
// handshake roughly every two minutes, so a gap beyond staleAfter means the
// link is down. graceAfter covers the window right after bring-up before the
// first handshake completes.
const (
	probeInterval = 15 * time.Second
	staleAfter    = 150 * time.Second
	graceAfter    = 90 * time.Second
	backoffBase   = 2 * time.Second
	backoffMax    = 60 * time.Second
)

// App owns the tunnel and the connection string.
type App struct {
	mu          sync.Mutex
	connMu      sync.Mutex // serializes tunnel up/down sequences
	store       *config.Store
	tun         *tunnel.Tunnel
	status      Status
	lastErr     string
	localConf   string
	dryRun      bool
	autoConnect bool
	onChange    func(Status, string)
	log         *slog.Logger

	autoReconnect bool
	desired       bool
	connectedAt   time.Time
	monCancel     context.CancelFunc

	lastTraffic   tunnel.Stats
	lastTrafficAt time.Time
}

// New creates an app backed by the given store.
func New(store *config.Store, log *slog.Logger) *App {
	if log == nil {
		log = slog.Default()
	}
	return &App{store: store, tun: tunnel.New(log), log: log, autoReconnect: true}
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

// Connect fetches (or reuses) the configuration and brings the tunnel up. It
// also arms the reconnect supervisor when auto-reconnect is enabled.
func (a *App) Connect() error {
	a.setDesired(true)
	err := a.bringUp()
	a.watch()
	return err
}

// bringUp loads the configuration and raises the tunnel.
func (a *App) bringUp() error {
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
	a.connMu.Lock()
	err = a.tun.Up(text, a.store.Whitelist())
	a.connMu.Unlock()
	if err != nil {
		a.setStatus(Error, err.Error())
		return err
	}
	a.mu.Lock()
	a.connectedAt = time.Now()
	a.mu.Unlock()
	a.setStatus(Connected, "")
	return nil
}

// Disconnect tears the tunnel down and stops the reconnect supervisor. This is
// the explicit "the user wants the tunnel down" action.
func (a *App) Disconnect() error {
	a.setDesired(false)
	a.stopMonitor()
	a.connMu.Lock()
	err := a.tun.Down()
	a.connMu.Unlock()
	if err != nil {
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

// TrafficStats is a snapshot of tunnel transfer counters from the client's
// point of view: Rx is downloaded, Tx is uploaded. The rates are measured
// against the previous Traffic call, so callers should poll at a steady
// interval.
type TrafficStats struct {
	Rx     uint64 // total bytes received (download)
	Tx     uint64 // total bytes sent (upload)
	RxRate float64
	TxRate float64
	Active bool // true when the tunnel is up and counters are readable
}

// Traffic returns the latest transfer counters and per-second rates. When the
// tunnel is down it returns a zero snapshot with Active=false.
func (a *App) Traffic() TrafficStats {
	st, ok := a.tun.Stats()
	if !ok {
		return TrafficStats{}
	}
	now := time.Now()
	a.mu.Lock()
	prev := a.lastTraffic
	prevAt := a.lastTrafficAt
	a.lastTraffic = st
	a.lastTrafficAt = now
	a.mu.Unlock()

	out := TrafficStats{Rx: st.RxBytes, Tx: st.TxBytes, Active: true}
	if !prevAt.IsZero() {
		if d := now.Sub(prevAt).Seconds(); d > 0 {
			if st.RxBytes >= prev.RxBytes {
				out.RxRate = float64(st.RxBytes-prev.RxBytes) / d
			}
			if st.TxBytes >= prev.TxBytes {
				out.TxRate = float64(st.TxBytes-prev.TxBytes) / d
			}
		}
	}
	return out
}

// SetAutoReconnect enables or disables automatic reconnection after the tunnel
// drops. It is enabled by default. Disabling it never tears an active tunnel
// down; it only stops the supervisor.
func (a *App) SetAutoReconnect(v bool) {
	a.mu.Lock()
	a.autoReconnect = v
	a.mu.Unlock()
}

func (a *App) setDesired(v bool) {
	a.mu.Lock()
	a.desired = v
	a.mu.Unlock()
}

func (a *App) isDesired() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.desired
}

// watch (re)starts the reconnect supervisor when auto-reconnect is enabled.
func (a *App) watch() {
	a.mu.Lock()
	if !a.autoReconnect || a.dryRun {
		a.mu.Unlock()
		return
	}
	if a.monCancel != nil {
		a.monCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.monCancel = cancel
	a.mu.Unlock()
	go a.monitor(ctx)
}

func (a *App) stopMonitor() {
	a.mu.Lock()
	cancel := a.monCancel
	a.monCancel = nil
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// monitor keeps the tunnel alive: it probes link health and re-establishes the
// tunnel with exponential backoff when the link drops. It exits when the
// context is cancelled (Disconnect) or when the tunnel is no longer desired.
func (a *App) monitor(ctx context.Context) {
	ticker := time.NewTicker(probeInterval)
	defer ticker.Stop()
	fails := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		if !a.isDesired() {
			return
		}
		if a.healthy() {
			fails = 0
			continue
		}
		fails++
		wait := backoff(fails)
		a.log.Warn("tunnel unhealthy; reconnecting", "attempt", fails, "wait", wait.String())
		a.setStatus(Connecting, "reconnecting")
		a.connMu.Lock()
		_ = a.tun.Down()
		a.connMu.Unlock()
		if !sleepCtx(ctx, wait) || !a.isDesired() {
			return
		}
		if err := a.bringUp(); err != nil {
			a.log.Warn("reconnect attempt failed", "error", err)
			continue
		}
		if !a.isDesired() {
			// Disconnect raced with the rebuild; honor it.
			a.connMu.Lock()
			_ = a.tun.Down()
			a.connMu.Unlock()
			a.setStatus(Disconnected, "")
			return
		}
		a.log.Info("tunnel reconnected")
		fails = 0
	}
}

// healthy reports whether the tunnel is up and its handshakes are fresh. A
// tunnel without persistent keepalive is treated as healthy while up, because
// an idle peer legitimately has no recent handshake.
func (a *App) healthy() bool {
	if !a.tun.IsUp() {
		return false
	}
	h, ok := a.tun.Stats()
	if !ok {
		return false
	}
	if !h.HasKeepalive {
		return true
	}
	if h.Last.IsZero() {
		a.mu.Lock()
		since := time.Since(a.connectedAt)
		a.mu.Unlock()
		return since < graceAfter
	}
	return time.Since(h.Last) < staleAfter
}

// backoff returns the delay before the given reconnect attempt (1-based),
// growing exponentially and capped at backoffMax.
func backoff(attempt int) time.Duration {
	d := backoffBase
	for i := 1; i < attempt && d < backoffMax; i++ {
		d *= 2
	}
	if d > backoffMax {
		d = backoffMax
	}
	return d
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

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
