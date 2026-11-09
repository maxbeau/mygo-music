package player

import (
	"sync/atomic"
	"testing"
	"time"

	"mygo-music/internal/lyrics"
	"mygo-music/internal/music"
)

type lyricSource struct {
	nothing
	get func(string) (*music.Lyrics, error)
}

func (s lyricSource) Can(method string) bool                  { return method == "lyrics" }
func (s lyricSource) Lyrics(id string) (*music.Lyrics, error) { return s.get(id) }

// A retry on the same track and an old track response both obey the newest lyrics request.
func TestLyricsRejectOlderResponse(t *testing.T) {
	for _, scenario := range []string{"retry", "change-track"} {
		t.Run(scenario, func(t *testing.T) {
			ch := make(chan func(), 8)
			p := New(func(fn func()) { ch <- fn })
			p.Cue([]music.Track{{ID: "old"}})
			entered, release := make(chan bool, 1), make(chan bool)
			var calls atomic.Int32
			src := lyricSource{get: func(string) (*music.Lyrics, error) {
				if calls.Add(1) == 1 {
					entered <- true
					<-release
					return &music.Lyrics{Text: "Old words"}, nil
				}
				return &music.Lyrics{Text: "New words"}, nil
			}}
			p.Source = func(string) music.Source { return src }
			p.RetryLyrics()
			select {
			case <-entered:
			case <-time.After(2 * time.Second):
				close(release)
				t.Fatal("lyrics were not requested")
			}
			if scenario == "change-track" {
				p.gen++
				p.Cue([]music.Track{{ID: "new"}})
			}
			p.RetryLyrics()
			next(t, ch)
			close(release)
			next(t, ch)
			if p.PlainLyrics() != "New words" || p.LyricsError() != "" {
				t.Fatal("older response replaced new lyrics")
			}
		})
	}
}

// Manual lyrics win over an in-flight automatic response on disk and when retried offline.
func TestManualLyricsSurvivePendingFetch(t *testing.T) {
	ch := make(chan func(), 8)
	p := New(func(fn func()) { ch <- fn })
	p.Cue([]music.Track{{ID: "seed", Source: "test"}})
	p.LyricsStore = &lyrics.Store{Dir: t.TempDir()}
	entered, release := make(chan bool, 1), make(chan bool)
	src := lyricSource{get: func(string) (*music.Lyrics, error) {
		entered <- true
		<-release
		return &music.Lyrics{Text: "Automatic"}, nil
	}}
	p.Source = func(string) music.Source { return src }
	p.RetryLyrics()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("not requested")
	}
	r := lyrics.Record{Data: lyrics.ParseDocument("Manual words"), Manual: true, OffsetMS: 500}
	if err := p.ApplyLyrics(r); err != nil {
		t.Fatal(err)
	}
	close(release)
	next(t, ch)
	if p.PlainLyrics() != "Manual words" || !p.LyricsStore.Read(*p.Current()).Manual {
		t.Fatal("stale fetch overwrote manual")
	}
	p.RetryLyrics()
	next(t, ch)
	if p.PlainLyrics() != "Manual words" || p.LyricsRecord().OffsetMS != 500 {
		t.Fatal("manual retry changed selection")
	}
	// A fresh player loads the same record without contacting its source.
	fresh := New(func(fn func()) { ch <- fn })
	fresh.Cue([]music.Track{*p.Current()})
	fresh.LyricsStore = &lyrics.Store{Dir: p.LyricsStore.Dir}
	fresh.loadLyrics(fresh.gen, nil, *fresh.Current())
	next(t, ch)
	if fresh.PlainLyrics() != "Manual words" {
		t.Fatal("offline restart lost lyrics")
	}
}
