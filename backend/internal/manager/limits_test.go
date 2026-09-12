package manager

import (
	"testing"

	"skyfire/internal/store"
)

func TestValidateLimits(t *testing.T) {
	if err := validateLimits(0, 0); err != nil {
		t.Fatalf("0/0 must be allowed: %v", err)
	}
	if err := validateLimits(8_000_000, 2_000_000); err != nil {
		t.Fatalf("positive limits must be allowed: %v", err)
	}
	if err := validateLimits(-1, 0); err == nil {
		t.Fatal("negative download limit must be rejected")
	}
	if err := validateLimits(0, -1); err == nil {
		t.Fatal("negative upload limit must be rejected")
	}
}

func TestShapingSpecs(t *testing.T) {
	iface := &store.Interface{
		Name: "wg0",
		Peers: []*store.Peer{
			{ // unlimited, no limits → skipped
				PublicKey:  "a",
				Address:    "10.42.0.2",
				AllowedIPs: []string{"10.42.0.2/32"},
				Enabled:    true,
			},
			{ // limited via assigned address
				PublicKey:     "b",
				Address:       "10.42.0.3",
				Enabled:       true,
				DownloadLimit: 8_000_000,
				UploadLimit:   2_000_000,
			},
			{ // limited but disabled → skipped
				PublicKey:     "c",
				Address:       "10.42.0.4",
				AllowedIPs:    []string{"10.42.0.4/32"},
				Enabled:       false,
				DownloadLimit: 1_000_000,
			},
			{ // limited via AllowedIPs
				PublicKey:     "d",
				Address:       "10.42.0.5",
				AllowedIPs:    []string{"10.42.0.5/32", "10.99.0.0/16"},
				Enabled:       true,
				DownloadLimit: 5_000_000,
			},
		},
	}
	specs := shapingSpecs(iface)
	if len(specs) != 2 {
		t.Fatalf("want 2 shaping specs, got %d", len(specs))
	}

	// peer "b" falls back to its assigned address as /32.
	b := specs[0]
	if len(b.Prefixes) != 1 || b.Prefixes[0] != "10.42.0.3/32" {
		t.Fatalf("peer b prefixes: %v", b.Prefixes)
	}
	if b.DownloadLimit != 8_000_000 || b.UploadLimit != 2_000_000 {
		t.Fatalf("peer b limits: dl=%d ul=%d", b.DownloadLimit, b.UploadLimit)
	}

	// peer "d" carries its AllowedIPs.
	d := specs[1]
	if len(d.Prefixes) != 2 {
		t.Fatalf("peer d prefixes: %v", d.Prefixes)
	}
	if d.UploadLimit != 0 {
		t.Fatalf("peer d upload should be 0 (unlimited), got %d", d.UploadLimit)
	}
}
