package app

import (
	"encoding/json"
	"errors"
	"github.com/egoist/mygo"
	"mygo-music/internal/diagnostics"
	"mygo-music/internal/music"
	"mygo-music/internal/netpolicy"
	"os"
	"runtime"
	"strings"
	"time"
)

type shortcutPreferences struct {
	PlayPause string `json:"playPause,omitempty"`
	Next      string `json:"next,omitempty"`
	Previous  string `json:"previous,omitempty"`
	Show      string `json:"show,omitempty"`
}
type utilityState struct {
	initialized bool
	network     netpolicy.Config
	keys        shortcutPreferences
	registered  []string
	applied     shortcutPreferences
	busy        bool
	checks      []music.DiagnosticCheck
	err         string
}

var registerHotkey = mygo.GlobalShortcut.Register
var unregisterHotkey = mygo.GlobalShortcut.Unregister
var configureNetwork = netpolicy.Configure

func (a *app) initializeUtilityDraft() {
	if a.utility.initialized {
		return
	}
	a.utility.initialized = true
	a.utility.network = a.settings.Network
	a.utility.keys = a.settings.Shortcuts
}
func (a *app) shortcutActions(s shortcutPreferences) []struct {
	key string
	fn  func()
} {
	return []struct {
		key string
		fn  func()
	}{
		{s.PlayPause, func() { a.pl.TogglePlay(); a.redraw() }}, {s.Next, func() { a.pl.Next(); a.redraw() }}, {s.Previous, func() { a.pl.Prev(); a.redraw() }}, {s.Show, func() {
			if a.win != nil {
				a.win.Show()
				a.win.Focus()
			}
		}},
	}
}
func (a *app) applyShortcuts(s shortcutPreferences) error {
	seen := map[string]bool{}
	for _, v := range a.shortcutActions(s) {
		if v.key == "" {
			continue
		}
		if !strings.Contains(v.key, "+") || seen[strings.ToLower(v.key)] {
			return errors.New(T("Use distinct shortcuts with a modifier key"))
		}
		seen[strings.ToLower(v.key)] = true
	}
	for _, key := range a.utility.registered {
		unregisterHotkey(key)
	}
	a.utility.registered = nil
	for _, v := range a.shortcutActions(s) {
		if v.key == "" {
			continue
		}
		if err := registerHotkey(v.key, v.fn); err != nil {
			for _, key := range a.utility.registered {
				unregisterHotkey(key)
			}
			a.utility.registered = nil
			for _, old := range a.shortcutActions(a.utility.applied) {
				if old.key != "" && registerHotkey(old.key, old.fn) == nil {
					a.utility.registered = append(a.utility.registered, old.key)
				}
			}
			return errors.New(T("Couldn't register a shortcut. It may already be in use."))
		}
		a.utility.registered = append(a.utility.registered, v.key)
	}
	a.utility.applied = s
	return nil
}
func (a *app) runDiagnostics() {
	if a.utility.busy {
		return
	}
	a.utility.busy = true
	a.utility.err = ""
	a.utility.checks = nil
	lib := a.library
	go func() {
		checks := []music.DiagnosticCheck{{Name: "Operating System", Value: runtime.GOOS + " / " + runtime.GOARCH, OK: true}}
		if lib != nil {
			if src, ok := lib.src.(music.DiagnosticSource); ok && lib.can("diagnostics") {
				list, err := src.Diagnostics()
				if err != nil {
					checks = append(checks, music.DiagnosticCheck{Name: "Source", Value: T("Checks failed. See the diagnostic log."), OK: false})
				} else {
					checks = append(checks, list...)
				}
			}
		}
		a.post(func() { a.utility.busy = false; a.utility.checks = checks })
	}()
}
func (a *app) exportDiagnostics() {
	values := struct {
		Version, Platform, Time string
		Network                 netpolicy.Config
		Checks                  []music.DiagnosticCheck
		Log                     string
	}{mygo.App.Version(), runtime.GOOS + "/" + runtime.GOARCH, time.Now().UTC().Format(time.RFC3339), a.settings.Network, append([]music.DiagnosticCheck(nil), a.utility.checks...), diagnostics.Tail()}
	b, _ := json.MarshalIndent(values, "", "  ")
	text := diagnostics.Redact(string(b))
	go func() {
		path, err := saveDiagnosticFile(a)
		if err == nil && path != "" {
			err = os.WriteFile(path, []byte(text), 0600)
		}
		if err != nil {
			a.post(func() { a.toast = T("Couldn't export the diagnostic report") })
		} else if path != "" {
			a.post(func() { a.toast = T("Diagnostic report exported") })
		}
	}()
}

var saveDiagnosticFile = func(a *app) (string, error) {
	return mygo.Dialog.Save(mygo.SaveDialogOptions{Parent: a.settingsWin, Title: T("Export Diagnostic Report"), DefaultPath: "MyGo-diagnostics.json"})
}

func (a *app) retryCurrent() {
	if a.pl.Recovering() {
		a.pl.NetworkRestored()
	} else if a.pl.Current() != nil {
		a.pl.RetryPlayback()
	}
}
