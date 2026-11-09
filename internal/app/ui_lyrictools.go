package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"mygo-music/internal/lyrics"
)

func (a *app) lyricOptionsButton(c *ui.Context) {
	if source := a.pl.LyricsRecord().Data.Provider; source != "" {
		ui.Text(c, T(source)).FontSize(11).TextColor(white.Alpha(0.5)).SingleLine()
	}
	if a.lyricTools.translating {
		ui.Text(c, T("Translating…")).FontSize(11).TextColor(white.Alpha(0.6))
	}
	iconButton(c.Key("lyric-options"), iconMore, 18, white.Alpha(0.7), T("Lyrics Options")).Menu(func(m *ui.Menu) {
		if m.Item(T("Search Lyric Sources…")).Chosen() {
			a.openLyricTools("search")
		}
		if m.Item(T("Edit Lyrics…")).Chosen() {
			a.openLyricTools("edit")
		}
		if m.Item(T("Import Lyrics…")).Chosen() {
			a.importLyrics()
		}
		if m.Item(T("Copy Lyrics")).Chosen() {
			copyText(lyrics.Export(a.pl.LyricsRecord().Data, false, 0))
		}
		if m.Item(T("Export LRC…")).Disabled(len(a.lyricLines()) == 0).Chosen() {
			a.exportLyrics(true)
		}
		if m.Item(T("Export Text…")).Chosen() {
			a.exportLyrics(false)
		}
		m.Separator()
		if m.Item(T("Translate Lyrics")).Chosen() {
			a.translateLyrics(false)
		}
		if m.Item(T("Translate Again")).Chosen() {
			a.translateLyrics(true)
		}
		r := a.pl.LyricsRecord()
		on := a.showRomanization(r)
		if m.Item(T("Romanize This Song")).Checked(on).Chosen() {
			on = !on
			r.Romanize = &on
			a.applyLyricRecord(r)
			if on {
				a.romanizeLyrics()
			}
		}
		if m.Item(T("Adjust Lyric Timing…")).Chosen() {
			a.openLyricTools("timing")
		}
		m.Separator()
		if m.Item(T("Refresh Lyrics")).Chosen() {
			a.pl.RetryLyrics()
		}
		if m.Item(T("Restore Automatic Lyrics")).Chosen() {
			r := a.pl.LyricsRecord()
			r.Manual = false
			r.Data = lyrics.ParseDocument("")
			r.Data.Provider = ""
			a.applyLyricRecord(r)
			a.pl.RetryLyrics()
		}
		if m.Item(T("Lyrics Settings…")).Chosen() {
			a.settingsPage = "lyrics"
			a.openSettings()
		}
	})
}

