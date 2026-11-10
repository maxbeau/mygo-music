package app

import (
	"github.com/egoist/mygo/ui"
	"mygo-music/internal/netpolicy"
	"runtime"
)

func (a *app) networkSettings(c *ui.Context, row settingsRow, block settingsBlock, p palette) {
	a.initializeUtilityDraft()
	d := &a.utility
	row(T("Proxy Mode"), "", func() {
		at := 0
		for i, s := range []string{"system", "direct", "custom"} {
			if d.network.Mode == s {
				at = i
			}
		}
		if i, ok := choice(c, "proxy-mode", T("Proxy Mode"), []string{"System / Environment", "Direct Connection", "Custom Proxy"}, at); ok {
			d.network.Mode = []string{"system", "direct", "custom"}[i]
		}
	})
	if d.network.Mode == "custom" {
		block(T("Proxy Address"), func() {
			ui.Column(c).Padding(12, 16).Gap(8).Children(func() {
				ui.Text(c, T("Proxy Address")).FontSize(13)
				ui.TextInput(c.Key("proxy-url"), &d.network.URL).Label(T("Proxy Address")).Placeholder("http://127.0.0.1:7890")
				if runtime.GOOS != "darwin" {
					ui.Text(c, T("Login uses the system proxy on this platform")).FontSize(12).TextColor(p.muted)
				}
				ui.Text(c, T("HTTP, HTTPS or SOCKS5; local playback connections bypass the proxy")).FontSize(12).TextColor(p.muted)
			})
		})
	}
	row(T("Save Network Settings"), "", func() {
		if ui.Button(c, T("Apply Network Settings")).Clicked() {
			valid, err := netpolicy.Validate(d.network)
			if err == nil {
				err = configureNetwork(valid)
			}
			if err != nil {
				d.err = T("Couldn't apply the proxy. Check the address and port.")
			} else {
				d.err = ""
				a.settings.Network = valid
				d.network = valid
				a.changed()
				a.pl.NetworkRestored()
			}
		}
	})
	block(T("Diagnostics"), func() {
		ui.Column(c).Padding(12, 16).Gap(10).Children(func() {
			ui.Row(c).Gap(10).Children(func() {
				if ui.Button(c, T("Run Connection Checks")).Disabled(d.busy).Clicked() {
					a.runDiagnostics()
				}
				if ui.Button(c, T("Export Diagnostic Report")).Clicked() {
					a.exportDiagnostics()
				}
				if ui.Button(c, T("Retry Playback")).Clicked() {
					a.retryCurrent()
				}
			})
			if d.busy {
				ui.Text(c, T("Checking…")).TextColor(p.muted)
			}
			for _, check := range d.checks {
				color := p.muted
				if !check.OK {
					color = p.accent
				}
				ui.Text(c, T(check.Name)+": "+T(check.Value)).FontSize(12).TextColor(color)
			}
			if d.err != "" {
				ui.Text(c, d.err).FontSize(12).TextColor(p.accent)
			}
		})
	})
}
func (a *app) shortcutSettings(c *ui.Context, row settingsRow, block settingsBlock, p palette) {
	a.initializeUtilityDraft()
	d := &a.utility
	for _, field := range []struct {
		name    string
		value   *string
		example string
	}{{"Play / Pause", &d.keys.PlayPause, "CmdOrCtrl+Alt+P"}, {"Next Song", &d.keys.Next, "CmdOrCtrl+Alt+N"}, {"Previous Song", &d.keys.Previous, "CmdOrCtrl+Alt+B"}, {"Show Player", &d.keys.Show, "CmdOrCtrl+Alt+M"}} {
		row(T(field.name), "", func() {
			ui.TextInput(c.Key(field.name), field.value).Width(240).Label(T(field.name)).Placeholder(field.example)
		})
	}
	row(T("Global Shortcuts"), T("Leave a field empty to disable its shortcut"), func() {
		if ui.Button(c, T("Save Shortcuts")).Clicked() {
			if err := a.applyShortcuts(d.keys); err != nil {
				a.toast = err.Error()
			} else {
				a.settings.Shortcuts = d.keys
				a.changed()
			}
		}
	})
}
