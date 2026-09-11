package manager

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skyfire/internal/driver"
)

func TestNonNilSlices(t *testing.T) {
	if nonNil(nil) == nil {
		t.Fatal("nonNil(nil) should return an empty slice")
	}
	if s := nonNil([]string{"a"}); len(s) != 1 {
		t.Fatal("nonNil should preserve non-nil slices")
	}
}

func TestInterfaceViewMarshalEmptyPeersAsArray(t *testing.T) {
	conf := filepath.Join(t.TempDir(), "skyfire.json")
	if err := os.WriteFile(conf, []byte(`{
  "version": 1,
  "interfaces": [
    {
      "name": "wg0",
      "publicKey": "k",
      "listenPort": 51820,
      "mtu": 1420
    }
  ]
}`), 0o600); err != nil {
		t.Fatal(err)
	}
	m, err := New(conf, driver.NewMock(), true, nil)
	if err != nil {
		t.Fatal(err)
	}
	v, err := m.Get("wg0")
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{`"peers":[]`, `"addresses":[]`, `"dns":[]`} {
		if !strings.Contains(s, want) {
			t.Fatalf("expected %s in:\n%s", want, s)
		}
	}
	if strings.Contains(s, `:null`) {
		t.Fatalf("JSON must not contain null fields:\n%s", s)
	}
}