package player

import (
	"mygo-music/internal/music"
	"testing"
)

// Autoplay starts only at natural completion and preserves the finished queue.
func TestAutoplayContinuesFinishedQueue(t *testing.T) {
	p := New(func(func()) {})
	p.Cue([]music.Track{{ID: "seed"}})
	p.playing = true
	var ready func(string, []music.Track, func(func([]music.Track)))
	p.Autoplay = func(track music.Track, got func(string, []music.Track, func(func([]music.Track)))) bool {
		if track.ID != "seed" {
			t.Fatal(track)
		}
		ready = got
		return true
	}
	p.ended()
	if ready == nil || !p.Loading() {
		t.Fatal("not waiting for recommendations")
	}
	ready("station", []music.Track{{ID: "seed"}, {ID: "next"}, {ID: "later"}, {ID: "last"}}, func(func([]music.Track)) {})
	if p.Radio() != "station" || p.Current().ID != "next" || len(p.Queue()) != 4 {
		t.Fatalf("radio %q current %+v queue %+v", p.Radio(), p.Current(), p.Queue())
	}
}

// A newer playback request invalidates recommendations still loading.
func TestAutoplayDropsStaleResponse(t *testing.T) {
	p := New(func(func()) {})
	p.Cue([]music.Track{{ID: "seed"}})
	p.playing = true
	var ready func(string, []music.Track, func(func([]music.Track)))
	p.Autoplay = func(_ music.Track, got func(string, []music.Track, func(func([]music.Track)))) bool {
		ready = got
		return true
	}
	p.ended()
	p.PlayQueue([]music.Track{{ID: "chosen"}}, 0)
	ready("stale", []music.Track{{ID: "old"}}, nil)
	if p.Current().ID != "chosen" || p.Radio() != "" || len(p.Queue()) != 1 {
		t.Fatal("old recommendations replaced playback")
	}
}

// Pausing while recommendations load does not start the resulting track.
func TestAutoplayKeepsPause(t *testing.T) {
	p := New(func(func()) {})
	p.Cue([]music.Track{{ID: "seed"}})
	p.playing = true
	var ready func(string, []music.Track, func(func([]music.Track)))
	p.Autoplay = func(_ music.Track, got func(string, []music.Track, func(func([]music.Track)))) bool {
		ready = got
		return true
	}
	p.ended()
	p.TogglePlay()
	ready("station", []music.Track{{ID: "next"}, {ID: "later"}, {ID: "last"}}, func(func([]music.Track)) {})
	if p.Playing() || p.Current().ID != "next" {
		t.Fatal("autoplay ignored pause")
	}
}

// Repeat-all keeps the existing queue and never asks for an autoplay station.
func TestAutoplayYieldsToRepeat(t *testing.T) {
	p := New(func(func()) {})
	p.Cue([]music.Track{{ID: "seed"}})
	p.repeat = RepeatAll
	p.Autoplay = func(music.Track, func(string, []music.Track, func(func([]music.Track)))) bool {
		t.Error("repeat requested autoplay")
		return false
	}
	p.ended()
	if p.Radio() != "" || len(p.Queue()) != 1 {
		t.Fatal("repeat changed queue")
	}
}

// Seeking while autoplay waits reopens the finished song and invalidates that response.
func TestAutoplaySeekCancelsRecommendations(t *testing.T) {
	p := New(func(func()) {})
	p.Cue([]music.Track{{ID: "seed"}})
	p.playing = true
	var ready func(string, []music.Track, func(func([]music.Track)))
	p.Autoplay = func(_ music.Track, got func(string, []music.Track, func(func([]music.Track)))) bool {
		ready = got
		return true
	}
	p.ended()
	p.Seek(0)
	ready("stale", []music.Track{{ID: "next"}}, nil)
	if p.Current().ID != "seed" || p.Radio() != "" || len(p.Queue()) != 1 {
		t.Fatal("seeking was replaced by autoplay")
	}
}
