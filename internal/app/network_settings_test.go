package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"mygo-music/internal/music"
)

// The listener can turn off the system proxy from Network and the choice
// is kept when the app opens again.
func TestTurnOffSystemProxyFromSettings(t *testing.T) {
	a := newTestApp()
	a.settings.path = filepath.Join(t.TempDir(), "settings.json")
	t.Cleanup(func() { music.SetFollowSystemProxy(true) })
	tt := ui.NewTester(a.settingsView, 780, 800)
	settled(tt)
	if err := tt.Click("Network"); err != nil {
		t.Fatal(err)
	}
	settled(tt)
	title, titleOK := tt.Find("Follow System Proxy")
	desc, descOK := tt.Find("Use the system proxy for catalogs, covers and audio, starting with the next request")
	if !titleOK || !descOK {
		t.Fatalf("proxy setting not found: %q", tt.Texts())
	}
	tt.ClickAt(780-28-16-12, (title.Y+desc.Y+desc.H)/2)
	tt.Frame()
	b, err := os.ReadFile(a.settings.path)
	if err != nil {
		t.Fatal(err)
	}
	s := defaultSettings()
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	if a.settings.FollowSystemProxy || s.FollowSystemProxy {
		t.Fatal("turning off the system proxy was not applied and saved")
	}
}
