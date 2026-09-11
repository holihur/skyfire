package ui

import (
	"bytes"
	"image"
	"image/color"
	"image/png"

	"github.com/holihur/skyfire/client/internal/app"
)

// iconFor renders a small status dot so the binary carries no image assets.
func iconFor(s app.Status) []byte {
	switch s {
	case app.Connected:
		return dot(0x22, 0xc5, 0x5e) // green
	case app.Connecting:
		return dot(0xf5, 0x9e, 0x0b) // amber
	case app.Error:
		return dot(0xef, 0x44, 0x44) // red
	default:
		return dot(0x94, 0xa3, 0xb8) // slate
	}
}

// dot draws a filled circle on a transparent 32x32 canvas.
func dot(r, g, b uint8) []byte {
	const size = 32
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	cx, cy := float64(size)/2-0.5, float64(size)/2-0.5
	rad := float64(size)/2 - 2
	rr := rad * rad
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
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
