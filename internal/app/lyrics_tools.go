package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"mygo-music/internal/lyrics"
	"mygo-music/internal/music"
)

type lyricToolsState struct {
	open                               bool
	mode                               string
	track                              music.Track
	title, artist, provider, text, err string
	candidates                         []music.LyricCandidate
	preview                            *music.Lyrics
	previewID                          string
	busy                               bool
	list                               ui.ListState
	gen                                uint64
	active, hash, autoKey, romanBusy   string
	revision                           uint64
	translating                        bool
	cancel                             context.CancelFunc
	keyInput                           string
	keyBusy                            bool
}

func translationSecret(c lyrics.TranslationConfig) string {
	h := sha256.Sum256([]byte(c.Provider + "\x00" + c.Endpoint))
	return "lyrics-translation-" + hex.EncodeToString(h[:12])
}

var lyricSecretGet = func(c lyrics.TranslationConfig) (string, error) {
	b, err := mygo.Secrets.Get(translationSecret(c))
	return string(b), err
}
var lyricSecretSet = func(c lyrics.TranslationConfig, key string) error {
	return mygo.Secrets.Set(translationSecret(c), []byte(key))
}

func (a *app) lyricCurrent(track music.Track, hash string) bool {
	return sameTrack(a.pl.Current(), &track) && lyrics.ContentHash(a.pl.LyricsRecord().Data) == hash
}

func (a *app) ensureLyricTools(tr *music.Track) {
	d := &a.lyricTools
	record := a.pl.LyricsRecord()
	key := lyrics.TrackKey(*tr)
	hash := d.hash
	if d.active != key || d.revision != a.pl.LyricsRevision() {
		hash = lyrics.ContentHash(record.Data)
		d.revision = a.pl.LyricsRevision()
	}
	if d.active != key || d.hash != hash {
		if d.cancel != nil {
			d.cancel()
			d.cancel = nil
		}
		d.active, d.hash = key, hash
		d.translating = false
		d.romanBusy = ""
		d.autoKey = ""
		if d.open && !sameTrack(&d.track, tr) {
			d.open = false
			d.busy = false
		}
	}
	_, loaded := a.pl.Lyrics()
	if !loaded {
		return
	}
	if a.showRomanization(record) && record.RomanizedHash != hash && d.romanBusy != hash && len(lyrics.TextLines(record.Data)) > 0 {
		a.romanizeLyrics()
	}
	s := a.settings.Lyrics
	if s.AutoTranslate && s.KeyReady && len(lyrics.TextLines(record.Data)) > 0 {
		key := lyrics.TranslationHashKey(hash, s.Translation)
		if d.autoKey != key && !d.translating && record.Translations[key] == nil {
			d.autoKey = key
			a.translateLyrics(false)
		}
	}
}

func (a *app) showRomanization(r lyrics.Record) bool {
	if r.Romanize != nil {
		return *r.Romanize
	}
	return a.settings.Lyrics.Romanization
}

func (a *app) applyLyricRecord(r lyrics.Record) bool {
	if err := a.pl.ApplyLyrics(r); err != nil {
		a.toast = T("Couldn't save lyrics")
		return false
	}
	return true
}

