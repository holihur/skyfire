package totp

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Config is the persisted TOTP state. Confirmed is false while an
// authenticator is being enrolled (the secret exists but the user has not yet
// proven possession by entering a valid code).
type Config struct {
	Secret    string `json:"secret"`
	Confirmed bool   `json:"confirmed"`
}

// Bound reports whether an authenticator has been enrolled and confirmed.
func (c Config) Bound() bool { return c.Secret != "" && c.Confirmed }

// Load reads the persisted TOTP configuration from path. A missing file
// yields an empty Config and no error.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, nil
		}
		return Config{}, fmt.Errorf("read totp file: %w", err)
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}, fmt.Errorf("parse totp file: %w", err)
	}
	return c, nil
}

// Save writes the TOTP configuration to path with owner-only permissions.
func Save(path string, c Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write totp file: %w", err)
	}
	return os.Rename(tmp, path)
}

// Clear removes the persisted configuration, forcing re-enrollment on the next
// login. A missing file is not an error.
func Clear(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
