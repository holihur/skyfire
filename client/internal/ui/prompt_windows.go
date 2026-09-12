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

// PromptWhitelist opens a native Windows input box for the domain/CIDR
// whitelist (comma separated; empty clears it).
func PromptWhitelist(def string) (string, error) {
	script := "Add-Type -AssemblyName Microsoft.VisualBasic; " +
		"[Microsoft.VisualBasic.Interaction]::InputBox(" +
		"'Domains or CIDRs routed through the tunnel (comma separated). Use *.example.com for subdomains and 10.0.0.0/8 for CIDR. Empty = full tunnel.'," +
		"'Skyfire - domain whitelist'," +
		"'" + strings.ReplaceAll(def, "'", "''") + "')"
	out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
