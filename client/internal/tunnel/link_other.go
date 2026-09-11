//go:build !windows && !darwin && !linux

package tunnel

import "fmt"

func tunName() string { return "skyfire" }

func configureLink(string, *Conf) error {
	return fmt.Errorf("unsupported operating system")
}

func unconfigureLink(string, *Conf) {}
