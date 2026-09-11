package manager

import (
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
