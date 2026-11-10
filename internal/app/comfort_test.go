package app

import (
	"errors"
	"github.com/egoist/mygo/ui"
	"mygo-music/internal/diagnostics"
	"mygo-music/internal/lyrics"
	"mygo-music/internal/music"
	"mygo-music/internal/netpolicy"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Invalid network drafts are rejected without changing playback or the persisted proxy policy.
func TestNetworkSettingsApply(t *testing.T) {
	a := newTestApp()
	a.settingsPage = "network"
	a.initializeUtilityDraft()
	previous := configureNetwork
	defer func() { configureNetwork = previous }()
	applied := netpolicy.Config{}
	calls := 0
	configureNetwork = func(c netpolicy.Config) error { applied = c; calls++; return nil }
	a.utility.network = netpolicy.Config{Mode: "custom", URL: "http://user:secret@localhost:80"}
	tt := ui.NewTester(a.settingsView, 900, 850)
	settled(tt)
	if err := tt.Click("Apply Network Settings"); err != nil {
		t.Fatal(err)
	}
	if calls != 0 || a.settings.Network.Mode != "" || a.utility.err == "" {
		t.Fatal("invalid proxy applied")
	}
	a.utility.network.URL = "127.0.0.1:7890"
	tt.Frame()
	if err := tt.Click("Apply Network Settings"); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || applied.URL != "http://127.0.0.1:7890" || a.settings.Network != applied || a.utility.err != "" {
		t.Fatal("valid policy not applied", applied, a.settings.Network)
	}
}

// Conflicting global hotkeys roll back the entire edit and retain working previous bindings.
func TestGlobalShortcutRollback(t *testing.T) {
	a := newTestApp()
	oldRegister, oldUnregister := registerHotkey, unregisterHotkey
	defer func() { registerHotkey, unregisterHotkey = oldRegister, oldUnregister }()
	active := map[string]func(){}
	registerHotkey = func(key string, fn func()) error {
		if key == "Ctrl+Alt+X" {
			return errors.New("occupied")
		}
		active[key] = fn
		return nil
	}
	unregisterHotkey = func(key string) { delete(active, key) }
	original := shortcutPreferences{PlayPause: "Ctrl+Alt+P", Next: "Ctrl+Alt+N"}
	if err := a.applyShortcuts(original); err != nil {
		t.Fatal(err)
	}
	if err := a.applyShortcuts(shortcutPreferences{PlayPause: "Ctrl+Alt+Q", Next: "Ctrl+Alt+X"}); err == nil {
		t.Fatal("conflict accepted")
	}
	if len(active) != 2 || active[original.PlayPause] == nil || active[original.Next] == nil || a.utility.applied != original {
		t.Fatal("old shortcuts not restored")
	}
	a.settingsPage = "shortcuts"
	a.initializeUtilityDraft()
	a.utility.keys = shortcutPreferences{Show: "Ctrl+Alt+S"}
	tt := ui.NewTester(a.settingsView, 900, 800)
	settled(tt)
	if err := tt.Click("Save Shortcuts"); err != nil {
		t.Fatal(err)
	}
	if a.settings.Shortcuts.Show != "Ctrl+Alt+S" || len(active) != 1 {
		t.Fatal("UI shortcut save failed")
	}
}

// Bracket keys calibrate lyrics without seeking; an open editor keeps those keys as text input.
func TestLyricTimingShortcuts(t *testing.T) {
	a, tt := lyricToolsApp(t)
	position := a.pl.Position()
	tt.Key(0, ui.KeyBracketRight)
	if a.pl.LyricsRecord().OffsetMS != 500 || a.pl.Position() != position {
		t.Fatal("shortcut sought audio")
	}
	a.openLyricTools("edit")
	tt.Frame()
	tt.Key(0, ui.KeyBracketLeft)
	if a.pl.LyricsRecord().OffsetMS != 500 {
		t.Fatal("editor changed timing")
	}
}

type checkingSource struct{ *fakeSource }

func (s *checkingSource) Diagnostics() ([]music.DiagnosticCheck, error) {
	return []music.DiagnosticCheck{{Name: "Dependency", Value: "ready", OK: true}}, nil
}

// Diagnostic export includes check results but excludes registered secrets from the report.
func TestDiagnosticChecksAndExport(t *testing.T) {
	a := newTestApp()
	ch := posts(a)
	a.libs[0].src = &checkingSource{fake(a)}
	a.settingsPage = "network"
	path := filepath.Join(t.TempDir(), "report.json")
	previous := saveDiagnosticFile
	defer func() { saveDiagnosticFile = previous }()
	saveDiagnosticFile = func(*app) (string, error) { return path, nil }
	tt := ui.NewTester(a.settingsView, 1100, 900)
	settled(tt)
	if err := tt.Click("Run Connection Checks"); err != nil {
		t.Fatal(err)
	}
	until(t, ch, func() bool { return !a.utility.busy })
	tt.Frame()
	diagnostics.Remember("test-only-secret")
	a.utility.checks = append(a.utility.checks, music.DiagnosticCheck{Name: "Context", Value: "test-only-secret"})
	if err := tt.Click("Export Diagnostic Report"); err != nil {
		t.Fatal(err)
	}
	runPost(t, ch)
	b, err := os.ReadFile(path)
	if err != nil || strings.Contains(string(b), "test-only-secret") || !strings.Contains(string(b), "Dependency") {
		t.Fatalf("report %s %v", b, err)
	}
}

type translatingSource struct {
	*fakeSource
	data chan *music.Lyrics
}

func (s *translatingSource) LyricsTranslation(track music.Track, language string) (*music.Lyrics, error) {
	return <-s.data, nil
}

// A delayed provider translation cannot overwrite a manual selection made while it was loading.
func TestSourceTranslationCannotReplaceEdit(t *testing.T) {
	a, _ := lyricToolsApp(t)
	ch := posts(a)
	source := &translatingSource{fakeSource: fake(a), data: make(chan *music.Lyrics, 1)}
	a.libs[0].src = source
	source.caps["lyricsTranslation"] = true
	a.settings.Lyrics.SourceTranslation = true
	a.lyricTools.sourceAsked = ""
	a.ensureLyricTools(a.pl.Current())
	r := a.pl.LyricsRecord()
	r.Data = lyrics.ParseDocument("[00:01.00]User edit")
	r.Manual = true
	a.applyLyricRecord(r)
	source.data <- &music.Lyrics{LRC: "[00:10.00]First line\n[00:20.00]Second line", Translation: "[00:10.00]First translation\n[00:20.00]Second translation"}
	runPost(t, ch)
	if a.pl.LyricsRecord().Data.LRC != "[00:01.00]User edit" || !a.pl.LyricsRecord().Manual {
		t.Fatal("late source result replaced edit")
	}
}

// Frame artifacts inspect network settings, diagnostic controls and global shortcut fields.
func TestComfortFrames(t *testing.T) {
	dir := framesDir(t)
	a := newTestApp()
	a.settingsPage = "network"
	a.initializeUtilityDraft()
	a.utility.network = netpolicy.Config{Mode: "custom", URL: "http://127.0.0.1:7890"}
	a.utility.checks = []music.DiagnosticCheck{{Name: "YouTube Connection", Value: "HTTP 200", OK: true}, {Name: "ffmpeg", Value: "ffmpeg 8.0", OK: true}}
	tt := ui.NewTester(a.settingsView, 1000, 820)
	settled(tt)
	saveFrame(t, dir, "comfort-network.png", tt.Image())
	a.settingsPage = "shortcuts"
	settled(tt)
	saveFrame(t, dir, "comfort-shortcuts.png", tt.Image())
	a.settingsPage = "lyrics"
	settled(tt)
	saveFrame(t, dir, "comfort-lyrics.png", tt.Image())
}

// Changing the target language hides a cached provider translation for a different language.
func TestSourceTranslationTargetChange(t *testing.T) {
	a, _ := lyricToolsApp(t)
	r := a.pl.LyricsRecord()
	r.Data.Translation = "[00:10.00]Translated first\n[00:20.00]Translated second"
	r.SourceTranslationLanguage = "zh-CN"
	a.applyLyricRecord(r)
	a.settings.Lyrics.Translation.Target = "zh-CN"
	if a.lyricLines()[0].Translation == "" {
		t.Fatal("matching translation hidden")
	}
	a.settings.Lyrics.Translation.Target = "en"
	if a.lyricLines()[0].Translation != "" {
		t.Fatal("old language still displayed")
	}
}
