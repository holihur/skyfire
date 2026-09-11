package api

import (
	"encoding/base64"
)

// urlEncodeKey converts a standard base64 WireGuard key into a URL-safe,
// unpadded form usable as a path segment.
func urlEncodeKey(k string) string {
	if k == "" {
		return ""
	}
	raw, err := base64.StdEncoding.DecodeString(k)
	if err != nil {
		// tolerate already-url-safe input
		return k
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

// urlDecodeKey restores a standard base64 key from its URL-safe form.
func urlDecodeKey(s string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(raw), nil
}
