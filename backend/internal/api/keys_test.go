package api

import (
	"encoding/base64"
	"testing"
)

func TestURLKeyRoundtrip(t *testing.T) {
	// This key happens to contain a '+', which must survive the trip through a
	// URL path segment.
	raw := "QFPxZvUXP02QHmMMoTIb6bNc3c7mHwFsAYUZGwZz+wo="
	enc := urlEncodeKey(raw)
	if enc == raw {
		t.Fatalf("expected a URL-safe transform, got unchanged %q", enc)
	}
	for _, c := range enc {
		if c == '+' || c == '/' || c == '=' {
			t.Fatalf("URL-safe key contains unsafe char %q", c)
		}
	}
	got, err := urlDecodeKey(enc)
	if err != nil {
		t.Fatalf("urlDecodeKey: %v", err)
	}
	if got != raw {
		t.Fatalf("roundtrip mismatch: got %q want %q", got, raw)
	}
}

func TestURLKeyRoundtripRandom(t *testing.T) {
	for i := 0; i < 100; i++ {
		b := make([]byte, 32)
		for j := range b {
			b[j] = byte(i*7 + j*13)
		}
		raw := base64.StdEncoding.EncodeToString(b)
		enc := urlEncodeKey(raw)
		got, err := urlDecodeKey(enc)
		if err != nil {
			t.Fatalf("key %q: %v", raw, err)
		}
		if got != raw {
			t.Fatalf("roundtrip mismatch: %q -> %q", raw, got)
		}
	}
}
