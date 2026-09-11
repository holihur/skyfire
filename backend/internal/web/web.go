// Package web embeds the compiled React frontend into the binary so the
// daemon is a single self-contained executable. `dist/` is written by the
// frontend build step (see the Makefile).
package web

import "embed"

// Dist holds the built frontend. A placeholder index.html is committed so the
// backend always compiles; the real build overwrites it.
//
//go:embed dist
var Dist embed.FS
