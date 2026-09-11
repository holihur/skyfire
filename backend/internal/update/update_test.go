package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestAssetName(t *testing.T) {
	cases := []struct {
		goos, goarch string
		want         string
		wantErr      bool
	}{
		{"linux", "amd64", "skyfired-linux-amd64", false},
		{"linux", "arm64", "skyfired-linux-arm64", false},
		{"darwin", "arm64", "skyfired-darwin-arm64", false},
		{"windows", "amd64", "skyfired-windows-amd64.exe", false},
		{"windows", "arm64", "", true},
		{"plan9", "amd64", "", true},
		{"linux", "mips", "", true},
	}
	for _, tc := range cases {
		got, err := assetName(tc.goos, tc.goarch)
		if tc.wantErr {
			if err == nil {
				t.Errorf("assetName(%s,%s): expected error, got %q", tc.goos, tc.goarch, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("assetName(%s,%s): unexpected error %v", tc.goos, tc.goarch, err)
		}
		if got != tc.want {
			t.Errorf("assetName(%s,%s) = %q, want %q", tc.goos, tc.goarch, got, tc.want)
		}
	}
}

func TestNewer(t *testing.T) {
	cases := []struct {
		current, latest string
		newer           bool
		comparable      bool
	}{
		{"v0.1.0", "v0.2.0", true, true},
		{"v0.1.0", "v0.1.0", false, true},
		{"v0.2.0", "v0.1.0", false, true},
		{"0.1.0", "v0.1.1", true, true},
		{"v1.0.0", "v1.0.0-rc.1", false, true},
		{"v1.0.0-rc.1", "v1.0.0", true, true},
		{"v1.0.0-rc.1", "v1.0.0-rc.2", true, true},
		{"v1.0.0-alpha", "v1.0.0-beta", true, true},
		{"v1.2", "v1.2.0", false, true}, // missing components default to 0
		{"v1.2.1", "v1.2", false, true},
		{"dev", "v0.2.0", false, false},
		{"v0.1.0", "nightly", false, false},
	}
	for _, tc := range cases {
		newer, comparable := Newer(tc.current, tc.latest)
		if newer != tc.newer || comparable != tc.comparable {
			t.Errorf("Newer(%q,%q) = (%v,%v), want (%v,%v)",
				tc.current, tc.latest, newer, comparable, tc.newer, tc.comparable)
		}
	}
}

func TestParseChecksums(t *testing.T) {
	text := "abc123  skyfired-linux-amd64\n" +
		"def456  skyfired-windows-amd64.exe\n" +
		"\n" +
		"malformed-line\n"
	got := parseChecksums(text)
	if got["skyfired-linux-amd64"] != "abc123" {
		t.Errorf("linux checksum = %q", got["skyfired-linux-amd64"])
	}
	if got["skyfired-windows-amd64.exe"] != "def456" {
		t.Errorf("windows checksum = %q", got["skyfired-windows-amd64.exe"])
	}
	if len(got) != 2 {
		t.Errorf("expected 2 entries, got %d", len(got))
	}
}

func TestInstall(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "skyfired")
	if err := os.WriteFile(target, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Install(target, []byte("new-binary")); err != nil {
		t.Fatalf("Install: %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new-binary" {
		t.Fatalf("content = %q, want %q", got, "new-binary")
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(target)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0o111 == 0 {
			t.Fatalf("installed binary is not executable: %v", info.Mode())
		}
	}
	// No stray temp files left behind.
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("expected 1 file in dir, got %d", len(entries))
	}
}

func TestFetchVerifiesChecksum(t *testing.T) {
	binary := []byte("#!/bin/sh\necho hi\n")
	sum := sha256.Sum256(binary)
	sums := hex.EncodeToString(sum[:]) + "  skyfired-linux-amd64\n"

	mux := http.NewServeMux()
	mux.HandleFunc("/releases/download/v9.9.9/skyfired-linux-amd64", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(binary)
	})
	mux.HandleFunc("/releases/download/v9.9.9/checksums.txt", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(sums))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := New("owner/repo")
	c.GOOS, c.GOARCH = "linux", "amd64"
	rel := &Release{
		Tag: "v9.9.9",
		Assets: []Asset{
			{Name: "skyfired-linux-amd64", URL: srv.URL + "/releases/download/v9.9.9/skyfired-linux-amd64"},
			{Name: "checksums.txt", URL: srv.URL + "/releases/download/v9.9.9/checksums.txt"},
		},
	}

	got, err := c.Fetch(context.Background(), rel)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if string(got) != string(binary) {
		t.Fatalf("payload mismatch")
	}

	// Tampered checksum must be rejected.
	badSums := "00" + sums[2:]
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if filepath.Base(r.URL.Path) == "checksums.txt" {
			_, _ = w.Write([]byte(badSums))
			return
		}
		_, _ = w.Write(binary)
	}))
	defer srv2.Close()
	badRel := &Release{Tag: "v9.9.9", Assets: []Asset{
		{Name: "skyfired-linux-amd64", URL: srv2.URL + "/skyfired-linux-amd64"},
		{Name: "checksums.txt", URL: srv2.URL + "/checksums.txt"},
	}}
	if _, err := c.Fetch(context.Background(), badRel); err == nil {
		t.Fatal("expected checksum mismatch error")
	}
}

func TestLatestParsesRelease(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/releases/latest" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"tag_name":"v1.2.3","assets":[{"name":"skyfired-linux-amd64","browser_download_url":"http://x/y"}]}`))
	}))
	defer srv.Close()

	c := New("owner/repo")
	c.API = srv.URL
	rel, err := c.Latest(context.Background())
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if rel.Tag != "v1.2.3" {
		t.Fatalf("tag = %q", rel.Tag)
	}
	if a, ok := rel.find("skyfired-linux-amd64"); !ok || a.URL != "http://x/y" {
		t.Fatalf("asset not parsed: %+v", rel.Assets)
	}
	if _, err := c.ByTag(context.Background(), "v0.0.1"); err == nil {
		t.Fatal("ByTag should fail for unknown path")
	}
}