func (a *app) translateLyrics(refresh bool) {
	tr := a.pl.Current()
	if tr == nil {
		return
	}
	r := a.pl.LyricsRecord()
	lines := lyrics.TextLines(r.Data)
	if len(lines) == 0 {
		return
	}
	cfg := a.settings.Lyrics.Translation
	key := lyrics.TranslationKey(r.Data, cfg)
	if !refresh && r.Translations[key] != nil {
		a.settings.Lyrics.HideTranslation = false
		a.changed()
		return
	}
	if !a.settings.Lyrics.KeyReady {
		a.settingsPage = "lyrics"
		a.openSettings()
		a.toast = T("Configure a translation service first")
		return
	}
	d := &a.lyricTools
	if d.cancel != nil {
		d.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	d.cancel = cancel
	d.translating = true
	track, hash := *tr, lyrics.ContentHash(r.Data)
	go func() {
		secret, err := lyricSecretGet(cfg)
		var translated []string
		if err == nil {
			translated, err = lyrics.Translate(ctx, nil, cfg, secret, lines)
		}
		a.post(func() {
			if ctx.Err() != nil || !a.lyricCurrent(track, hash) {
				return
			}
			d.translating = false
			d.cancel = nil
			cancel()
			if err != nil {
				a.toast = T("Translation failed. Check the service settings.")
				return
			}
			r := a.pl.LyricsRecord()
			if r.Translations == nil {
				r.Translations = map[string][]string{}
			}
			r.Translations[key] = translated
			a.applyLyricRecord(r)
			a.settings.Lyrics.HideTranslation = false
			a.changed()
		})
	}()
}

func (a *app) romanizeLyrics() {
	tr := a.pl.Current()
	if tr == nil {
		return
	}
	r := a.pl.LyricsRecord()
	hash := lyrics.ContentHash(r.Data)
	track := *tr
	a.lyricTools.romanBusy = hash
	lines := lyrics.TextLines(r.Data)
	go func() {
		readings, err := lyrics.Romanize(lines)
		a.post(func() {
			if !a.lyricCurrent(track, hash) {
				return
			}
			if err != nil {
				a.toast = T("Romanization is unavailable")
				return
			}
			r := a.pl.LyricsRecord()
			r.Romanized, r.RomanizedHash = readings, hash
			a.applyLyricRecord(r)
		})
	}()
}

func (a *app) lyricPosition() time.Duration {
	return a.pl.Position() - time.Duration(a.pl.LyricsRecord().OffsetMS)*time.Millisecond
}

func (a *app) lyricFontSize() int {
	size := a.settings.Lyrics.FontSize
	if size == 0 {
		return 30
	}
	return max(18, min(48, size))
}
func (a *app) lyricOffset(delta int64) {
	r := a.pl.LyricsRecord()
	r.OffsetMS = max(-60000, min(60000, r.OffsetMS+delta))
	a.applyLyricRecord(r)
}

func (a *app) lyricLines() []lyrics.Line {
	base, _ := a.pl.Lyrics()
	out := append([]lyrics.Line(nil), base...)
	r := a.pl.LyricsRecord()
	hash := a.lyricDocumentHash()
	translated := r.Translations[lyrics.TranslationHashKey(hash, a.settings.Lyrics.Translation)]
	for i := range out {
		if i < len(translated) {
			out[i].Translation = translated[i]
		}
		if a.settings.Lyrics.HideTranslation {
			out[i].Translation = ""
		}
		if a.showRomanization(r) && r.RomanizedHash == hash && i < len(r.Romanized) {
			out[i].Romanization = r.Romanized[i]
		}
	}
	return out
}

func (a *app) lyricPlainText() string {
	r := a.pl.LyricsRecord()
	original := lyrics.TextLines(r.Data)
	hash := a.lyricDocumentHash()
	translations := r.Translations[lyrics.TranslationHashKey(hash, a.settings.Lyrics.Translation)]
	var rows []string
	for i, text := range original {
		var sub []string
		if !a.settings.Lyrics.HideTranslation && i < len(translations) {
			sub = append(sub, translations[i])
		}
		if a.showRomanization(r) && r.RomanizedHash == hash && i < len(r.Romanized) && r.Romanized[i] != "" {
			sub = append(sub, r.Romanized[i])
		}
		if !a.settings.Lyrics.HideOriginal || len(sub) == 0 {
			sub = append([]string{text}, sub...)
		}
		rows = append(rows, strings.Join(sub, "\n"))
	}
	return strings.Join(rows, "\n\n")
}

func (a *app) openLyricTools(mode string) {
	tr := a.pl.Current()
	if tr == nil {
		return
	}
	d := &a.lyricTools
	d.gen++
	d.open = true
	d.mode = mode
	d.track = *tr
	d.title, d.artist = tr.Name, tr.ArtistNames()
	d.provider = "All"
	d.err = ""
	d.busy = false
	d.preview = nil
	d.candidates = nil
	d.list = ui.ListState{}
	r := a.pl.LyricsRecord()
	d.text = lyrics.Export(r.Data, true, 0)
	if d.text == "" {
		d.text = r.Data.Text
	}
}

func (a *app) searchLyricCandidates() {
	d := &a.lyricTools
	lib := a.libOf(d.track.Source)
	if lib == nil {
		return
	}
	src, ok := lib.src.(music.LyricsSearchSource)
	if !ok || !lib.can("searchLyrics") {
		d.err = T("This source doesn't support lyric search")
		return
	}
	d.gen++
	gen := d.gen
	d.busy = true
	d.preview = nil
	d.previewID = ""
	d.candidates = nil
	d.err = ""
	track, title, artist, provider := d.track, d.title, d.artist, d.provider
	go func() {
		candidates, err := src.SearchLyrics(track, title, artist, provider)
		a.post(func() {
			if d.gen != gen || !d.open || !sameTrack(a.pl.Current(), &track) {
				return
			}
			d.busy = false
			if err != nil {
				d.err = T("Couldn't search lyrics")
				return
			}
			d.candidates = candidates
			d.preview = nil
			if len(candidates) == 0 {
				d.err = T("No matching lyrics")
			}
		})
	}()
}

func (a *app) previewLyricCandidate(candidate music.LyricCandidate) {
	d := &a.lyricTools
	d.gen++
	d.busy = false
	d.previewID = candidate.ID
	d.preview = nil
	d.err = ""
	if candidate.Data != nil {
		copy := *candidate.Data
		d.preview = &copy
		return
	}
	lib := a.libOf(d.track.Source)
	if lib == nil {
		return
	}
	src, ok := lib.src.(music.LyricsSearchSource)
	if !ok {
		return
	}
	d.gen++
	gen := d.gen
	track := d.track
	d.busy = true
	go func() {
		data, err := src.LyricCandidate(candidate.ID)
		a.post(func() {
			if d.gen != gen || !d.open || !sameTrack(a.pl.Current(), &track) {
				return
			}
			d.busy = false
			if err != nil {
				d.err = T("Couldn't load lyrics")
				return
			}
			d.preview = data
		})
	}()
}

var openLyricFile = func(a *app) (string, error) {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.win, Title: T("Import Lyrics"), Filters: []mygo.FileFilter{{Name: "Lyrics", Extensions: []string{"lrc", "txt"}}}})
	if err != nil || len(paths) == 0 {
		return "", err
	}
	b, err := os.ReadFile(paths[0])
	if len(b) > 1<<20 {
		return "", fmt.Errorf("lyrics file too large")
	}
	return string(b), err
}
var saveLyricFile = func(a *app, name string) (string, error) {
	dir, _ := mygo.App.Path(mygo.PathDownloads)
	return mygo.Dialog.Save(mygo.SaveDialogOptions{Parent: a.win, Title: T("Export Lyrics"), DefaultPath: filepath.Join(dir, name), CreateDirectories: true})
}

