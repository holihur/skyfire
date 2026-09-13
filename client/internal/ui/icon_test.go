package ui

import (
	"bytes"
	"testing"

	"github.com/holihur/skyfire/client/internal/app"
)

var pngSignature = []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}

func TestDotPNG(t *testing.T) {
	b := dotPNG(0x22, 0xc5, 0x5e)
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
