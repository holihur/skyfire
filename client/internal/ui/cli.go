//go:build !(windows || (darwin && cgo))

package ui

import (
	"log/slog"
)

// Run uses the terminal fallback: no tray backend is compiled into this
// binary (Linux, or macOS built without cgo).
func Run(c Controller, log *slog.Logger) error {
	log.Info("system tray not available in this build; using terminal mode")
	return RunCLI(c, log)
}
