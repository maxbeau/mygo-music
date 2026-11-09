package app

import (
	"errors"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"mygo-music/internal/music"
	"mygo-music/internal/player"
)

// Retrying failed lyrics changes only the words, leaving audio progress and queue intact.
func TestRetryLyricsKeepsPlayback(t *testing.T) {
	a := newTestApp()
	ch := posts(a)
	a.pl = player.New(a.post)
	a.pl.Source = a.sourceByID
	requests := 0
	fake(a).lyrics = func(string) (*music.Lyrics, error) {
		requests++
		if requests == 1 {
			return nil, errors.New("offline")
		}
		return &music.Lyrics{Text: "First line\nSecond line"}, nil
	}
	a.pl.PlayTrack(sourced(song("seed", "Seed")), 0)
	until(t, ch, func() bool { _, loaded := a.pl.Lyrics(); return loaded })
	until(t, ch, func() bool { return !a.pl.Loading() })
	a.pl.Hold(time.Minute)
	version := a.pl.QueueVersion()
	a.nowPlaying, a.npPane = true, paneLyrics
	tt := ui.NewTester(a.view, 1180, 780)
	settled(tt)
	if err := tt.Click("Retry"); err != nil {
		t.Fatal(err)
	}
	until(t, ch, func() bool { _, loaded := a.pl.Lyrics(); return loaded })
	tt.Frame()
	if a.pl.LyricsError() != "" || a.pl.PlainLyrics() != "First line\nSecond line" {
		t.Fatal("retry did not load plain lyrics")
	}
	if a.pl.Position() != time.Minute || a.pl.QueueVersion() != version || len(a.pl.Queue()) != 1 || !a.pl.Playing() {
		t.Fatal("retry changed playback")
	}
	// A click in the plain text area cannot invent a seek target.
	tt.Press(800, 200)
	tt.Release(800, 200)
	if a.pl.Position() != time.Minute {
		t.Fatal("plain lyrics sought the audio")
	}
}

// With MM_FRAMES set, inspect readable plain lyrics and the retryable failure state.
func TestPlainLyricsFrames(t *testing.T) {
	dir := framesDir(t)
	a := newTestApp()
	ch := posts(a)
	a.pl = player.New(a.post)
	a.pl.Source = a.sourceByID
	fake(a).lyrics = func(string) (*music.Lyrics, error) {
		return &music.Lyrics{Text: "Plain lyrics without timestamps\n\nWords remain readable.\nScrolling does not seek the song.\n\n没有时间轴的歌词也能显示。"}, nil
	}
	a.pl.PlayTrack(framesTracks(a), 0)
	until(t, ch, func() bool { _, loaded := a.pl.Lyrics(); return loaded })
	a.nowPlaying, a.npPane = true, paneLyrics
	tt := ui.NewTester(a.view, 1180, 780)
	settled(tt)
	saveFrame(t, dir, "lyrics-plain.png", tt.Image())
	a.npPane = paneStage
	settled(tt)
	saveFrame(t, dir, "lyrics-plain-stage.png", tt.Image())
}
