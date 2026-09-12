package ui

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/holihur/skyfire/client/internal/app"
)

var pngSignature = []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}

func TestDotICO(t *testing.T) {
	b := dotICO(0x22, 0xc5, 0x5e)
	if len(b) < 22+len(pngSignature) {
		t.Fatalf("ico too short: %d bytes", len(b))
	}
	// ICONDIR: reserved=0, type=1 (icon), count=1.
	if b[0] != 0 || b[1] != 0 || b[2] != 1 || b[3] != 0 {
		t.Fatalf("bad ICONDIR header: % x", b[:4])
	}
	if got := binary.LittleEndian.Uint16(b[4:6]); got != 1 {
		t.Fatalf("image count = %d, want 1", got)
	}
	// ICONDIRENTRY: width/height.
	if b[6] != dotSize || b[7] != dotSize {
		t.Fatalf("entry size = %dx%d, want %dx%d", b[6], b[7], dotSize, dotSize)
	}
	size := binary.LittleEndian.Uint32(b[14:18])
	offset := binary.LittleEndian.Uint32(b[18:22])
	if offset != 22 {
		t.Fatalf("image offset = %d, want 22", offset)
	}
	if int(offset)+int(size) != len(b) {
		t.Fatalf("offset(%d)+size(%d) != total(%d)", offset, size, len(b))
	}
	if !bytes.Equal(b[offset:offset+8], pngSignature) {
		t.Fatalf("payload is not PNG: % x", b[offset:offset+8])
	}
}

func TestDotPNG(t *testing.T) {
	b := dotPNG(0xef, 0x44, 0x44)
	if !bytes.Equal(b[:8], pngSignature) {
		t.Fatalf("dotPNG did not produce a PNG: % x", b[:8])
	}
}

func TestStatusColor(t *testing.T) {
	r, g, bl := statusColor(app.Connected)
	if !(g > r && g > bl) {
		t.Errorf("Connected color = %d,%d,%d, want dominant green", r, g, bl)
	}
	r, g, bl = statusColor(app.Error)
	if !(r > g && r > bl) {
		t.Errorf("Error color = %d,%d,%d, want dominant red", r, g, bl)
	}
}