func (a *app) lyricToolsDialog(c *ui.Context) {
	d := &a.lyricTools
	if !d.open {
		return
	}
	p := a.pal
	if !sameTrack(a.pl.Current(), &d.track) {
		d.open = false
		return
	}
	ui.DialogBase(c.Key("lyric-tools"), &d.open, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0.4))
		panel.Width(720).Padding(22).Radius(18).Gap(14).Background(p.bg).Border(1, p.hairline).Shadow(0, 24, 60, -12, ui.RGBA(0, 0, 0, 0.4))
		label := T("Edit Lyrics")
		if d.mode == "search" {
			label = T("Search Lyric Sources")
		}
		if d.mode == "timing" {
			label = T("Lyric Timing")
		}
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).Gap(4).Children(func() {
				ui.Text(c, label).FontSize(18).FontWeight(700)
				ui.Text(c, d.track.Name).FontSize(12).TextColor(p.muted).SingleLine()
			})
			if iconButton(c.Key("close"), iconX, 18, p.muted, T("Close")).Clicked() {
				d.open = false
				d.gen++
			}
		})
		switch d.mode {
		case "search":
			ui.Row(c).Gap(8).Children(func() {
				ui.TextInput(c.Key("title"), &d.title).Label(T("Song Title")).Placeholder(T("Song Title")).Grow(1)
				ui.TextInput(c.Key("artist"), &d.artist).Label(T("Artist")).Placeholder(T("Artist")).Grow(1)
				ui.Select(c.Key("provider"), &d.provider, []string{"All", "YouTube", "BetterLyrics", "LRCLIB", "KuGou"}).Label(T("Lyrics Source")).Width(120)
				if pillButton(c.Key("search"), iconSearch, T("Search"), p.accent, p.onAccent).Disabled(d.busy).Clicked() {
					a.searchLyricCandidates()
				}
			})
			ui.Row(c).Height(310).AlignItems(ui.Stretch).Gap(14).Children(func() {
				ui.List(c.Key("candidates"), &d.list, len(d.candidates), func(i int) {
					candidate := d.candidates[i]
					b := ui.ButtonBase(c.Key(candidate.ID)).Column().AlignItems(ui.Start).Padding(10).Gap(4).Radius(8).Label(candidate.Provider + ": " + candidate.Title)
					if b.Hovered() || d.previewID == candidate.ID {
						b.Background(p.hover)
					}
					b.Children(func() {
						ui.Text(c, candidate.Title).FontSize(13).FontWeight(600).MaxLines(2)
						ui.Text(c, candidate.Provider+" · "+candidate.Artist+" · "+clock(time.Duration(candidate.Duration)*time.Millisecond)).FontSize(11).TextColor(p.muted).MaxLines(2)
					})
					if b.Clicked() {
						a.previewLyricCandidate(candidate)
					}
				}).Width(260)
				ui.Scroll(c.Key("preview")).Grow(1).Padding(10).Background(p.hover).Radius(8).Children(func() {
					if d.preview != nil {
						ui.Text(c, lyrics.Export(*d.preview, false, 0)).FontSize(14).LineHeight(1.6)
					} else {
						ui.Text(c, T("Select a result to preview")).FontSize(13).TextColor(p.muted)
					}
				})
			})
		case "timing":
			r := a.pl.LyricsRecord()
			ui.Text(c, fmt.Sprintf("%+.1f s", float64(r.OffsetMS)/1000)).FontSize(30).FontWeight(700)
			ui.Row(c).Gap(8).Children(func() {
				for _, step := range []struct {
					label string
					ms    int64
				}{{"Earlier 500 ms", -500}, {"Earlier 100 ms", -100}, {"Later 100 ms", 100}, {"Later 500 ms", 500}} {
					if pillButton(c.Key(step.label), nil, T(step.label), p.hover, p.text).Clicked() {
						a.lyricOffset(step.ms)
					}
				}
			})
			if pillButton(c.Key("reset"), nil, T("Reset Timing"), p.hover, p.text).Clicked() {
				r.OffsetMS = 0
				a.applyLyricRecord(r)
			}
		default:
			ui.TextArea(c.Key("editor"), &d.text).Label(T("Lyrics Text")).Height(320).FontSize(14)
		}
		if d.err != "" {
			ui.Text(c, d.err).FontSize(12).TextColor(p.accent)
		}
		ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
			if d.busy {
				ui.Text(c, T("Loading…")).FontSize(12).TextColor(p.muted)
			}
			ui.Spacer(c)
			if pillButton(c.Key("cancel"), nil, T("Cancel"), p.hover, p.text).Clicked() {
				d.open = false
				d.gen++
			}
			if d.mode != "timing" {
				if pillButton(c.Key("apply"), nil, T("Apply Lyrics"), p.accent, p.onAccent).Disabled(d.busy || (d.mode == "search" && d.preview == nil)).Clicked() {
					data := lyrics.ParseDocument(d.text)
					if d.mode == "search" && d.preview != nil {
						data = *d.preview
					}
					if !lyrics.Usable(&data) {
						d.err = T("Lyrics are empty")
						return
					}
					r := a.pl.LyricsRecord()
					r.Data = data
					r.Manual = true
					r.Updated = time.Now()
					if a.applyLyricRecord(r) {
						d.open = false
						d.gen++
					}
				}
			}
		})
	})
}

