//go:build darwin && cgo

package ui

import "github.com/holihur/skyfire/client/internal/app"

// iconFor returns a PNG for the given status; the macOS systray accepts PNG
// (and ICO/JPG) icon bytes.
func iconFor(s app.Status) []byte {
	r, g, b := statusColor(s)
	return dotPNG(r, g, b)
}
