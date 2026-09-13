package ui

import (
	"fmt"

	"github.com/holihur/skyfire/client/internal/app"
	"github.com/holihur/skyfire/client/internal/i18n"
)

// statusLabel renders a tunnel status in the active interface language.
func statusLabel(s app.Status) string {
	switch s {
	case app.Connected:
		return i18n.T("status.connected")
	case app.Connecting:
		return i18n.T("status.connecting")
	case app.Error:
		return i18n.T("status.error")
	default:
		return i18n.T("status.disconnected")
	}
}

// fmtBytes renders a byte count using binary units (KiB, MiB, …).
func fmtBytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := uint64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

// fmtRate renders a bytes-per-second rate.
func fmtRate(bps float64) string {
	switch {
	case bps < 1:
		return "0 B/s"
	case bps < 1024:
		return fmt.Sprintf("%.0f B/s", bps)
	case bps < 1024*1024:
		return fmt.Sprintf("%.1f KiB/s", bps/1024)
	default:
		return fmt.Sprintf("%.1f MiB/s", bps/(1024*1024))
	}
}
