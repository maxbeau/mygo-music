package plugin

import (
	"encoding/json"
	"mygo-music/internal/i18n"
	"mygo-music/internal/music"
	"os"
	"path/filepath"
	"testing"
)

// Display translations never change the IDs sent to a plugin; an omitted filter uses its own default.
func TestDeclaredLyricSourceDispatch(t *testing.T) {
	dir := t.TempDir()
	manifest := `{"id":"custom","name":"Custom","lyricSources":[{"id":"archive-v2","name":"Archive"}]}`
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(manifest), 0600); err != nil {
		t.Fatal(err)
	}
	script := `export function searchLyrics({provider="plugin-default"}) {return [{id:provider,title:"Preview",provider,data:{text:provider}}]}`
	if err := os.WriteFile(filepath.Join(dir, "index.js"), []byte(script), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "locales"), 0700); err != nil {
		t.Fatal(err)
	}
	translations, _ := json.Marshal(map[string]string{"Archive": "Archive localized"})
	os.WriteFile(filepath.Join(dir, "locales", "zh-CN.json"), translations, 0600)
	p, err := Load(dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	defer i18n.SetLanguage(i18n.Language())
	i18n.SetLanguage(i18n.Chinese)
	info := p.Info()
	if len(info.LyricSources) != 1 || info.LyricSources[0].Name != "Archive localized" || info.LyricSources[0].ID != "archive-v2" {
		t.Fatal(info.LyricSources)
	}
	i18n.SetLanguage(i18n.English)
	if p.Info().LyricSources[0].Name != "Archive" {
		t.Fatal("localized manifest was mutated")
	}
	for _, filter := range []string{"archive-v2", ""} {
		rows, err := p.SearchLyrics(music.Track{ID: "song"}, "Title", "Artist", filter)
		want := filter
		if want == "" {
			want = "plugin-default"
		}
		if err != nil || len(rows) != 1 || rows[0].Data == nil || rows[0].Data.Text != want {
			t.Fatalf("filter %q: %v %v", filter, rows, err)
		}
	}
}

// Invalid IDs cannot create an ambiguous selector; an unnamed source uses its ID as display text.
func TestLyricSourceManifestValidation(t *testing.T) {
	for _, declaration := range []string{`[{"id":""}]`, `[{"id":"a"},{"id":"a"}]`, `[{"id":"a"}]`} {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(`{"id":"custom","lyricSources":`+declaration+`}`), 0600)
		os.WriteFile(filepath.Join(dir, "index.js"), []byte(`export function searchLyrics(){return []}`), 0600)
		p, err := Load(dir, t.TempDir())
		if declaration == `[{"id":"a"}]` {
			if err != nil {
				t.Fatal(err)
			}
			if p.Info().LyricSources[0].Name != "a" {
				t.Fatal("missing label fallback")
			}
			p.Close()
		} else if err == nil {
			p.Close()
			t.Fatal("invalid declaration accepted", declaration)
		}
	}
}
