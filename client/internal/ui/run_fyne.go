//go:build cgo && (windows || darwin)

package ui

import (
	"log/slog"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"github.com/holihur/skyfire/client/internal/app"
	"github.com/holihur/skyfire/client/internal/config"
	"github.com/holihur/skyfire/client/internal/i18n"
)

// trayRefreshInterval is how often the tray re-reads transfer counters.
const trayRefreshInterval = 2 * time.Second

// Run starts the fyne-based tray application (system tray + dialogs) and blocks
// on the main goroutine until the user quits. Requires cgo.
func Run(c Controller, log *slog.Logger) error {
	fa := fyneapp.New()
	desk, _ := fa.(desktop.App)

	// The window exists only to host modal dialogs. fyne quits when the last
	// window is *destroyed*, so we always Hide (never Close) it.
	win := fa.NewWindow("Skyfire")
	win.SetContent(widget.NewLabel(""))
	win.Resize(fyne.NewSize(560, 240))
	win.Hide()

	var (
		mu     sync.Mutex
		cur    = app.Disconnected
		curMsg string
	)

	mStatus := fyne.NewMenuItem(i18n.T("status.disconnected"), nil)
	mStatus.Disabled = true
	mTraffic := fyne.NewMenuItem(i18n.T("tray.trafficNone"), nil)
	mTraffic.Disabled = true
	mLatency := fyne.NewMenuItem(i18n.T("tray.latencyNone"), nil)
	mLatency.Disabled = true
	mToggle := fyne.NewMenuItem(i18n.T("tray.toggle"), nil)
	mSet := fyne.NewMenuItem(i18n.T("tray.setConnect"), nil)
	mWhite := fyne.NewMenuItem(i18n.T("tray.setWhitelist"), nil)
	mQuit := fyne.NewMenuItem(i18n.T("tray.quit"), nil)
	mEn := fyne.NewMenuItem(i18n.Name(i18n.EN), nil)
	mZh := fyne.NewMenuItem(i18n.Name(i18n.ZH), nil)
	mLang := fyne.NewMenuItem(i18n.T("tray.language"), nil)
	mLang.ChildMenu = fyne.NewMenu("", mEn, mZh)

	menu := fyne.NewMenu("Skyfire",
		mStatus, mTraffic, mLatency,
		fyne.NewMenuItemSeparator(),
		mToggle, mSet, mWhite, mLang,
		fyne.NewMenuItemSeparator(),
		mQuit,
	)

	// refresh runs on the fyne goroutine only; it recomputes labels and pushes
	// them to the tray, avoiding redundant redraws when nothing changed.
	var (
		lastStatus = app.Status(-1)
		lastLabels = ""
	)
	refresh := func() {
		mu.Lock()
		s, msg := cur, curMsg
		mu.Unlock()

		if desk != nil && s != lastStatus {
			icon := fyne.NewStaticResource("skyfire.png", dotPNG(statusColor(s)))
			desk.SetSystemTrayIcon(icon)
			lastStatus = s
		}

		mStatus.Label = statusLabel(s)
		if msg != "" {
			mStatus.Label += " \u2014 " + msg
		}
		traffic := i18n.T("tray.trafficNone")
		if s == app.Connected {
			if tr := c.Traffic(); tr.Active {
				traffic = i18n.T("tray.trafficValue", fmtBytes(tr.Rx), fmtBytes(tr.Tx))
			}
		}
		mTraffic.Label = traffic
		lat := i18n.T("tray.latencyNone")
		if s == app.Connected {
			if d, ok := c.Latency(); ok {
				lat = i18n.T("tray.latencyValue", d.Milliseconds())
			}
		}
		mLatency.Label = lat
		if s == app.Connected || s == app.Connecting {
			mToggle.Label = i18n.T("tray.toggleOn")
		} else {
			mToggle.Label = i18n.T("tray.toggle")
		}
		mSet.Label = i18n.T("tray.setConnect")
		mWhite.Label = i18n.T("tray.setWhitelist")
		mLang.Label = i18n.T("tray.language")
		mQuit.Label = i18n.T("tray.quit")
		mEn.Checked = i18n.Current() == i18n.EN
		mZh.Checked = i18n.Current() == i18n.ZH

		key := strings.Join([]string{mStatus.Label, mTraffic.Label, mLatency.Label, mToggle.Label}, "\x00")
		if key != lastLabels {
			menu.Refresh()
			lastLabels = key
		}
	}

	showForm := func(title, label, def string, onOK func(string)) {
		entry := widget.NewEntry()
		entry.SetText(def)
		items := []*widget.FormItem{widget.NewFormItem(label, entry)}
		d := dialog.NewForm(title, i18n.T("common.ok"), i18n.T("common.cancel"), items, func(ok bool) {
			if !ok {
				win.Hide()
				return
			}
			onOK(strings.TrimSpace(entry.Text))
		}, win)
		d.Resize(fyne.NewSize(560, 200))
		win.Show()
		d.Show()
	}

	showError := func(err error) {
		d := dialog.NewError(err, win)
		d.SetOnClosed(func() { win.Hide() })
		win.Show()
		d.Show()
	}

	askConnectionString := func() {
		showForm(i18n.T("prompt.connect.title"), i18n.T("prompt.connect.label"), c.ConnectString(), func(v string) {
			if v == "" {
				win.Hide()
				return
			}
			if err := c.SetConnectString(v); err != nil {
				log.Warn("invalid connection string", "error", err)
				showError(err)
				return
			}
			log.Info("connection string saved")
			win.Hide()
			go func() { _ = c.Connect() }()
		})
	}

	askWhitelist := func() {
		showForm(i18n.T("prompt.whitelist.title"), i18n.T("prompt.whitelist.label"), strings.Join(c.Whitelist(), ", "), func(v string) {
			list := config.ParseWhitelist(v)
			if err := c.SetWhitelist(list); err != nil {
				log.Warn("invalid whitelist", "error", err)
				showError(err)
				return
			}
			if len(list) == 0 {
				log.Info("whitelist cleared (full tunnel)")
			} else {
				log.Info("whitelist saved", "entries", list)
			}
			win.Hide()
		})
	}

	setLang := func(l i18n.Lang) {
		if err := c.SetLang(string(l)); err != nil {
			log.Warn("set language", "error", err)
			return
		}
		refresh()
	}

	mToggle.Action = func() {
		go func() {
			if err := c.Toggle(); err != nil {
				log.Warn("toggle tunnel", "error", err)
			}
		}()
	}
	mSet.Action = func() { askConnectionString() }
	mWhite.Action = func() { askWhitelist() }
	mEn.Action = func() { setLang(i18n.EN) }
	mZh.Action = func() { setLang(i18n.ZH) }
	mQuit.Action = func() {
		_ = c.Disconnect()
		fa.Quit()
	}

	if desk != nil {
		desk.SetSystemTrayMenu(menu)
		desk.SetSystemTrayIcon(fyne.NewStaticResource("skyfire.png", dotPNG(statusColor(app.Disconnected))))
	}

	c.SetOnChange(func(s app.Status, msg string) {
		mu.Lock()
		cur, curMsg = s, msg
		mu.Unlock()
		fyne.Do(refresh)
	})

	fa.Lifecycle().SetOnStarted(func() {
		{
			s, msg := c.Status()
			mu.Lock()
			cur, curMsg = s, msg
			mu.Unlock()
		}
		refresh()

		go func() {
			t := time.NewTicker(trayRefreshInterval)
			defer t.Stop()
			for range t.C {
				fyne.Do(refresh)
			}
		}()

		if c.ConnectString() == "" {
			askConnectionString()
		} else if c.AutoConnect() {
			go func() {
				if err := c.Connect(); err != nil {
					log.Warn("auto connect", "error", err)
				}
			}()
		}
	})

	fa.Run()
	return nil
}
