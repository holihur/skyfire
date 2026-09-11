//go:build windows

package ui

import (
	"os/exec"
	"strings"
)

// PromptConnectionString opens a native Windows input box.
func PromptConnectionString(def string) (string, error) {
	script := "Add-Type -AssemblyName Microsoft.VisualBasic; " +
		"[Microsoft.VisualBasic.Interaction]::InputBox(" +
		"'Paste the Skyfire connection string (peer detail page in the console).'," +
		"'Skyfire'," +
		"'" + strings.ReplaceAll(def, "'", "''") + "')"
	out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
