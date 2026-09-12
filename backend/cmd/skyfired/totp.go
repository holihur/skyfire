package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/skip2/go-qrcode"

	"skyfire/internal/totp"
)

// runTOTPCmd implements `skyfired totp [status|reset]`. Enrollment itself
// happens on first login in the web UI; this command inspects or clears the
// binding (the recovery path for a lost authenticator).
func runTOTPCmd(args []string) int {
	fs := flag.NewFlagSet("totp", flag.ContinueOnError)
	configPath := fs.String("config", "", "path to the persisted config (used to locate the TOTP file)")
	username := fs.String("username", "admin", "account name shown in the authenticator app")
	fs.Usage = func() {
		out := fs.Output()
		fmt.Fprint(out, "Usage: skyfired totp [status|reset] [flags]\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	path := *configPath
	if path == "" {
		path = resolveConfigPath()
	}
	totpPath := totpFilePath(path)

	switch fs.Arg(0) {
	case "", "status":
		return totpStatus(totpPath, *username)
	case "reset":
		if err := totp.Clear(totpPath); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		fmt.Println("TOTP binding cleared. The next login will require re-enrollment.")
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown totp action %q (want status | reset)\n", fs.Arg(0))
		return 2
	}
}

func totpStatus(path, account string) int {
	cfg, err := totp.Load(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	switch {
	case cfg.Secret == "":
		fmt.Println("TOTP: disabled (no secret configured)")
	case !cfg.Confirmed:
		fmt.Println("TOTP: enrollment pending (the secret is shown on the next login)")
		fmt.Println("Secret:", cfg.Secret)
		printTOTPQR(cfg.Secret, account)
	default:
		fmt.Println("TOTP: enabled (authenticator bound)")
		fmt.Println("Secret:", cfg.Secret)
		printTOTPQR(cfg.Secret, account)
	}
	return 0
}

func printTOTPQR(secret, account string) {
	uri := totp.ProvisioningURI(secret, account, "Skyfire")
	fmt.Println("URI:   ", uri)
	if code, err := totp.Code(secret); err == nil {
		fmt.Println("Current code:", code)
	}
	if q, err := qrcode.New(uri, qrcode.Medium); err == nil {
		fmt.Println(q.ToSmallString(false))
	}
}

// totpFilePath stores the TOTP state alongside the daemon configuration.
func totpFilePath(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), "totp.json")
}
