//go:build !windows

package i18n

// detectOS has no additional source on non-Windows platforms; Detect already
// consulted the POSIX locale environment variables.
func detectOS() Lang { return Default }
