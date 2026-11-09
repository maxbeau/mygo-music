package player

import (
	"testing"
	"time"

	"mygo-music/internal/music"
)

type radioReady = func(string, []music.Track, func(func([]music.Track)))

// The current song keeps its audio, position and pause while radio replaces only its future.
func TestSongRadioPreservesAudioAndHistory(t *testing.T) {
	p, ch, src, q := servedQueue(t, "old-next", "seed", "past")
	p.PlayQueue(q, 1)
	settle(t, ch, p)
	// A shuffled queue's past must be retained in play order, not storage order.
	p.order, p.at, p.shuffle = []int{2, 1, 0}, 1, true
	p.out.Pause()
	p.out.Seek(time.Second)
	p.playing = false
	out, gen, pos := p.out, p.gen, p.Position()
	var ready radioReady
	p.StartSongRadio(q[1], func(got radioReady) { ready = got })
	ready("radio", []music.Track{q[1], q[2], {ID: "a", Source: "s"}, {ID: "b", Source: "s"}, {ID: "c", Source: "s"}}, func(func([]music.Track)) {})
	if p.out != out || p.gen != gen || p.Position() != pos || p.Playing() {
		t.Fatal("radio reloaded or resumed the current audio")
	}
	want := []string{"past", "seed", "a", "b", "c"}
	if len(p.queue) != len(want) || p.at != 1 || !p.shuffle {
		t.Fatalf("queue %+v at %d shuffled %v", p.queue, p.at, p.shuffle)
	}
	for i, id := range want {
		if p.queue[p.order[i]].ID != id {
			t.Fatalf("queue order %+v", p.queue)
		}
	}
	if p.ahead == nil || p.ahead.id != "a" || src.count(src.asked, "seed") != 1 {
		t.Fatal("next preload or current stream changed incorrectly")
	}
	// Exercise the same completion path as the unchanged audio's OnEnd callback.
	p.playing = true
	p.ended()
	if p.Current().ID != "a" {
		t.Fatal("natural completion did not enter radio")
	}
	settle(t, ch, p)
}

// Natural completion during preparation waits instead of advancing into the old playlist.
func TestSongRadioWaitsAndKeepsPause(t *testing.T) {
	p := New(func(func()) {})
	p.Cue([]music.Track{{ID: "seed"}, {ID: "old-next"}})
	p.Hold(time.Minute)
	var ready radioReady
	p.StartSongRadio(*p.Current(), func(got radioReady) { ready = got })
	p.ended()
	if p.Current().ID != "seed" || !p.awaiting || !p.Loading() {
		t.Fatal("radio skipped into the old queue while loading")
	}
	p.TogglePlay()
	ready("radio", []music.Track{{ID: "a"}, {ID: "b"}, {ID: "c"}}, func(func([]music.Track)) {})
	if p.Current().ID != "a" || p.Playing() || p.awaiting || p.tuning {
		t.Fatal("prepared radio ignored the pause or failed to advance")
	}
}

// A failed preparation leaves the current song and its original future intact.
func TestSongRadioFailureKeepsQueue(t *testing.T) {
	p := New(func(func()) {})
	p.Cue([]music.Track{{ID: "seed"}, {ID: "old-next"}})
	p.Hold(time.Minute)
	var ready radioReady
	p.StartSongRadio(*p.Current(), func(got radioReady) { ready = got })
	ready("", nil, nil)
	if p.Current().ID != "seed" || p.Position() != time.Minute || !p.Playing() || len(p.queue) != 2 || p.tuning {
		t.Fatal("failed radio changed playback")
	}
}

// Next with no old successor waits for radio rather than restarting the seed.
func TestSongRadioNextWaitsAtSeed(t *testing.T) {
	p := New(func(func()) {})
	p.Cue([]music.Track{{ID: "seed"}})
	p.Hold(time.Minute)
	var ready radioReady
	p.StartSongRadio(*p.Current(), func(got radioReady) { ready = got })
	gen := p.gen
	p.Next()
	if p.gen != gen || !p.awaiting || !p.Loading() {
		t.Fatal("Next restarted the seed during preparation")
	}
	ready("radio", []music.Track{{ID: "a"}, {ID: "b"}, {ID: "c"}}, func(func([]music.Track)) {})
	if p.Current().ID != "a" || !p.Playing() {
		t.Fatal("Next did not advance once recommendations arrived")
	}
}

// Failure after natural completion stops waiting and does not fall into the old playlist.
func TestSongRadioFailureAfterEndStops(t *testing.T) {
	p := New(func(func()) {})
	p.Cue([]music.Track{{ID: "seed"}, {ID: "old"}})
	p.Hold(time.Minute)
	var ready radioReady
	p.StartSongRadio(*p.Current(), func(got radioReady) { ready = got })
	p.ended()
	ready("", nil, nil)
	if p.Current().ID != "seed" || p.Playing() || p.Loading() || p.awaiting || p.tuning {
		t.Fatal("failed radio stayed busy or started another song")
	}
}

// Switching tracks, seeking after completion, leaving, or starting again invalidates old results.
func TestSongRadioRejectsSupersededResults(t *testing.T) {
	for _, action := range []string{"next", "seek", "leave", "restart"} {
		t.Run(action, func(t *testing.T) {
			p := New(func(func()) {})
			p.Cue([]music.Track{{ID: "seed"}, {ID: "chosen"}})
			var ready radioReady
			p.StartSongRadio(*p.Current(), func(got radioReady) { ready = got })
			switch action {
			case "next":
				p.Next()
			case "seek":
				p.ended()
				p.Seek(0)
			case "leave":
				p.EndRadio()
			case "restart":
				p.StartSongRadio(*p.Current(), func(r radioReady) {
					r("new", []music.Track{{ID: "new-next"}, {ID: "b"}, {ID: "c"}}, func(func([]music.Track)) {})
				})
			}
			ready("stale", []music.Track{{ID: "wrong"}}, nil)
			if p.Radio() == "stale" {
				t.Fatal("old radio attached")
			}
			for _, track := range p.queue {
				if track.ID == "wrong" {
					t.Fatal("old radio changed the queue")
				}
			}
		})
	}
}

// A continuation already in flight for a previous station cannot pollute the new radio.
func TestSongRadioRejectsOldFeed(t *testing.T) {
	p := New(func(func()) {})
	p.Cue([]music.Track{{ID: "seed"}, {ID: "old-next"}})
	var old func([]music.Track)
	p.AttachRadio("old", func(got func([]music.Track)) { old = got })
	var ready radioReady
	p.StartSongRadio(*p.Current(), func(got radioReady) { ready = got })
	old([]music.Track{{ID: "stale"}})
	ready("new", []music.Track{{ID: "a"}, {ID: "b"}, {ID: "c"}}, func(func([]music.Track)) {})
	if len(p.queue) != 4 || p.queue[1].ID != "a" || p.Radio() != "new" {
		t.Fatalf("queue %+v radio %q", p.queue, p.Radio())
	}
}

// Equal IDs from different sources are different seeds and must start the selected song.
func TestSongRadioStartsOtherSourceSeed(t *testing.T) {
	p := New(func(func()) {})
	p.Cue([]music.Track{{ID: "same", Source: "first"}})
	p.Hold(time.Minute)
	p.StartSongRadio(music.Track{ID: "same", Source: "second"}, func(r radioReady) {})
	if p.Current().Source != "second" || p.Position() != 0 || !p.Playing() {
		t.Fatal("source identity was ignored")
	}
}
