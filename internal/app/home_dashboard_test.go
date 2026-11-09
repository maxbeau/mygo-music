package app

import (
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"mygo-music/internal/music"
)

func dashboardApp() *app {
	a := newTestApp()
	a.home = &music.Browse{Layout: "discovery", Sections: []music.Section{{Title: "For You"}}}
	a.homeAt, a.mineAt = time.Now(), time.Now()
	a.started, a.accountAsked = true, true
	return a
}

// Mixed shelf order wins over the compatibility lists, and songs are not counted twice.
func TestHomeEntriesKeepOrderAndDedupeSongs(t *testing.T) {
	a := music.Artist{ID: "artist", Name: "Singer"}
	p := music.Playlist{ID: "playlist", Name: "Mix"}
	song := music.Track{ID: "song", Name: "Song"}
	section := music.Section{Entries: []music.BrowseEntry{{Artist: &a}, {Playlist: &p}, {Track: &song}}, Tracks: []music.Track{song}}
	entries := sectionEntries(section)
	if entries[0].Artist == nil || entries[1].Playlist == nil || entries[2].Track == nil {
		t.Fatal("mixed entries were regrouped")
	}
	tracks := homeTracks(&music.Browse{Sections: []music.Section{section, section}})
	if len(tracks) != 1 {
		t.Fatalf("tracks %+v", tracks)
	}
}

// A large shelf stays compact on Home, but See All retains every item and its navigation.
func TestHomeShelfSeeAllKeepsHiddenItems(t *testing.T) {
	a := dashboardApp()
	for i := 0; i < 12; i++ {
		artist := music.Artist{ID: fmt.Sprint(i), Name: fmt.Sprintf("Singer %d", i)}
		a.home.Sections[0].Entries = append(a.home.Sections[0].Entries, music.BrowseEntry{Artist: &artist})
	}
	tt := ui.NewTester(contentOnly(a), 1180, 780)
	tt.Frame()
	if err := tt.Click("See All"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.route.page != pageSection || len(a.sectionOf(a.route).Entries) != 12 {
		t.Fatal("full shelf was lost")
	}
	if err := tt.Click("Singer 0"); err != nil {
		t.Fatal(err)
	}
	if a.route.page != pageArtist || a.route.id != "0" {
		t.Fatal(a.route)
	}
}

// A quick pick plays from its full queue, rather than discarding songs outside the preview.
func TestHomeQuickPickPlaysFullQueue(t *testing.T) {
	a := dashboardApp()
	tracks := framesTracks(a)
	a.home.Sections[0].Tracks = tracks
	tt := ui.NewTester(contentOnly(a), 1180, 780)
	tt.Frame()
	if err := tt.Click(tracks[0].Name); err != nil {
		t.Fatal(err)
	}
	if a.pl.Current().ID != tracks[0].ID || len(a.pl.Queue()) != len(tracks) {
		t.Fatal("quick pick lost its queue")
	}
}

// Surprise Me creates a radio from one of the supplied quick picks.
func TestHomeSurpriseStartsSeededRadio(t *testing.T) {
	a := dashboardApp()
	ch := posts(a)
	tracks := framesTracks(a)
	a.home.Sections[0].Tracks = tracks
	asked := make(chan string, 1)
	fake(a).radio = func(kind, id string) (*music.Station, error) {
		asked <- id
		return &music.Station{ID: "session", Name: "Surprise", Public: true}, nil
	}
	fake(a).stationNext = func(string) ([]music.Track, error) {
		return sourced(song("next", "Next"), song("later", "Later"), song("last", "Last")), nil
	}
	tt := ui.NewTester(contentOnly(a), 1180, 780)
	tt.Frame()
	if err := tt.Click("Surprise Me"); err != nil {
		t.Fatal(err)
	}
	seed := wait(t, asked, "surprise radio")
	found := false
	for _, track := range tracks {
		found = found || track.ID == seed
	}
	if !found {
		t.Fatal(seed)
	}
	until(t, ch, func() bool { return a.pl.Radio() != "" })
	if a.pl.Current().ID != "next" {
		t.Fatal("radio did not start")
	}
}

// Quick Picks' full collection is local navigation and never asks the source for a fabricated page ID.
func TestHomeQuickCollectionDoesNotCallBrowse(t *testing.T) {
	a := dashboardApp()
	a.home.Sections[0].Tracks = framesTracks(a)
	fake(a).browse = func(string) (*music.Browse, error) { t.Error("called browse for a local collection"); return nil, nil }
	tt := ui.NewTester(contentOnly(a), 1180, 780)
	tt.Frame()
	if err := tt.Click("See All"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.route.page != pageHomeCollection || len(a.browses[a.route.id].page.Sections[0].Entries) != 8 {
		t.Fatal("collection lost songs")
	}
	a.open(route{page: pagePlaylists})
	a.libraryChanged(a.library)
	a.back()
	tt.Frame()
	if a.route.page != pageHomeCollection || a.browses[a.route.id] == nil {
		t.Fatal("library edit broke collection history")
	}
	a.back()
	if a.route.page != pageHome {
		t.Fatal(a.route)
	}
}

// Refreshing Home discards a related response that belongs to the previous home snapshot.
func TestHomeRelatedDiscardsOldResponse(t *testing.T) {
	a := dashboardApp()
	ch := posts(a)
	a.pl.PlayTrack(framesTracks(a), 0)
	seed := a.pl.Current().ID
	asked := make(chan string, 1)
	release := make(chan struct{})
	fake(a).related = func(id string) (*music.Browse, error) {
		asked <- id
		<-release
		return &music.Browse{Title: "Old"}, nil
	}
	a.loadHomeRelated(a.library)
	if got := wait(t, asked, "home related"); got != seed {
		close(release)
		t.Fatal(got)
	}
	replacement := &browseData{page: &music.Browse{Title: "New"}}
	a.homeRelated = replacement
	close(release)
	runPost(t, ch)
	if a.homeRelated != replacement || a.homeRelated.page.Title != "New" {
		t.Fatal("stale related response replaced the refreshed homepage")
	}
}

// With MM_FRAMES set, renders the discovery homepage at standard and narrow desktop widths.
func TestHomeDashboardFrames(t *testing.T) {
	dir := framesDir(t)
	a := dashboardApp()
	tracks := framesTracks(a)
	a.mine = []music.Playlist{{ID: "p", Name: "Evening Walks", Mine: true, ImageURL: "cover:0"}, {ID: "q", Name: "Focus", Mine: true, ImageURL: "cover:1"}}
	a.home.Sections = []music.Section{
		{Title: "Listen Again", Entries: []music.BrowseEntry{{Artist: &tracks[0].Artists[0]}, {Track: &tracks[0]}, {Playlist: &a.mine[0]}, {Track: &tracks[1]}, {Album: &tracks[2].Album}, {Track: &tracks[3]}, {Track: &tracks[4]}, {Track: &tracks[5]}, {Track: &tracks[6]}, {Track: &tracks[7]}}},
		{Title: "Explore More", Entries: []music.BrowseEntry{{Station: &music.Station{ID: "mix", Name: "Your Mix", ImageURL: "cover:2", Public: true}}}},
	}
	tt := ui.NewTester(a.view, 1180, 780)
	settled(tt)
	saveFrame(t, dir, "home-dashboard.png", tt.Image())
	narrow := ui.NewTester(a.view, 880, 660)
	settled(narrow)
	saveFrame(t, dir, "home-dashboard-narrow.png", narrow.Image())
}