func (a *app) lyricsSettings(c *ui.Context, row settingsRow, block settingsBlock, p palette) {
	s := &a.settings.Lyrics
	d := &a.lyricTools
	row(T("Show Original Lyrics"), "", func() {
		on := !s.HideOriginal
		if ui.Switch(c.Key("original"), &on).Label(T("Show Original Lyrics")).Changed() {
			s.HideOriginal = !on
			a.changed()
		}
	})
	row(T("Show Translation"), "", func() {
		on := !s.HideTranslation
		if ui.Switch(c.Key("translation"), &on).Label(T("Show Translation")).Changed() {
			s.HideTranslation = !on
			a.changed()
		}
	})
	row(T("Show Romanization"), T("Japanese readings, Korean romanization and Chinese pinyin"), func() {
		if ui.Switch(c.Key("romanization"), &s.Romanization).Label(T("Show Romanization")).Changed() {
			a.changed()
		}
	})
	row(T("Word Highlighting"), "", func() {
		if ui.Switch(c.Key("words"), &s.WordHighlight).Label(T("Word Highlighting")).Changed() {
			a.changed()
		}
	})
	row(T("Lyrics Font Size"), "", func() {
		values := []int{22, 26, 30, 34, 38, 42}
		labels := []string{"22", "26", "30", "34", "38", "42"}
		at := 2
		for i, v := range values {
			if s.FontSize == v {
				at = i
			}
		}
		if i, ok := choice(c, "lyric-font", T("Lyrics Font Size"), labels, at); ok {
			s.FontSize = values[i]
			a.changed()
		}
	})
	row(T("Lyrics Alignment"), "", func() {
		at := 0
		if s.Alignment == "center" {
			at = 1
		}
		if i, ok := choice(c, "lyric-align", T("Lyrics Alignment"), []string{"Left", "Center"}, at); ok {
			s.Alignment = []string{"left", "center"}[i]
			a.changed()
		}
	})
	row(T("Translation Service"), "", func() {
		at := 0
		if s.Translation.Provider == "deepl" {
			at = 1
		}
		if i, ok := choice(c, "translation-provider", T("Translation Service"), []string{"Compatible API", "DeepL"}, at); ok {
			s.Translation.Provider = []string{"compatible", "deepl"}[i]
			s.KeyReady = false
			if i == 1 {
				s.Translation.Endpoint = "https://api-free.deepl.com/v2/translate"
			} else {
				s.Translation.Endpoint = ""
			}
			a.changed()
		}
	})
	block(T("Translation Endpoint"), func() {
		ui.Column(c).Padding(12, 16).Gap(8).Children(func() {
			ui.Text(c, T("Translation Endpoint")).FontSize(12).TextColor(p.muted)
			if ui.TextInput(c.Key("translation-endpoint"), &s.Translation.Endpoint).Label(T("Translation Endpoint")).Placeholder("https://…").Changed() {
				s.KeyReady = false
			}
		})
	})
	if s.Translation.Provider != "deepl" {
		block(T("Translation Model"), func() {
			ui.Column(c).Padding(12, 16).Gap(8).Children(func() {
				ui.Text(c, T("Translation Model")).FontSize(12).TextColor(p.muted)
				ui.TextInput(c.Key("translation-model"), &s.Translation.Model).Label(T("Translation Model"))
			})
		})
	}
	row(T("Translation Language"), "", func() {
		codes := []string{"zh-CN", "zh-TW", "en", "ja", "ko", "fr", "de", "es"}
		names := []string{"Chinese (Simplified)", "Chinese (Traditional)", "English", "Japanese", "Korean", "French", "German", "Spanish"}
		at := 0
		for i, v := range codes {
			if s.Translation.Target == v {
				at = i
			}
		}
		if i, ok := choice(c, "target-language", T("Translation Language"), names, at); ok {
			s.Translation.Target = codes[i]
			a.changed()
		}
	})
	row(T("Translate Automatically"), T("Translation starts after lyrics load when a service is configured"), func() {
		if ui.Switch(c.Key("auto-translate"), &s.AutoTranslate).Label(T("Translate Automatically")).Changed() {
			a.changed()
		}
	})
	block(T("Translation API Key"), func() {
		ui.Column(c).Padding(12, 16).Gap(8).Children(func() {
			ui.Text(c, T("Translation API Key")).FontSize(12).TextColor(p.muted)
			ui.TextInput(c.Key("translation-key"), &d.keyInput).Password().Label(T("Translation API Key")).Placeholder(T("Enter a key to replace the saved key"))
			if s.KeyReady {
				ui.Text(c, T("Key saved in the system credential store")).FontSize(11).TextColor(p.muted)
			}
			if pillButton(c.Key("save-translation"), nil, T("Save Translation Settings"), p.accent, p.onAccent).Disabled(d.keyBusy).Clicked() {
				if d.cancel != nil {
					d.cancel()
					d.cancel = nil
					d.translating = false
				}

				cfg, key := s.Translation, d.keyInput
				d.keyBusy = true
				go func() {
					var err error
					if strings.TrimSpace(key) == "" {
						var saved string
						saved, err = lyricSecretGet(cfg)
						if err == nil && strings.TrimSpace(saved) == "" {
							err = fmt.Errorf("no saved key")
						}
					} else {
						err = lyricSecretSet(cfg, key)
					}
					a.post(func() {
						d.keyBusy = false
						if err != nil {
							a.toast = T("Couldn't save the translation key")
							return
						}
						d.keyInput = ""
						if s.Translation.Endpoint == cfg.Endpoint && s.Translation.Provider == cfg.Provider {
							s.KeyReady = true
						}
						a.changed()
					})
				}()
			}
		})
	})
}
