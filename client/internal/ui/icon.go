package ui

import (
	"bytes"
	"image"
	"image/color"
	"image/png"

	"github.com/holihur/skyfire/client/internal/app"
)

// dotSize is the tray icon edge length in pixels.
const dotSize = 32

// statusColor maps a tunnel status to the dot colour.
func statusColor(s app.Status) (r, g, b uint8) {
	switch s {
	case app.Connected:
		return 0x22, 0xc5, 0x5e // green
	case app.Connecting:
		return 0xf5, 0x9e, 0x0b // amber
	case app.Error:
		return 0xef, 0x44, 0x44 // red
	default:
		return 0x94, 0xa3, 0xb8 // slate
	}
}

// dotPNG draws a filled circle on a transparent canvas and encodes it as PNG,
// ready for fyne.NewStaticResource.
func dotPNG(r, g, b uint8) []byte {
	img := image.NewRGBA(image.Rect(0, 0, dotSize, dotSize))
	cx, cy := float64(dotSize)/2-0.5, float64(dotSize)/2-0.5
	rad := float64(dotSize)/2 - 2
	rr := rad * rad
	for y := 0; y < dotSize; y++ {
		for x := 0; x < dotSize; x++ {
			dx, dy := float64(x)-cx, float64(y)-cy
			if dx*dx+dy*dy <= rr {
				img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
			}
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}
