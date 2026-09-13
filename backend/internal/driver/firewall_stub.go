//go:build !linux

package driver

// applyBlacklistFirewall is unsupported off Linux: those drivers rely on the
// OS network stack, and no cross-platform firewall backend is wired up.
func applyBlacklistFirewall(string, []string) error { return ErrBlacklistUnsupported }
