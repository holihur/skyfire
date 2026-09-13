// Package ui provides the desktop frontends for the client: a system tray on
// Windows (and macOS when built with cgo), and a terminal fallback elsewhere.
package ui

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/holihur/skyfire/client/internal/app"
)

// Controller is the surface the UI drives.
type Controller interface {
	Connect() error
	Disconnect() error
	Toggle() error
	Status() (app.Status, string)
	Traffic() app.TrafficStats
	Latency() (time.Duration, bool)
	ConnectString() string
	SetConnectString(string) error
	Whitelist() []string
	SetWhitelist([]string) error
	Lang() string
	SetLang(string) error
	AutoConnect() bool
	SetOnChange(func(app.Status, string))
}

// trafficLogInterval is how often the terminal UI prints a traffic line.
const trafficLogInterval = 5 * time.Second

// RunCLI connects if needed and blocks until the process is interrupted.
func RunCLI(c Controller, log *slog.Logger) error {
	if s, _ := c.Status(); s != app.Connected && s != app.Connecting {
		if err := c.Connect(); err != nil {
			return err
		}
	}
	log.Info("tunnel active; press Ctrl+C to disconnect and exit")
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)

	stop := make(chan struct{})
	go func() {
		t := time.NewTicker(trafficLogInterval)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				if s, _ := c.Status(); s != app.Connected {
					continue
				}
				tr := c.Traffic()
				if !tr.Active {
					continue
				}
				log.Info("traffic",
					"down", fmtBytes(tr.Rx), "up", fmtBytes(tr.Tx),
					"downRate", fmtRate(tr.RxRate), "upRate", fmtRate(tr.TxRate))
			}
		}
	}()

	<-ch
	close(stop)
	log.Info("shutting down")
	return c.Disconnect()
}
