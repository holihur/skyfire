//go:build !linux

package driver

import "errors"

// NewKernel is unavailable outside Linux: wgctrl's kernel backend is netlink
// based and only builds for Linux. The userspace driver is the cross-platform
// alternative.
func NewKernel() (Driver, error) {
	return nil, errors.New("the kernel driver requires Linux (use -driver userspace)")
}
