package app

import (
	"strings"

	"mygo-music/internal/music"
)

// startSeedRadio resolves a seed off the UI thread. A later playback request
// wins even when an older network request answers last.
func (a *app) startSeedRadio(lib *library, kind, id string) {
	src, ok := lib.src.(music.RadioSource)
	if !ok || !lib.can("radio") || !lib.can("stationNext") {
		return
	}
	a.radioRequest++
	a.radioStarting = ""
	request, version := a.radioRequest, a.pl.QueueVersion()
	go func() {
		station, err := src.Radio(kind, id)
		a.post(func() {
			if request != a.radioRequest || version != a.pl.QueueVersion() || a.libOf(lib.id) != lib {
				return
			}
			if err != nil {
				a.actionFailed(lib, err, T("Couldn't start this radio"))
				return
			}
			if station == nil || station.ID == "" {
				return
			}
			a.playStation(lib, station)
		})
	}()
}

// autoplayRadio starts only after natural completion; a manual Next retains
// its normal behavior. It never replaces the listener's queue while loading.
func (a *app) autoplayRadio(track music.Track, ready func(string, []music.Track, func(func([]music.Track)))) bool {
	lib := a.libOf(track.Source)
	if !a.settings.Autoplay || track.ShowID != "" || lib == nil || !lib.can("radio") || !lib.can("stationNext") {
		return false
	}
	src, ok := lib.src.(music.RadioSource)
	if !ok {
		return false
	}
	a.radioRequest++
	version, request := a.pl.QueueVersion(), a.radioRequest
	go func() {
		st, err := src.Radio("track", track.ID)
		var tracks []music.Track
		if err == nil && st != nil && st.ID != "" {
			tracks, err = lib.src.StationNext(st.ID)
		}
		a.post(func() {
			if version != a.pl.QueueVersion() {
				return
			}
			if err != nil {
				a.actionFailed(lib, err, T("Couldn't start this radio"))
			}
			if request != a.radioRequest || !a.settings.Autoplay || a.libOf(lib.id) != lib || err != nil || st == nil || st.ID == "" {
				ready("", nil, nil)
				return
			}
			name := radioName(lib.id, st)
			ready(name, playable(tracks), a.radioFeed(name))
		})
	}()
	return true
}

const relatedPagePrefix = "@related:"

func (a *app) openRelated(lib *library, id string) {
	a.openPage(route{page: pageBrowse, src: lib.id, id: relatedPagePrefix + id})
}

// relatedBrowse keeps the same loading, back navigation and retry behavior as
// other browse pages while keeping track IDs private to their source.
func relatedBrowse(src music.Source, page string) (*music.Browse, error) {
	if strings.HasPrefix(page, relatedPagePrefix) {
		if related, ok := src.(music.RelatedSource); ok && src.Can("related") {
			return related.Related(strings.TrimPrefix(page, relatedPagePrefix))
		}
		return nil, music.ErrUnsupported
	}
	return src.Browse(page)
}
