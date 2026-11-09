package app

import (
	"encoding/json"
	"github.com/egoist/mygo/ui"
	"mygo-music/internal/lyrics"
	"mygo-music/internal/music"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func lyricToolsApp(t *testing.T) (*app, *ui.Tester) {
	t.Helper()
	a := newTestApp()
	a.pl.Cue(sourced(song("seed", "Song", "Singer")))
	a.pl.LyricsStore = &lyrics.Store{Dir: t.TempDir()}
	if !a.applyLyricRecord(lyrics.Record{Data: lyrics.ParseDocument("[00:10.00]First line\n[00:20.00]Second line")}) {
		t.Fatal("save")
	}
	a.pl.Hold(12 * time.Second)
	a.nowPlaying, a.npPane = true, paneLyrics
	tt := ui.NewTester(a.view, 1180, 780)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Move(600, 400)
	settled(tt)
	return a, tt
}
func chooseLyrics(t *testing.T, tt *ui.Tester, item string) {
	t.Helper()
	if err := tt.Click("Lyrics Options"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem(item); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
}

// Applying edited lyrics persists the document without interrupting the audio or queue.
func TestLyricToolsEditKeepsPlayback(t *testing.T) {
	a, tt := lyricToolsApp(t)
	pos, queue := a.pl.Position(), a.pl.QueueVersion()
	chooseLyrics(t, tt, "Edit Lyrics…")
	a.lyricTools.text = "[00:05.00]Edited line"
	tt.Frame()
	if err := tt.Click("Apply Lyrics"); err != nil {
		t.Fatal(err)
	}
	r := a.pl.LyricsStore.Read(*a.pl.Current())
	if !r.Manual || r.Data.LRC != "[00:05.00]Edited line" || a.lyricTools.open {
		t.Fatalf("edit not persisted %+v", r)
	}
	if a.pl.Position() != pos || a.pl.QueueVersion() != queue || !a.pl.Playing() {
		t.Fatal("edit changed playback")
	}
}

// Timing controls persist per song and clicking a shifted line seeks to its displayed audio time.
func TestLyricToolsTimingSeek(t *testing.T) {
	a, tt := lyricToolsApp(t)
	chooseLyrics(t, tt, "Adjust Lyric Timing…")
	if err := tt.Click("Later 500 ms"); err != nil {
		t.Fatal(err)
	}
	if a.pl.LyricsStore.Read(*a.pl.Current()).OffsetMS != 500 || a.lyricPosition() != 11500*time.Millisecond {
		t.Fatal("offset lost")
	}
	if err := tt.Click("Cancel"); err != nil {
		t.Fatal(err)
	}
	settled(tt)
	if err := tt.Click("First line"); err != nil {
		t.Fatal(err)
	}
	if a.pl.Position() != 10500*time.Millisecond {
		t.Fatalf("seek %v", a.pl.Position())
	}
}

type lyricSearchFake struct{ *fakeSource }

func (s *lyricSearchFake) SearchLyrics(track music.Track, title, artist, provider string) ([]music.LyricCandidate, error) {
	return []music.LyricCandidate{{ID: "pick", Provider: "LRCLIB", Title: "Alternate", Artist: "Singer", Duration: 100000, Data: &music.Lyrics{LRC: "[00:02.00]Selected", Provider: "LRCLIB"}}}, nil
}
func (s *lyricSearchFake) LyricCandidate(string) (*music.Lyrics, error) {
	return nil, music.ErrUnsupported
}

// Search only changes lyrics after preview and explicit application, leaving audio intact.
func TestLyricToolsSearchPreview(t *testing.T) {
	a, tt := lyricToolsApp(t)
	ch := posts(a)
	a.libs[0].src = &lyricSearchFake{fake(a)}
	chooseLyrics(t, tt, "Search Lyric Sources…")
	if err := tt.Click("Search"); err != nil {
		t.Fatal(err)
	}
	until(t, ch, func() bool { return !a.lyricTools.busy })
	tt.Frame()
	if err := tt.Click("LRCLIB: Alternate"); err != nil {
		t.Fatal(err)
	}
	if a.pl.LyricsRecord().Manual {
		t.Fatal("preview applied early")
	}
	tt.Frame()
	if err := tt.Click("Apply Lyrics"); err != nil {
		t.Fatal(err)
	}
	if r := a.pl.LyricsRecord(); !r.Manual || r.Data.Provider != "LRCLIB" || r.Data.LRC != "[00:02.00]Selected" {
		t.Fatal(r)
	}
	if a.pl.Position() != 12*time.Second {
		t.Fatal("preview changed playback")
	}
}

// Cached translation and reading overlays stay aligned and are invalidated by a document edit.
func TestLyricToolsDerivedText(t *testing.T) {
	a, _ := lyricToolsApp(t)
	r := a.pl.LyricsRecord()
	on := true
	r.Romanize = &on
	r.RomanizedHash = lyrics.ContentHash(r.Data)
	r.Romanized = []string{"first", "second"}
	r.Translations = map[string][]string{lyrics.TranslationKey(r.Data, a.settings.Lyrics.Translation): {"一", "二"}}
	a.applyLyricRecord(r)
	a.settings.Lyrics.HideOriginal = true
	lines := a.lyricLines()
	if lines[0].Translation != "一" || lines[1].Romanization != "second" {
		t.Fatal(lines)
	}
	r.Data = lyrics.ParseDocument("[00:01.00]Changed")
	a.applyLyricRecord(r)
	if got := a.lyricLines(); got[0].Translation != "" || got[0].Romanization != "" {
		t.Fatal("stale annotation", got)
	}
}

// Frame artifacts cover the triple-line display, candidate preview, editor, timing and settings.
func TestLyricToolsFrames(t *testing.T) {
	dir := framesDir(t)
	a, tt := lyricToolsApp(t)
	r := a.pl.LyricsRecord()
	on := true
	r.Romanize = &on
	r.RomanizedHash = lyrics.ContentHash(r.Data)
	r.Romanized = []string{"first line", "second line"}
	r.Translations = map[string][]string{lyrics.TranslationKey(r.Data, a.settings.Lyrics.Translation): {"第一句歌词", "第二句歌词"}}
	a.applyLyricRecord(r)
	a.settings.Lyrics.Alignment = "center"
	settled(tt)
	saveFrame(t, dir, "lyric-tools-display.png", tt.Image())
	a.libs[0].src = &lyricSearchFake{fake(a)}
	fakeSource := a.libs[0].src.(*lyricSearchFake).fakeSource
	fakeSource.info.LyricSources = []music.LyricSource{{ID: "custom-archive", Name: "Archive"}}
	a.openLyricTools("search")
	a.lyricTools.candidates = []music.LyricCandidate{{ID: "preview", Provider: "LRCLIB", Title: "Song", Artist: "Singer", Data: &r.Data}}
	a.previewLyricCandidate(a.lyricTools.candidates[0])
	settled(tt)
	saveFrame(t, dir, "lyric-tools-search.png", tt.Image())
	for _, mode := range []string{"edit", "timing"} {
		a.openLyricTools(mode)
		settled(tt)
		saveFrame(t, dir, "lyric-tools-"+mode+".png", tt.Image())
	}
	a.lyricTools.open = false
	a.settingsPage = "lyrics"
	st := ui.NewTester(a.settingsView, 780, 760)
	settled(st)
	saveFrame(t, dir, "lyric-tools-settings.png", st.Image())
}

// A completed translation queued before a track change cannot annotate the new song.
func TestLyricTranslationRejectsOldSong(t *testing.T) {
	a, _ := lyricToolsApp(t)
	ch := posts(a)
	previous := lyricSecretGet
	lyricSecretGet = func(lyrics.TranslationConfig) (string, error) { return "test-only", nil }
	defer func() { lyricSecretGet = previous }()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": `[{"id":0,"text":"一"},{"id":1,"text":"二"}]`}}}})
	}))
	defer srv.Close()
	a.settings.Lyrics.Translation = lyrics.TranslationConfig{Endpoint: srv.URL, Model: "test", Target: "zh-CN"}
	a.settings.Lyrics.KeyReady = true
	a.translateLyrics(false)
	// The worker has finished; only applying its result is delayed.
	var callback func()
	select {
	case callback = <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("translation did not complete")
	}
	a.pl.Cue(sourced(song("new", "New Song")))
	a.applyLyricRecord(lyrics.Record{Data: lyrics.ParseDocument("New words")})
	a.ensureLyricTools(a.pl.Current())
	callback()
	if len(a.pl.LyricsRecord().Translations) != 0 || a.lyricTools.translating {
		t.Fatal("old translation reached new song")
	}
}

