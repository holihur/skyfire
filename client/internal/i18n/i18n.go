// Package i18n holds the desktop client's translation catalogue. The client is
// dependency-free by design, so this is a tiny string table rather than a
// general-purpose localisation framework.
package i18n

import (
	"fmt"
	"os"
	"strings"
	"sync"
)

// Lang is a supported interface language.
type Lang string

const (
	// EN is English.
	EN Lang = "en"
	// ZH is Simplified Chinese.
	ZH Lang = "zh"
	// Default is used when parsing or detection is inconclusive.
	Default = EN
)

var (
	mu      sync.RWMutex
	current = Default
)

// Parse normalises a locale code ("zh-CN", "zh_CN", "en-US", "en", "C") to a
// supported Lang, falling back to Default.
func Parse(code string) Lang {
	c := strings.ToLower(strings.TrimSpace(code))
	c = strings.ReplaceAll(c, "_", "-")
	switch {
	case strings.HasPrefix(c, "zh"):
		return ZH
	case strings.HasPrefix(c, "en"):
		return EN
	default:
		return Default
	}
}

// Supported lists the languages the client ships, in menu order.
func Supported() []Lang { return []Lang{EN, ZH} }

// Name returns the language's own name (endonym), used for the menu label.
func Name(l Lang) string {
	switch l {
	case ZH:
		return "中文"
	default:
		return "English"
	}
}

// Set changes the active language.
func Set(l Lang) {
	mu.Lock()
	current = l
	mu.Unlock()
}

// Current returns the active language.
func Current() Lang {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

// Detect guesses the language from the environment: an explicit SKYFIRE_LANG
// first, then the standard POSIX locale variables, and finally the OS setting
// (used on Windows, where the POSIX variables are usually unset).
func Detect() Lang {
	for _, k := range []string{"SKYFIRE_LANG", "LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return Parse(v)
		}
	}
	return detectOS()
}

// T returns the translation for key, formatted with args when provided. Missing
// keys fall back to English and then to the key itself.
func T(key string, args ...any) string {
	mu.RLock()
	l := current
	mu.RUnlock()
	s := catalog[l][key]
	if s == "" {
		s = catalog[Default][key]
	}
	if s == "" {
		return key
	}
	if len(args) > 0 {
		return fmt.Sprintf(s, args...)
	}
	return s
}
