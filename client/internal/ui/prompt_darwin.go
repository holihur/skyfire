//go:build darwin

package ui

import (
	"os/exec"
	"strings"
)

// PromptConnectionString opens a native macOS input dialog.
func PromptConnectionString(def string) (string, error) {
	script := `text returned of (display dialog "Paste the Skyfire connection string (peer detail page in the console)." default answer "` +
		escapeAppleScript(def) + `" with title "Skyfire" buttons {"Cancel", "OK"} default button "OK")`
	out, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func escapeAppleScript(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return strings.ReplaceAll(s, `"`, `\"`)
}
