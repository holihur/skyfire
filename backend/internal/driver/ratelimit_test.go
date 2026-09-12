package driver

import (
	"testing"
)

func TestNewByteLimiter(t *testing.T) {
	if l := newByteLimiter(0); l != nil {
		t.Fatal("0 bps must yield no limiter")
	}
	if l := newByteLimiter(-5); l != nil {
		t.Fatal("negative bps must yield no limiter")
	}
	l := newByteLimiter(8_000_000) // 1 MB/s
	if l == nil {
		t.Fatal("positive bps must yield a limiter")
	}
	if l.Burst() < 64*1024 {
		t.Fatalf("burst must fit a max UDP datagram, got %d", l.Burst())
	}
}

func TestBuildNSShaping(t *testing.T) {
	peers := []PeerShaping{
		{ // no limit → skipped
			Prefixes: []string{"10.0.0.2/32"},
		},
		{ // download only
			Prefixes:      []string{"10.0.0.3/32"},
			DownloadLimit: 8_000_000,
		},
		{ // invalid prefix → skipped
			Prefixes:      []string{"not-a-prefix"},
			DownloadLimit: 1_000_000,
		},
		{ // upload only, two prefixes
			Prefixes:    []string{"10.0.0.4/32", "10.1.0.0/16"},
			UploadLimit: 2_000_000,
		},
	}
	out := buildNSShaping(peers)
	if len(out) != 2 {
		t.Fatalf("want 2 shaping entries, got %d", len(out))
	}
	if out[0].download != 8_000_000 || out[0].upload != 0 {
		t.Fatalf("first entry limits wrong: dl=%d ul=%d", out[0].download, out[0].upload)
	}
	if out[1].download != 0 || out[1].upload != 2_000_000 || len(out[1].prefixes) != 2 {
		t.Fatalf("second entry wrong: dl=%d ul=%d prefixes=%d", out[1].download, out[1].upload, len(out[1].prefixes))
	}
}
