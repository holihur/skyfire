package manager

import (
	"crypto/rand"
	"encoding/hex"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// genKeys generates a fresh WireGuard keypair.
func genKeys() (priv, pub string, err error) {
	k, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return "", "", err
	}
	return k.String(), k.PublicKey().String(), nil
}

// genPreshared generates an optional preshared key.
func genPreshared() (string, error) {
	k, err := wgtypes.GenerateKey()
	if err != nil {
		return "", err
	}
	return k.String(), nil
}

// genClientToken generates a 32-byte random hex token used by the desktop
// client to fetch its config without logging in.
func genClientToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
