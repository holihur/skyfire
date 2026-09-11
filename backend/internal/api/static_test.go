package api

import (
	"net/http/httptest"
	"testing"
)

func TestSetStaticCache(t *testing.T) {
	const immutable = "public, max-age=31536000, immutable"
	cases := []struct{ name, want string }{
		{"/assets/index-abc123.js", immutable},
		{"assets/index-abc123.css", immutable},
		{"index.html", "no-cache"},
		{"/", "no-cache"},
		{"/interfaces/wg0", "no-cache"},
	}
	for _, tc := range cases {
		w := httptest.NewRecorder()
		setStaticCache(w, tc.name)
		if got := w.Header().Get("Cache-Control"); got != tc.want {
			t.Errorf("setStaticCache(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}
