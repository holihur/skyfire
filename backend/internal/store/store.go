package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// DefaultMTU is used when no MTU is configured.
const DefaultMTU = 1420

// Load reads the configuration file. A missing file results in an empty
// default configuration (no error).
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Config{Version: 1}, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	if cfg.Version == 0 {
		cfg.Version = 1
	}
	for _, iface := range cfg.Interfaces {
		for _, p := range iface.Peers {
			if len(p.ClientRoutes) == 0 {
				p.ClientRoutes = DefaultRoutes()
			}
		}
	}
	return &cfg, nil
}

// Save writes the configuration atomically.
func Save(path string, cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("mkdir config dir: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o640); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("commit config: %w", err)
	}
	return nil
}

// DefaultRoutes are the routes pushed to fresh clients.
func DefaultRoutes() []string {
	return []string{"0.0.0.0/0", "::/0"}
}
