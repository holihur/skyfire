package ui

import (
	"bytes"
	"encoding/binary"
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

// dotPNG draws a filled circle on a transparent canvas and encodes it as PNG.
// macOS (and other non-Windows trays) accept PNG icon bytes.
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

// dotICO wraps a PNG in a single-image ICO container. Windows' SetIcon loads
// the bytes with LoadImage(IMAGE_ICON), which only understands ICO; PNG data
// embedded in an ICO entry is supported since Windows Vista.
func dotICO(r, g, b uint8) []byte {
	png := dotPNG(r, g, b)
	buf := new(bytes.Buffer)
	// ICONDIR header.
	_ = binary.Write(buf, binary.LittleEndian, uint16(0)) // reserved
	_ = binary.Write(buf, binary.LittleEndian, uint16(1)) // type: 1 = icon
	_ = binary.Write(buf, binary.LittleEndian, uint16(1)) // image count
	// ICONDIRENTRY.
	buf.WriteByte(dotSize)                                       // width
	buf.WriteByte(dotSize)                                       // height
	buf.WriteByte(0)                                             // palette colours
	buf.WriteByte(0)                                             // reserved
	_ = binary.Write(buf, binary.LittleEndian, uint16(1))        // colour planes
	_ = binary.Write(buf, binary.LittleEndian, uint16(32))       // bits per pixel
	_ = binary.Write(buf, binary.LittleEndian, uint32(len(png))) // size of image data
	_ = binary.Write(buf, binary.LittleEndian, uint32(6+16))     // offset of image data
	buf.Write(png)
	return buf.Bytes()
}
