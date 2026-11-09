package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"mygo-music/internal/music"
)

// A song's menu starts a station from that song, even while signed out when public.
func TestSongMenuStartsSeedRadio(t *testing.T) {
	a, ch := radioApp()
	a.account = nil
	a.accountAsked = true
	a.playlists["p"] = &music.Playlist{ID: "p", Tracks: sourced(song("seed", "Seed"))}
	a.route = route{page: pagePlaylist, src: "fake", id: "p"}
	asked := make(chan string, 1)
	fake(a).radio = func(kind, id string) (*music.Station, error) {
		asked <- kind + ":" + id
		return &music.Station{ID: "session", Name: "Seed Radio", Public: true}, nil
	}
	fake(a).stationNext = func(id string) ([]music.Track, error) {
		return sourced(song("a", "A"), song("b", "B"), song("c", "C")), nil
	}
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.RightClick("Seed"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Start Song Radio"); err != nil {
		t.Fatal(err)
	}
	if got := wait(t, asked, "seed radio"); got != "track:seed" {
		t.Fatal(got)
	}
	until(t, ch, func() bool { return a.pl.Radio() != "" })
	if a.pl.Current().ID != "a" || a.signIn.open {
		t.Fatalf("current %+v, sign-in %v", a.pl.Current(), a.signIn.open)
	}
}

// A delayed seed lookup cannot replace a song the listener chose afterwards.
func TestSeedRadioDropsStaleLookup(t *testing.T) {
	a, ch := radioApp()
	entered, release := make(chan bool, 1), make(chan bool)
	fake(a).radio = func(kind, id string) (*music.Station, error) {
		entered <- true
		<-release
		return &music.Station{ID: id, Public: true}, nil
	}
	fake(a).stationNext = func(string) ([]music.Track, error) { t.Error("stale lookup fetched a station"); return nil, nil }
	a.startSeedRadio(a.library, "track", "old")
	wait(t, entered, "lookup")
	a.pl.PlayQueue(sourced(song("chosen", "Chosen")), 0)
	close(release)
	runPost(t, ch)
	if a.pl.Current().ID != "chosen" || a.pl.Radio() != "" {
		t.Fatal("stale radio replaced playback")
	}
}

// Artist pages use the artist ID, rather than the first visible song, for radio.
func TestArtistButtonStartsSeedRadio(t *testing.T) {
	a, ch := radioApp()
	a.artists["artist"] = &artistData{page: &music.ArtistPage{Artist: music.Artist{ID: "artist", Name: "Singer"}}}
	a.route = route{page: pageArtist, src: "fake", id: "artist"}
	asked := make(chan string, 1)
	fake(a).radio = func(kind, id string) (*music.Station, error) {
		asked <- kind + ":" + id
		return nil, music.ErrUnavailable
	}
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.Click("Start Artist Radio"); err != nil {
		t.Fatal(err)
	}
	if got := wait(t, asked, "artist radio"); got != "artist:artist" {
		t.Fatal(got)
	}
	runPost(t, ch)
}

// Related music opens as a source-scoped browse page through the real menu.
func TestSongMenuOpensRelatedMusic(t *testing.T) {
	a := newTestApp()
	ch := posts(a)
	a.playlists["p"] = &music.Playlist{ID: "p", Tracks: sourced(song("seed", "Seed"))}
	a.route = route{page: pagePlaylist, src: "fake", id: "p"}
	asked := make(chan string, 1)
	fake(a).related = func(id string) (*music.Browse, error) {
		asked <- id
		return &music.Browse{Title: "Related", Sections: []music.Section{{Tracks: sourced(song("other", "Other"))}}}, nil
	}
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.RightClick("Seed"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Related Music"); err != nil {
		t.Fatal(err)
	}
	if got := wait(t, asked, "related"); got != "seed" {
		t.Fatal(got)
	}
	runPost(t, ch)
	if a.route.src != "fake" || a.route.id != relatedPagePrefix+"seed" {
		t.Fatalf("route %+v", a.route)
	}
	if a.pl.Current() != nil {
		t.Fatal("browsing started playback")
	}
}

// Autoplay is opt-in and does not request recommendations for podcast episodes.
func TestAutoplayRespectsSettingAndEpisodes(t *testing.T) {
	a := newTestApp()
	fake(a).radio = func(string, string) (*music.Station, error) { t.Error("requested radio"); return nil, nil }
	ready := func(string, []music.Track, func(func([]music.Track))) { t.Error("finished an unstarted request") }
	if a.autoplayRadio(music.Track{ID: "song", Source: "fake"}, ready) {
		t.Fatal("autoplay on by default")
	}
	a.settings.Autoplay = true
	if a.autoplayRadio(music.Track{ID: "episode", Source: "fake", ShowID: "show"}, ready) {
		t.Fatal("episode autoplayed")
	}
}

// Reordering a duplicate song selects its own occurrence, not the first same ID.
func TestMoveDuplicatePlaylistOccurrence(t *testing.T) {
	a := newTestApp()
	ch := posts(a)
	first, second := song("same", "Same"), song("same", "Same")
	first.PlaylistItemID, second.PlaylistItemID = "first", "second"
	a.playlists["p"] = &music.Playlist{ID: "p", Tracks: []music.Track{first, song("other", "Other"), second}}
	moved := make(chan [2]int, 1)
	fake(a).move = func(_ string, from, to int) error { moved <- [2]int{from, to}; return nil }
	a.moveTrack(a.library, "p", &second, -1)
	if got := wait(t, moved, "move"); got != [2]int{2, 1} {
		t.Fatal(got)
	}
	runPost(t, ch)
	if a.playlists["p"].Tracks[1].PlaylistItemID != "second" {
		t.Fatal("moved the wrong occurrence")
	}
}

// An initial station batch arriving after another queue cannot leave a tuning spinner.
func TestStationDropsStaleInitialBatch(t *testing.T) {
	a, ch := radioApp()
	release := make(chan bool)
	asked := make(chan bool, 1)
	fake(a).stationNext = func(string) ([]music.Track, error) { asked <- true; <-release; return sourced(song("old", "Old")), nil }
	a.playStation(a.library, &a.stations[0])
	wait(t, asked, "initial batch")
	a.pl.PlayQueue(sourced(song("chosen", "Chosen")), 0)
	close(release)
	runPost(t, ch)
	if a.radioStarting != "" || a.pl.Current().ID != "chosen" || a.pl.Radio() != "" {
		t.Fatal("stale batch changed playback or left spinner")
	}
}

// With MM_FRAMES set, saves the new artist sections and radio control alongside autoplay settings.
func TestDiscoveryArtistFrames(t *testing.T) {
	dir := framesDir(t)
	a := newTestApp()
	tracks := framesTracks(a)
	a.account = &music.Account{ID: "test"}
	a.artists["1"] = &artistData{page: &music.ArtistPage{
		Artist: music.Artist{ID: "1", Name: "The Polar Lights", ImageURL: "cover:0"}, Tracks: tracks,
		Sections: []music.Section{{Title: "Songs", Tracks: tracks[:2], MorePage: "all-songs"}, {Title: "Albums & Singles", Albums: []music.Album{tracks[0].Album, tracks[1].Album}}},
	}}
	a.route = route{page: pageArtist, src: "fake", id: "1"}
	tt := ui.NewTester(a.view, 1180, 780)
	settled(tt)
	saveFrame(t, dir, "artist-discovery.png", tt.Image())
	a.settingsPage = "playback"
	settings := ui.NewTester(a.settingsView, 780, 640)
	settled(settings)
	saveFrame(t, dir, "autoplay-settings.png", settings.Image())
}
