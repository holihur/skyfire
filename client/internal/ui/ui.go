// Package ui provides the desktop frontends for the client: a system tray on
// Windows (and macOS when built with cgo), and a terminal fallback elsewhere.
package ui

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/holihur/skyfire/client/internal/app"
)

// Controller is the surface the UI drives.
type Controller interface {
	Connect() error
	Disconnect() error
	Toggle() error
	Status() (app.Status, string)
	ConnectString() string
	SetConnectString(string) error
	Whitelist() []string
	SetWhitelist([]string) error
	AutoConnect() bool
	SetOnChange(func(app.Status, string))
}

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
	<-ch
	log.Info("shutting down")
	return c.Disconnect()
}
