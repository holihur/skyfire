//go:build windows || (darwin && cgo)

package ui

import (
	"log/slog"

	"fyne.io/systray"

	"github.com/holihur/skyfire/client/internal/app"
)

// Run shows a system tray icon and blocks until the user quits.
func Run(c Controller, log *slog.Logger) error {
	systray.Run(func() { onReady(c, log) }, func() {})
	return nil
}

func onReady(c Controller, log *slog.Logger) {
	systray.SetTitle("Skyfire")

	mStatus := systray.AddMenuItem("Disconnected", "Tunnel status")
	mStatus.Disable()
	systray.AddSeparator()
	mToggle := systray.AddMenuItem("Connect", "Connect or disconnect the tunnel")
	mSet := systray.AddMenuItem("Set connection string…", "Paste the connection string from the Skyfire console")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "Quit Skyfire")

	apply := func(s app.Status, msg string) {
		systray.SetIcon(iconFor(s))
		tip := "Skyfire: " + s.String()
		if msg != "" {
			tip += " — " + msg
		}
		systray.SetTooltip(tip)
		mStatus.SetTitle(s.String())
		if s == app.Connected || s == app.Connecting {
			mToggle.SetTitle("Disconnect")
		} else {
			mToggle.SetTitle("Connect")
		}
	}
	c.SetOnChange(apply)
	{
		s, msg := c.Status()
		apply(s, msg)
	}

	// First run: ask for the connection string and connect right away.
	if c.ConnectString() == "" {
		go func() {
			if err := askConnectionString(c, log); err != nil {
				log.Warn("connection string", "error", err)
				return
			}
			if c.ConnectString() != "" {
				_ = c.Connect()
			}
		}()
	} else if c.AutoConnect() {
		// Launched with -connect: bring the tunnel up without a second click.
		go func() {
			if err := c.Connect(); err != nil {
				log.Warn("auto connect", "error", err)
			}
		}()
	}

	go func() {
		for {
			select {
			case <-mToggle.ClickedCh:
				if err := c.Toggle(); err != nil {
					log.Warn("toggle tunnel", "error", err)
				}
			case <-mSet.ClickedCh:
				_ = askConnectionString(c, log)
			case <-mQuit.ClickedCh:
				_ = c.Disconnect()
				systray.Quit()
				return
			}
		}
	}()
}

func askConnectionString(c Controller, log *slog.Logger) error {
	v, err := PromptConnectionString(c.ConnectString())
	if err != nil {
		return err
	}
	if v == "" {
		return nil
	}
	if err := c.SetConnectString(v); err != nil {
		log.Warn("invalid connection string", "error", err)
		return err
	}
	log.Info("connection string saved")
	return nil
}
