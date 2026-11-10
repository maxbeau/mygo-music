package player

import (
	"errors"
	"mygo-music/internal/music"
	"testing"
	"time"
)

// A temporary outage preserves the song and progress; a server cooldown cannot be bypassed by network notifications.
func TestRecoveryRetainsPlaybackAndCooldown(t *testing.T) {
	ch := make(chan func(), 16)
	p := New(func(fn func()) { ch <- fn })
	p.Cue([]music.Track{{ID: "one", Duration: 300000}, {ID: "two", Duration: 300000}})
	p.Hold(time.Minute)
	now := time.Unix(1, 0)
	p.now = func() time.Time { return now }
	var delayed func()
	var wait time.Duration
	p.after = func(d time.Duration, fn func()) { wait, delayed = d, fn }
	version := p.QueueVersion()
	if !p.recoverPlayback(&music.TemporaryError{After: 30 * time.Second, Limited: true}) {
		t.Fatal("not classified")
	}
	p.NetworkRestored()
	if !p.Recovering() || wait != 30*time.Second || p.Current().ID != "one" || p.Position() != time.Minute || p.QueueVersion() != version {
		t.Fatal("recovery changed playback")
	}
	p.TogglePlay()
	now = now.Add(wait)
	delayed()
	next(t, ch)
	if p.Playing() || !p.Recovering() {
		t.Fatal("retry ignored pause")
	}
	p.gen++
	p.retryPending = false
	delayed()
	next(t, ch)
	if p.Current().ID != "one" {
		t.Fatal("stale retry changed track")
	}
}

// Repeated transient errors stop after a bounded number of attempts and never advance the queue.
func TestRecoveryStopsAfterBudget(t *testing.T) {
	p := New(func(func()) {})
	p.Cue([]music.Track{{ID: "one"}, {ID: "two"}})
	p.Hold(time.Second)
	p.after = func(time.Duration, func()) {}
	for i := 0; i < 6; i++ {
		if !p.recoverPlayback(&music.TemporaryError{}) {
			t.Fatal("unhandled")
		}
	}
	if p.Playing() || p.Recovering() || p.Current().ID != "one" || p.Err() == "" {
		t.Fatal("unbounded recovery")
	}
	if p.recoverPlayback(music.ErrSignIn) || p.recoverPlayback(errors.New("bad format")) {
		t.Fatal("permanent failure retried")
	}
}
