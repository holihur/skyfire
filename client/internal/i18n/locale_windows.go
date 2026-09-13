//go:build windows

package i18n

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32                     = windows.NewLazySystemDLL("kernel32.dll")
	procGetUserDefaultLocaleName = kernel32.NewProc("GetUserDefaultLocaleName")
)

// detectOS returns the language for the current Windows user's locale.
func detectOS() Lang {
	const localeNameMaxLength = 85 // LOCALE_NAME_MAX_LENGTH
	buf := make([]uint16, localeNameMaxLength)
	n, _, _ := procGetUserDefaultLocaleName.Call(
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
	)
	if n == 0 {
		return Default
	}
	return Parse(windows.UTF16ToString(buf))
}
