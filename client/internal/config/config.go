// Package config stores the desktop client's connection string and caches the
// WireGuard configuration it fetches from the Skyfire server.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// State is the persisted client state.
type State struct {
	// Connect is the full token-scoped config URL, e.g.
	// https://vpn.example.com:51821/api/p/<token>/wg.conf
	Connect string `json:"connect,omitempty"`
}

// ErrNoConnect reports a missing connection string.
var ErrNoConnect = errors.New("no connection string configured")

// Store persists the client state in the user configuration directory.
type Store struct {
	path string
	mu   sync.Mutex
	st   State
}

// Open loads (or initializes) the client state.
func Open() (*Store, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	s := &Store{path: filepath.Join(dir, "config.json")}
	data, err := os.ReadFile(s.path)
	switch {
	case err == nil:
		if err := json.Unmarshal(data, &s.st); err != nil {
			return nil, fmt.Errorf("parse %s: %w", s.path, err)
		}
	case errors.Is(err, os.ErrNotExist):
		// first run
	default:
		return nil, err
	}
	return s, nil
}

// Dir returns the per-user Skyfire client directory.
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "skyfire-client"), nil
}

// Connect returns the configured connection string (may be empty).
func (s *Store) Connect() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.st.Connect
}

// SetConnect validates and persists a connection string.
func (s *Store) SetConnect(conn string) error {
	if err := ValidateConnect(conn); err != nil {
		return err
	}
	s.mu.Lock()
	s.st.Connect = strings.TrimSpace(conn)
	st := s.st
	s.mu.Unlock()
	return s.save(st)
}

func (s *Store) save(st State) error {
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return os.Rename(tmp, s.path)
}

// ValidateConnect checks that conn looks like a Skyfire peer config URL.
func ValidateConnect(conn string) error {
	conn = strings.TrimSpace(conn)
	if conn == "" {
		return ErrNoConnect
	}
	u, err := url.Parse(conn)
	if err != nil {
		return fmt.Errorf("invalid connection string: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("connection string must start with http:// or https://")
	}
	if u.Host == "" {
		return errors.New("connection string has no host")
	}
	if !strings.Contains(u.Path, "/api/p/") {
		return errors.New("connection string is not a Skyfire peer config URL")
	}
	return nil
}

// CachePath returns where the fetched configuration is cached.
func CachePath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "wg.conf"), nil
}

// Fetch downloads the WireGuard configuration from conn and caches it on
// disk so the client can still start when the server is briefly unreachable.
func Fetch(conn string, timeout time.Duration) (string, error) {
	if err := ValidateConnect(conn); err != nil {
		return "", err
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(conn)
	if err != nil {
		return "", fmt.Errorf("fetch config: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("read config: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch config: server returned %s", resp.Status)
	}
	text := string(body)
	if !strings.Contains(text, "[Interface]") {
		return "", errors.New("fetch config: response is not a WireGuard configuration")
	}
	if p, err := CachePath(); err == nil {
		_ = os.MkdirAll(filepath.Dir(p), 0o700)
		_ = os.WriteFile(p, body, 0o600)
	}
	return text, nil
}

// LoadCached returns the last successfully fetched configuration, if any.
func LoadCached() (string, error) {
	p, err := CachePath()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
