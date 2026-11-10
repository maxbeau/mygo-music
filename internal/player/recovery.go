package player

import (
	"mygo-music/internal/i18n"
	"mygo-music/internal/music"
	"time"
)

func (p *Player) Recovering() bool { return p.retryPending }
func (p *Player) RecoveryStatus() string {
	if !p.retryPending {
		return ""
	}
	if p.retryLimited {
		return i18n.T("Service is busy. Waiting to retry…")
	}
	return i18n.T("Connection lost. Reconnecting…")
}
func (p *Player) recoverPlayback(err error) bool {
	delay, limited, ok := music.RetryDelay(err)
	if !ok {
		return false
	}
	p.pending = p.Position()
	p.dropAhead()
	p.stop()
	if p.retryCount >= 5 {
		p.err = i18n.T("Couldn't reconnect. Press Play to retry.")
		p.playing = false
		p.retryPending = false
		p.sync()
		return true
	}
	if delay <= 0 {
		delay = time.Second * time.Duration(1<<min(p.retryCount, 4))
	}
	delay = min(delay, 15*time.Minute)
	p.retryCount++
	p.retryPending = true
	p.retryLimited = limited
	p.retryAt = p.now().Add(delay)
	p.err = ""
	p.loading = false
	gen, media := p.gen, p.mediaGen
	p.after(delay, func() {
		p.post(func() {
			if p.gen == gen && p.mediaGen == media && p.retryPending && p.playing && !p.networkOffline {
				p.retryNow()
			}
		})
	})
	p.sync()
	return true
}
func (p *Player) retryNow() {
	if !p.retryPending || !p.playing || p.networkOffline || p.now().Before(p.retryAt) {
		return
	}
	attempts, recorded := p.retryCount, p.recorded
	p.loadSkipping(p.at, 0, stay, p.playing, p.pending, true)
	p.retryCount, p.recorded = attempts, recorded
}

// NetworkRestored resumes transient failures; server cooldowns always retain their deadline.
func (p *Player) NetworkRestored() {
	if p.retryPending {
		if !p.retryLimited {
			p.retryAt = p.now()
		}
		p.retryNow()
	}
}
func (p *Player) audioEnded() {
	if p.stream != nil {
		if dl, _ := p.stream.source(); dl != nil {
			dl.mu.Lock()
			err := dl.err
			local := dl.onDisk
			dl.mu.Unlock()
			if err == nil && !local && p.Position()+max(3*time.Second, p.Duration()/50) < p.Duration() {
				err = &music.TemporaryError{Message: "stream ended early"}
			}
			if err != nil && p.recoverPlayback(err) {
				return
			}
		}
	}
	p.retryCount = 0
	p.ended()
}

func (p *Player) RetryPlayback() {
	if p.track == nil {
		return
	}
	pos := p.Position()
	p.loadSkipping(p.at, 0, stay, true, pos, true)
}

func (p *Player) NetworkChanged(online bool) {
	p.networkOffline = !online
	if online {
		p.NetworkRestored()
	}
}
