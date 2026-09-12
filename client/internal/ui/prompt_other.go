//go:build !windows && !darwin

package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// PromptConnectionString reads the connection string from the terminal.
func PromptConnectionString(def string) (string, error) {
	fmt.Fprint(os.Stderr, "Skyfire connection string")
	if def != "" {
		fmt.Fprintf(os.Stderr, " [%s]", def)
	}
	fmt.Fprint(os.Stderr, ": ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return def, nil
	}
	return line, nil
}

// PromptWhitelist reads the domain/CIDR whitelist from the terminal.
func PromptWhitelist(def string) (string, error) {
	fmt.Fprint(os.Stderr, "Whitelist (comma separated; empty = full tunnel)")
	if def != "" {
		fmt.Fprintf(os.Stderr, " [%s]", def)
	}
	fmt.Fprint(os.Stderr, ": ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return def, nil
	}
	return line, nil
}