// Import previews before saving; plain text remains untimed and exports/copies without changing audio.
func TestLyricToolsImportExport(t *testing.T) {
	a, tt := lyricToolsApp(t)
	ch := posts(a)
	previousOpen, previousSave, previousCopy := openLyricFile, saveLyricFile, copyText
	defer func() { openLyricFile = previousOpen; saveLyricFile = previousSave; copyText = previousCopy }()
	path := filepath.Join(t.TempDir(), "lyrics.txt")
	openLyricFile = func(*app) (string, error) { return "Imported words\nSecond line", nil }
	saveLyricFile = func(*app, string) (string, error) { return path, nil }
	copied := ""
	copyText = func(text string) error { copied = text; return nil }
	chooseLyrics(t, tt, "Import Lyrics…")
	runPost(t, ch)
	tt.Frame()
	if a.pl.LyricsRecord().Manual {
		t.Fatal("import applied before preview")
	}
	if err := tt.Click("Apply Lyrics"); err != nil {
		t.Fatal(err)
	}
	chooseLyrics(t, tt, "Copy Lyrics")
	if copied != "Imported words\nSecond line" {
		t.Fatal("copy", copied)
	}
	chooseLyrics(t, tt, "Export Text…")
	runPost(t, ch)
	b, err := os.ReadFile(path)
	if err != nil || string(b) != copied {
		t.Fatalf("export %q %v", b, err)
	}
	if lines, _ := a.pl.Lyrics(); len(lines) != 0 || a.pl.Position() != 12*time.Second {
		t.Fatal("import invented timings")
	}
}

