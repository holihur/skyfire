//go:build !(cgo && (windows || darwin))

package ui

import "log/slog"

// Run falls back to the terminal. The fyne GUI is only built for Windows and
// macOS (with cgo); Linux always runs the terminal client.
func Run(c Controller, log *slog.Logger) error {
	log.Info("using terminal mode")
	return RunCLI(c, log)
}
