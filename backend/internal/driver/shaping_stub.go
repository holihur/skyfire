//go:build !linux

package driver

// Rate limiting relies on Linux tc(8) traffic control, which is unavailable
// on this platform for the OS-stack drivers.

func applyTCShaping(dev string, peers []PeerShaping) error {
	return ErrShapingUnsupported
}

func removeTCShaping(dev string) {}