func (a *app) importLyrics() {
	tr := a.pl.Current()
	if tr == nil {
		return
	}
	track := *tr
	go func() {
		text, err := openLyricFile(a)
		a.post(func() {
			if !sameTrack(a.pl.Current(), &track) {
				return
			}
			if err != nil {
				a.toast = T("Couldn't import lyrics")
				return
			}
			if text == "" {
				return
			}
			a.openLyricTools("edit")
			a.lyricTools.text = text
		})
	}()
}
func (a *app) exportLyrics(lrc bool) {
	tr := a.pl.Current()
	if tr == nil {
		return
	}
	r := a.pl.LyricsRecord()
	text := lyrics.Export(r.Data, lrc, r.OffsetMS)
	if text == "" {
		return
	}
	ext := "txt"
	if lrc {
		ext = "lrc"
	}
	name := songFileName(tr, ext)
	go func() {
		path, err := saveLyricFile(a, name)
		if err == nil && path != "" {
			err = os.WriteFile(path, []byte(text), 0o600)
		}
		if err != nil {
			a.post(func() { a.toast = T("Couldn't export lyrics") })
		} else if path != "" {
			a.post(func() { a.toast = T("Lyrics exported") })
		}
	}()
}

func (a *app) lyricDocumentHash() string {
	if tr := a.pl.Current(); tr != nil && a.lyricTools.active == lyrics.TrackKey(*tr) && a.lyricTools.revision == a.pl.LyricsRevision() {
		return a.lyricTools.hash
	}
	return lyrics.ContentHash(a.pl.LyricsRecord().Data)
}
