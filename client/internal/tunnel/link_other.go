//go:build !windows && !darwin && !linux

package tunnel

import (
	"fmt"
	"net/netip"
)

func tunName() string { return "skyfire" }

func configureLink(string, *Conf) error {
	return fmt.Errorf("unsupported operating system")
}

func unconfigureLink(string, *Conf) {}

func configureDNS(string, []string) error { return nil }

func unconfigureDNS(string, []string) {}

func addTunnelPrefix(string, netip.Prefix) error {
	return fmt.Errorf("unsupported operating system")
}

func delTunnelPrefix(string, netip.Prefix) {}

func currentDNSServers() []string { return nil }
