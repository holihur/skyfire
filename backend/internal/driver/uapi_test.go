package driver

import (
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func TestRenderUAPI(t *testing.T) {
	priv := mustKey(t)
	pub := mustKey(t)
	psk := mustKey(t)

	cfg := Config{
		PrivateKey: priv,
		ListenPort: 51820,
		Peers: []PeerSpec{{
			PublicKey:           pub,
			PresharedKey:        psk,
			Endpoint:            "vpn.example.com:51820",
			PersistentKeepalive: 25,
			AllowedIPs:          []string{"10.42.0.2/32"},
		}},
	}
	uapi, err := renderUAPI(cfg)
	if err != nil {
		t.Fatalf("renderUAPI: %v", err)
	}

	want := []string{
		"private_key=" + hex.EncodeToString(mustBytes(t, priv)),
		"listen_port=51820",
		"replace_peers=true",
		"public_key=" + hex.EncodeToString(mustBytes(t, pub)),
		"preshared_key=" + hex.EncodeToString(mustBytes(t, psk)),
		"endpoint=vpn.example.com:51820",
		"persistent_keepalive_interval=25",
		"replace_allowed_ips=true",
		"allowed_ip=10.42.0.2/32",
	}
	for _, w := range want {
		if !strings.Contains(uapi, w) {
			t.Fatalf("uapi missing %q:\n%s", w, uapi)
		}
	}
}

func TestRenderUAPIRejectsBadKey(t *testing.T) {
	_, err := renderUAPI(Config{PrivateKey: "not-a-key"})
	if err == nil {
		t.Fatal("expected error for invalid key")
	}
}

func mustKey(t *testing.T) string {
	t.Helper()
	k, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return k.String()
}

func mustBytes(t *testing.T, s string) []byte {
	t.Helper()
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		t.Fatalf("decode %q: %v", s, err)
	}
	return b
}