type declaredLyricSearch struct {
	*fakeSource
	requests chan string
	search   func(string) []music.LyricCandidate
}

func (s *declaredLyricSearch) SearchLyrics(track music.Track, title, artist, provider string) ([]music.LyricCandidate, error) {
	s.requests <- provider
	if s.search != nil {
		return s.search(provider), nil
	}
	return []music.LyricCandidate{{ID: provider, Title: "Candidate", Provider: provider, Data: &music.Lyrics{Text: "Preview"}}}, nil
}
func (s *declaredLyricSearch) LyricCandidate(string) (*music.Lyrics, error) {
	return nil, music.ErrUnsupported
}

// Selecting a plugin-defined source sends its opaque ID, including when another library is active.
func TestLyricSourcesFollowPlayingSource(t *testing.T) {
	a, tt := lyricToolsApp(t)
	ch := posts(a)
	source := &declaredLyricSearch{fakeSource: fake(a), requests: make(chan string, 2)}
	source.info.LyricSources = []music.LyricSource{{ID: "custom-archive", Name: "Archive"}, {ID: "other-archive", Name: "Archive"}}
	a.libs[0].src = source
	a.library = addFake(a, newFakeSource("another-library"))
	chooseLyrics(t, tt, "Search Lyric Sources…")
	if err := tt.Click("Lyrics Source"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Archive (other-archive)"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Search"); err != nil {
		t.Fatal(err)
	}
	until(t, ch, func() bool { return !a.lyricTools.busy })
	if got := <-source.requests; got != "other-archive" {
		t.Fatalf("sent %q, wanted plugin ID", got)
	}
	tt.Frame()
	if err := tt.Click("Lyrics Source"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("All"); err != nil {
		t.Fatal(err)
	}
	if a.lyricTools.preview != nil || len(a.lyricTools.candidates) != 0 {
		t.Fatal("previous filter preview survived")
	}
	if err := tt.Click("Search"); err != nil {
		t.Fatal(err)
	}
	until(t, ch, func() bool { return !a.lyricTools.busy })
	if got := <-source.requests; got != "" {
		t.Fatalf("All sent a provider-specific ID: %q", got)
	}
}

// A legacy searchable source without declarations still searches with its own default.
func TestLyricSourcesLegacyDefault(t *testing.T) {
	a, tt := lyricToolsApp(t)
	ch := posts(a)
	source := &declaredLyricSearch{fakeSource: fake(a), requests: make(chan string, 1)}
	a.libs[0].src = source
	chooseLyrics(t, tt, "Search Lyric Sources…")
	if err := tt.Click("Lyrics Source"); err == nil {
		t.Fatal("undeclared selector shown")
	}
	if err := tt.Click("Search"); err != nil {
		t.Fatal(err)
	}
	until(t, ch, func() bool { return !a.lyricTools.busy })
	if got := <-source.requests; got != "" {
		t.Fatalf("legacy source got %q", got)
	}
}

// Sources that cannot search offer no search action while editing remains available.
func TestLyricSourcesUnsupported(t *testing.T) {
	a, tt := lyricToolsApp(t)
	if err := tt.Click("Lyrics Options"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Search Lyric Sources…"); err == nil {
		t.Fatal("unsupported action offered")
	}
	if err := tt.ChooseMenuItem("Edit Lyrics…"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.lyricTools.open || a.lyricTools.mode != "edit" {
		t.Fatal("generic editor unavailable")
	}
}

// A request started under All cannot populate the list after the listener selects another filter.
func TestLyricSourcesRejectPreviousFilter(t *testing.T) {
	a, tt := lyricToolsApp(t)
	ch := posts(a)
	release := make(chan bool)
	source := &declaredLyricSearch{fakeSource: fake(a), requests: make(chan string, 2)}
	source.info.LyricSources = []music.LyricSource{{ID: "new-filter", Name: "Archive"}}
	source.search = func(provider string) []music.LyricCandidate {
		<-release
		return []music.LyricCandidate{{ID: "pick", Provider: provider, Title: "Candidate", Data: &music.Lyrics{Text: "Preview"}}}
	}
	a.libs[0].src = source
	chooseLyrics(t, tt, "Search Lyric Sources…")
	if err := tt.Click("Search"); err != nil {
		t.Fatal(err)
	}
	if got := wait(t, source.requests, "search"); got != "" {
		t.Fatal(got)
	}
	if err := tt.Click("Lyrics Source"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Archive"); err != nil {
		t.Fatal(err)
	}
	close(release)
	runPost(t, ch)
	if a.lyricTools.busy || len(a.lyricTools.candidates) != 0 || a.lyricTools.provider != "new-filter" {
		t.Fatal("old result reached new filter")
	}
	if err := tt.Click("Search"); err != nil {
		t.Fatal(err)
	}
	until(t, ch, func() bool { return !a.lyricTools.busy })
	if got := wait(t, source.requests, "filtered search"); got != "new-filter" {
		t.Fatal(got)
	}
	if len(a.lyricTools.candidates) != 1 || a.lyricTools.candidates[0].Provider != "new-filter" {
		t.Fatal("new filter did not search")
	}
}
