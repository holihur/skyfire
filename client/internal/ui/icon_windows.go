//go:build windows

package ui

import "github.com/holihur/skyfire/client/internal/app"

// iconFor returns a Windows ICO for the given status. The systray library
// loads the bytes with LoadImage(IMAGE_ICON), which requires ICO, not PNG.
func iconFor(s app.Status) []byte {
	r, g, b := statusColor(s)
	return dotICO(r, g, b)
}
