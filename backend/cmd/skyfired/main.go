// skyfired is the Skyfire WireGuard management daemon.
//
// It reconciles a persisted desired-state configuration against a WireGuard
// driver (userspace wireguard-go by default, kernel wgctrl, or a safe mock)
// and exposes a JSON REST API with an optional bundled web UI.
//
// IMPORTANT: real network changes require root / CAP_NET_ADMIN. Use
// --dry-run to preview every operation without touching the system.
package main

import (
	"context"
	crand "crypto/rand"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"math/big"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"skyfire/internal/api"
	"skyfire/internal/dnsproxy"
	"skyfire/internal/driver"
	"skyfire/internal/manager"
	"skyfire/internal/store"
	"skyfire/internal/totp"
	"skyfire/internal/web"
)

// Version is injected at release time via -ldflags -X main.version=<ver>.
var version = "dev"

func main() {
	// `skyfired update` (aliased as `skyfire update`) is a self-contained
	// subcommand: handle it before the daemon's own flag parsing.
	if len(os.Args) > 1 && os.Args[1] == "update" {
		os.Exit(runUpdate(os.Args[2:]))
	}
	// `skyfired totp` inspects or resets the two-factor binding.
	if len(os.Args) > 1 && os.Args[1] == "totp" {
		os.Exit(runTOTPCmd(os.Args[2:]))
	}

	var (
		configPath = flag.String("config", "", "path to the persisted configuration file (auto-resolved when empty)")
		addr       = flag.String("addr", ":51821", "listen address for the web server")
		driverName = flag.String("driver", "netstack", "wireguard backend: netstack | userspace | kernel | mock")
		dryRun     = flag.Bool("dry-run", false, "log every live operation without applying it (safe preview)")
		token      = flag.String("token", "", "bearer token required for API access (empty disables auth)")
		username   = flag.String("username", "admin", "username for the single-user web login")
		password   = flag.String("password", "", "password for the single-user web login (auto-generated if empty)")
		totpOn     = flag.Bool("totp", true, "require TOTP two-factor authentication on web login (first login binds an authenticator)")
		static     = flag.String("static", "", "path to the built frontend directory to serve")
		demo       = flag.Bool("demo", false, "start with a mock driver and sample data (no system changes)")
		verbose    = flag.Bool("verbose", false, "debug logging")
		showVer    = flag.Bool("version", false, "print the version and exit")
		dnsAddr    = flag.String("dns", "127.0.0.1:53", "listen address for the tunnel DNS forwarder (empty disables); netstack relays tunnel DNS to loopback")
		dnsUp      = flag.String("dns-upstream", "8.8.8.8:53,1.1.1.1:53", "comma-separated upstream DNS resolvers for the forwarder")
	)
	flag.Usage = func() {
		out := flag.CommandLine.Output()
		fmt.Fprint(out, "Usage: skyfired [flags]\n")
		fmt.Fprint(out, "       skyfired update [flags]   self-update to the latest release\n")
		fmt.Fprint(out, "       skyfired totp [status|reset]   inspect or re-bind two-factor auth\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *showVer {
		fmt.Println("skyfired", version)
		return
	}

	level := slog.LevelInfo
	if *verbose {
		level = slog.LevelDebug
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	if *demo {
		*dryRun = true
		*driverName = "mock"
		*configPath = filepath.Join(os.TempDir(), "skyfire-demo.json")
		if *token == "" {
			*token = "demo"
		}
		if *password == "" {
			*username = "admin"
			*password = "demo"
		}
		// demo mode keeps the login simple: no authenticator needed
		*totpOn = false
	}

	if *configPath == "" {
		*configPath = resolveConfigPath()
	}

	// TOTP two-factor state. Enabled by default; the secret is generated (but
	// not yet bound) so the first successful password login can enroll an
	// authenticator.
	totpPath := ""
	var totpCfg totp.Config
	if *totpOn {
		totpPath = totpFilePath(*configPath)
		loaded, err := totp.Load(totpPath)
		if err != nil {
			log.Error("load totp configuration", "error", err)
			os.Exit(1)
		}
		totpCfg = loaded
		if totpCfg.Secret == "" {
			secret, err := totp.GenerateSecret()
			if err != nil {
				log.Error("generate totp secret", "error", err)
				os.Exit(1)
			}
			totpCfg = totp.Config{Secret: secret}
			if err := totp.Save(totpPath, totpCfg); err != nil {
				log.Error("save totp configuration", "error", err)
				os.Exit(1)
			}
		}
		log.Info("two-factor authentication enabled (TOTP)",
			"bound", totpCfg.Bound(),
			"hint", "bind an authenticator on first login; run 'skyfired totp reset' to re-bind")
	} else {
		log.Warn("two-factor authentication disabled (-totp=false)")
	}

	if *password == "" {
		pw, err := randomPassword()
		if err != nil {
			log.Error("cannot generate password", "error", err)
			os.Exit(1)
		}
		*password = pw
		log.Info("web login credentials (single user)",
			"username", *username,
			"password", pw,
			"hint", "change with --password or wipe the cookie to re-login",
		)
	}

	drv, err := buildDriver(*driverName)
	if err != nil {
		log.Error("driver init failed", "error", err)
		os.Exit(1)
	}
	defer drv.Close()

	mgr, err := manager.New(*configPath, drv, *dryRun, log)
	if err != nil {
		log.Error("manager init failed", "error", err)
		os.Exit(1)
	}

	if *demo {
		seedDemo(mgr, log)
	}

	if *token == "" {
		log.Warn("no auth token configured: the API is unauthenticated")
	} else {
		log.Info("auth enabled (use 'Authorization: Bearer <token>' header)")
	}

	// Tunnel DNS forwarder: clients point DNS at the server's tunnel address,
	// netstack relays it to loopback, and this forwarder resolves it through
	// clean upstreams. Skipped in dry-run/demo and when disabled.
	var dnsProxy *dnsproxy.Server
	if !*dryRun && *dnsAddr != "" {
		if proxy, err := dnsproxy.New(splitDNSUpstreams(*dnsUp), log); err != nil {
			log.Warn("dns proxy config invalid", "error", err)
		} else if err := proxy.Start(*dnsAddr); err != nil {
			log.Warn("dns proxy failed to start", "addr", *dnsAddr, "error", err)
		} else {
			dnsProxy = proxy
		}
	}

	log.Info("skyfired starting",
		"version", version,
		"driver", drv.Name(),
		"dryRun", *dryRun,
		"addr", *addr,
		"config", *configPath,
	)

	srv := api.New(mgr, drv, *dryRun, api.Options{
		StaticDir:  *static,
		Token:      *token,
		Username:   *username,
		Password:   *password,
		TOTPSecret: totpCfg.Secret,
		TOTPBound:  totpCfg.Bound(),
		TOTPPath:   totpPath,
		Version:    version,
		Log:        log,
	})
	if *static == "" {
		sub, err := fs.Sub(web.Dist, "dist")
		if err != nil {
			log.Error("cannot read embedded frontend", "error", err)
			os.Exit(1)
		}
		srv = api.New(mgr, drv, *dryRun, api.Options{
			StaticFS:   sub,
			Token:      *token,
			Username:   *username,
			Password:   *password,
			TOTPSecret: totpCfg.Secret,
			TOTPBound:  totpCfg.Bound(),
			TOTPPath:   totpPath,
			Version:    version,
			Log:        log,
		})
	}
	httpSrv := &http.Server{
		Addr:              *addr,
		Handler:           srv,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Info("listening", "addr", *addr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server failed", "error", err)
			os.Exit(1)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)
	if dnsProxy != nil {
		dnsProxy.Close()
	}
}

// splitDNSUpstreams parses the comma-separated -dns-upstream flag.
func splitDNSUpstreams(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func buildDriver(name string) (driver.Driver, error) {
	switch strings.ToLower(name) {
	case "netstack":
		return driver.NewNetstack(), nil
	case "userspace":
		return driver.NewUserspace(), nil
	case "kernel":
		return driver.NewKernel()
	case "mock":
		return driver.NewMock(), nil
	default:
		return nil, fmt.Errorf("unknown driver %q (want netstack | userspace | kernel | mock)", name)
	}
}

// seedDemo populates a mock manager with an example router and two peers.
func seedDemo(mgr *manager.Manager, log *slog.Logger) {
	_ = mgr.UpdateSettings(store.Settings{PublicEndpoint: "vpn.example.com"})

	iface := &store.Interface{
		Name:       "wg0",
		ListenPort: 51820,
		Addresses:  []string{"10.42.0.1/24"},
		MTU:        store.DefaultMTU,
		DNS:        []string{"1.1.1.1", "9.9.9.9"},
		Up:         true,
	}
	if _, err := mgr.CreateInterface(iface); err != nil {
		log.Warn("demo interface", "error", err)
		return
	}
	peers := []struct{ name, addr string }{
		{"laptop", "10.42.0.2"},
		{"phone", "10.42.0.3"},
	}
	for _, p := range peers {
		wp := true
		in := manager.PeerInput{
			Name:                p.name,
			Address:             p.addr,
			GenerateKeys:        true,
			WithPreshared:       &wp,
			ClientRoutes:        []string{"0.0.0.0/0", "::/0"},
			PersistentKeepalive: 25,
			Enabled:             true,
		}
		if _, err := mgr.CreatePeer("wg0", &in); err != nil {
			log.Warn("demo peer", "name", p.name, "error", err)
		}
	}
}

// randomPassword generates a 16-character credential from a printable charset.
func randomPassword() (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	buf := make([]byte, 16)
	for i := range buf {
		n, err := crand.Int(crand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		buf[i] = charset[n.Int64()]
	}
	return string(buf), nil
}

// resolveConfigPath picks a writable location for the persisted state so the
// daemon can run with no arguments (as root it keeps the system path).
func resolveConfigPath() string {
	candidates := []string{
		"/etc/skyfire/config.json",
		"",
	}
	for _, c := range candidates {
		if c != "" && canWriteDir(filepath.Dir(c)) {
			return c
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			return filepath.Join(xdg, "skyfire", "config.json")
		}
		return filepath.Join(home, ".config", "skyfire", "config.json")
	}
	return "skyfire.json"
}

func canWriteDir(dir string) bool {
	f, err := os.CreateTemp(dir, ".skyfire-write-test-*")
	if err != nil {
		return false
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return true
}
