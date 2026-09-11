// Command skyfire-client is the one-click desktop client for Skyfire: it
// fetches a peer configuration through a token-scoped URL and runs an
// embedded userspace WireGuard tunnel behind a system tray icon.
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/holihur/skyfire/client/internal/app"
	"github.com/holihur/skyfire/client/internal/config"
	"github.com/holihur/skyfire/client/internal/ui"
)

// version is injected at release time via -ldflags -X main.version=<ver>.
var version = "dev"

func main() {
	var (
		connect     = flag.String("connect", "", "Skyfire connection string (peer config URL with token); saved and used on later runs")
		confPath    = flag.String("conf", "", "use a local WireGuard .conf file instead of fetching from the server")
		dryRun      = flag.Bool("dry-run", false, "validate the configuration without creating a tunnel")
		cliMode     = flag.Bool("cli", false, "run in the terminal instead of the system tray")
		verbose     = flag.Bool("verbose", false, "enable debug logging")
		showVersion = flag.Bool("version", false, "print the version and exit")
	)
	flag.Parse()

	if *showVersion {
		fmt.Println("skyfire-client", version)
		return
	}

	level := slog.LevelInfo
	if *verbose {
		level = slog.LevelDebug
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	store, err := config.Open()
	if err != nil {
		log.Error("open client configuration", "error", err)
		os.Exit(1)
	}
	if *connect != "" {
		if err := store.SetConnect(*connect); err != nil {
			log.Error("invalid connection string", "error", err)
			os.Exit(1)
		}
		log.Info("connection string saved")
	}

	a := app.New(store, log)
	if *dryRun {
		a.SetDryRun(true)
	}
	if *confPath != "" {
		data, err := os.ReadFile(*confPath)
		if err != nil {
			log.Error("read configuration file", "error", err)
			os.Exit(1)
		}
		a.SetLocalConf(string(data))
	}

	if *dryRun {
		if err := a.Connect(); err != nil {
			log.Error("configuration check failed", "error", err)
			os.Exit(1)
		}
		return
	}

	if *cliMode {
		if err := ui.RunCLI(a, log); err != nil {
			log.Error("client stopped", "error", err)
			os.Exit(1)
		}
		return
	}
	if err := ui.Run(a, log); err != nil {
		log.Error("client stopped", "error", err)
		os.Exit(1)
	}
}
