package driver

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// renderUAPI serializes a Config into the userspace WireGuard UAPI text
// consumed by device.IpcSet. The userspace implementation expects keys in
// hex form and a replace_peers=true header (the manager always applies the
// full desired state).
func renderUAPI(cfg Config) (string, error) {
	keyHex := func(key string) (string, error) {
		if key == "" {
			return "", errors.New("empty base64 key")
		}
		k, err := wgtypes.ParseKey(key)
		if err != nil {
			return "", fmt.Errorf("invalid key: %w", err)
		}
		return hex.EncodeToString(k[:]), nil
	}

	var b strings.Builder
	if cfg.PrivateKey != "" {
		h, err := keyHex(cfg.PrivateKey)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "private_key=%s\n", h)
	}
	if cfg.ListenPort > 0 {
		fmt.Fprintf(&b, "listen_port=%d\n", cfg.ListenPort)
	}
	b.WriteString("replace_peers=true\n")

	for _, p := range cfg.Peers {
		pk, err := keyHex(p.PublicKey)
		if err != nil {
			return "", fmt.Errorf("peer public key: %w", err)
		}
		fmt.Fprintf(&b, "public_key=%s\n", pk)
		if p.PresharedKey != "" {
			psk, err := keyHex(p.PresharedKey)
			if err != nil {
				return "", fmt.Errorf("peer preshared key: %w", err)
			}
			fmt.Fprintf(&b, "preshared_key=%s\n", psk)
		}
		if p.Endpoint != "" {
			fmt.Fprintf(&b, "endpoint=%s\n", p.Endpoint)
		}
		fmt.Fprintf(&b, "persistent_keepalive_interval=%d\n", p.PersistentKeepalive)
		b.WriteString("replace_allowed_ips=true\n")
		for _, a := range p.AllowedIPs {
			fmt.Fprintf(&b, "allowed_ip=%s\n", a)
		}
	}
	return b.String(), nil
}
