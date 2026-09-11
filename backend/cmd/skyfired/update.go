package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"skyfire/internal/update"
)

// runUpdate implements `skyfire update` / `skyfired update`. It returns a
// process exit code.
func runUpdate(args []string) int {
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	var (
		check   = fs.Bool("check", false, "only check whether a newer release exists")
		tag     = fs.String("version", "", "release tag to install (default: latest)")
		repo    = fs.String("repo", update.DefaultRepo, "GitHub repository (owner/name)")
		apiBase = fs.String("api", update.DefaultAPI, "GitHub API base URL")
		force   = fs.Bool("force", false, "install even when the current version is not older")
		restart = fs.Bool("restart", true, "restart the skyfire systemd service after updating")
	)
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), "Usage: skyfire update [flags]\n\nUpdate skyfired to the latest GitHub release.\n\n")
		fs.PrintDefaults()
	}
	_ = fs.Parse(args)

	client := update.New(*repo)
	client.API = *apiBase
	client.Token = os.Getenv("GITHUB_TOKEN")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	rel, err := client.Resolve(ctx, *tag)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	name, err := client.AssetName()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	newer, comparable := update.Newer(version, rel.Tag)
	if comparable {
		if !newer && !*force {
			fmt.Printf("skyfire is up to date (%s)\n", version)
			return 0
		}
	} else {
		fmt.Printf("current version %q is not a release version; latest is %s\n", version, rel.Tag)
	}

	fmt.Printf("latest release: %s (asset %s)\n", rel.Tag, name)
	if *check {
		fmt.Println("run again without -check to install it")
		return 0
	}

	data, err := client.Fetch(ctx, rel)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	target, err := executablePath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	if err := update.Install(target, data); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		if runtime.GOOS != "windows" {
			fmt.Fprintln(os.Stderr, "hint: try again with sudo (need write access to", filepath.Dir(target)+")")
		}
		return 1
	}
	fmt.Printf("installed %s to %s\n", rel.Tag, target)

	if *restart {
		restartService()
	}
	return 0
}

// executablePath resolves the real path of the running binary, following the
// `skyfire` -> `skyfired` symlink so the actual file is replaced.
func executablePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		return resolved, nil
	}
	return exe, nil
}

// restartService restarts the skyfire systemd unit when it is active. Failures
// are reported but not fatal: the new binary is already installed.
func restartService() {
	if runtime.GOOS != "linux" {
		return
	}
	if _, err := exec.LookPath("systemctl"); err != nil {
		return
	}
	if err := exec.Command("systemctl", "is-active", "--quiet", "skyfire").Run(); err != nil {
		return // not running as a systemd service
	}
	fmt.Println("restarting skyfire service…")
	if err := exec.Command("systemctl", "restart", "--no-block", "skyfire").Run(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not restart the service: %v\n", err)
		fmt.Fprintln(os.Stderr, "run manually: sudo systemctl restart skyfire")
	}
}
