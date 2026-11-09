package app

import (
	"encoding/json"
	"github.com/egoist/mygo/ui"
	"mygo-music/internal/music"
	"os"
	"path/filepath"
	"testing"
)

// Pin removal affects only navigation, not cloud playlists or playback.
func TestSidebarUnpinKeepsLibraryAndQueue(t *testing.T) {
	a := editTestApp()
	a.pl.Cue(a.playlists["10"].Tracks)
	before := a.route
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.RightClick("睡前"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Unpin from Sidebar"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if _, ok := tt.Find("睡前"); ok {
		t.Fatal("unpinned shortcut stayed")
	}
	if a.route != before || len(a.mine) != 2 || len(a.saved) != 1 || len(a.pl.Queue()) != 2 {
		t.Fatal("unpin changed music data")
	}
	// Empty pins are an explicit preference, not a request to seed defaults again.
	a.setPins(a.id, nil)
	a.seedPins(a.library)
	if len(a.pinsFor(a.library)) != 0 {
		t.Fatal("empty pins were reseeded")
	}
}

// Local shortcuts persist independently by source, including equal IDs of different types.
func TestSidebarPinsPersistBySource(t *testing.T) {
	a := editTestApp()
	a.settings.path = filepath.Join(t.TempDir(), "settings.json")
	a.setPins(a.id, nil)
	a.togglePin(a.library, sidebarPin{Kind: "artist", ID: "same", Name: "Singer"})
	a.togglePin(a.library, sidebarPin{Kind: "album", ID: "same", Name: "Record"})
	other := addFake(a, newFakeSource("other"))
	a.togglePin(other, sidebarPin{Kind: "playlist", ID: "same", Name: "Other"})
	if len(a.pinsFor(a.library)) != 2 || len(a.pinsFor(other)) != 1 {
		t.Fatal("sources or types collided")
	}
	b, err := os.ReadFile(a.settings.path)
	if err != nil {
		t.Fatal(err)
	}
	var saved settings
	if err := json.Unmarshal(b, &saved); err != nil {
		t.Fatal(err)
	}
	a.settings.SidebarPins = saved.SidebarPins
	if len(a.pinsFor(a.library)) != 2 || a.pinsFor(other)[0].Name != "Other" {
		t.Fatal("pins did not survive reload")
	}
}

// Dragging a pin changes persistent order without navigating or starting playback.
func TestSidebarPinsDragReorder(t *testing.T) {
	a := editTestApp()
	a.setPins(a.id, []sidebarPin{playlistPin(a.mine[0]), playlistPin(a.mine[1]), playlistPin(a.saved[0])})
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	first, _ := tt.Find("通勤")
	last, _ := tt.Find("飙升榜")
	before := a.route
	tt.Press(first.X+12, first.Y+first.H/2)
	tt.Move(first.X+12, first.Y+first.H/2+8)
	tt.Frame()
	tt.Move(last.X+12, last.Y+last.H/2)
	tt.Frame()
	tt.Release(last.X+12, last.Y+last.H/2)
	tt.Frame()
	pins := a.pinsFor(a.library)
	if pins[0].ID != "11" || pins[2].ID != "10" {
		t.Fatalf("order %+v", pins)
	}
	if a.route != before || a.pl.Current() != nil {
		t.Fatal("drag navigated or played music")
	}
}

// The library keeps creation available even after every shortcut has been removed.
func TestNewPlaylistWithoutSidebarPins(t *testing.T) {
	a := editTestApp()
	a.setPins(a.id, nil)
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.Click("New Playlist"); err != nil {
		t.Fatal(err)
	}
	if !a.naming.open {
		t.Fatal("creation dialog did not open")
	}
}

// Explore loads just the selected tab and back restores the previous tab.
func TestExploreTabsLoadAndBack(t *testing.T) {
	a := newTestApp()
	ch := posts(a)
	fake(a).info.Navigation = []music.NavItem{{Builtin: "home"}, {Group: "explore", Page: "charts", Title: "Charts"}, {Group: "explore", Page: "moods", Title: "Moods"}}
	asked := make(chan string, 3)
	fake(a).browse = func(page string) (*music.Browse, error) { asked <- page; return &music.Browse{Title: page}, nil }
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.Click("Explore"); err != nil {
		t.Fatal(err)
	}
	if got := wait(t, asked, "explore"); got != "charts" {
		t.Fatal(got)
	}
	runPost(t, ch)
	tt.Frame()
	if err := tt.Click("Moods"); err != nil {
		t.Fatal(err)
	}
	if got := wait(t, asked, "moods"); got != "moods" {
		t.Fatal(got)
	}
	runPost(t, ch)
	tt.Frame()
	tt.Key(0, ui.KeyBack)
	tt.Frame()
	if a.route.page != pageExplore || exploreID(a.library, a.route.id) != "charts" {
		t.Fatal(a.route)
	}
}

// Playlist filters and sorting operate on a copy, preserving the cloud order.
func TestPlaylistLibraryFilteringAndOrder(t *testing.T) {
	a := editTestApp()
	a.mine[0].Name = "Zebra"
	a.mine[1].Name = "Alpha"
	a.saved[0].Name = "Saved"
	rows := playlistLibraryItems(a.library, 1, "", 1)
	if len(rows) != 2 || rows[0].Name != "Alpha" || a.mine[0].Name != "Zebra" {
		t.Fatal("sort mutated source order")
	}
	rows = playlistLibraryItems(a.library, 2, "sav", 0)
	if len(rows) != 1 || rows[0].ID != "20" {
		t.Fatal(rows)
	}
}

// Library filtering changes what opens; pinning a card does not open or play it.
func TestPlaylistLibraryFilterAndPin(t *testing.T) {
	a := editTestApp()
	a.mineLoaded = true
	a.setPins(a.id, nil)
	a.route = route{page: pagePlaylists, src: "fake"}
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.Click("Saved by Me"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.RightClick("飙升榜"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Pin to Sidebar"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(a.pinsFor(a.library)) != 1 || a.route.page != pagePlaylists || a.pl.Current() != nil {
		t.Fatal("pin changed playback or navigation")
	}
	a.setPins(a.id, nil)
	tt.Frame()
	if err := tt.Click("飙升榜"); err != nil {
		t.Fatal(err)
	}
	if a.route.page != pagePlaylist || a.route.id != "20" {
		t.Fatal(a.route)
	}
}

// Space in the playlist filter is text input, not a play/pause shortcut.
func TestPlaylistSearchSpaceDoesNotTogglePlayback(t *testing.T) {
	a := editTestApp()
	a.mineLoaded = true
	a.route = route{page: pagePlaylists, src: "fake"}
	a.pl.Cue(a.playlists["10"].Tracks)
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.Click("Search Playlists"); err != nil {
		t.Fatal(err)
	}
	tt.Type("Night")
	tt.Key(0, ui.KeySpace)
	tt.Type(" Drive")
	tt.Frame()
	if a.pl.Playing() || a.playlistQuery != "Night Drive" {
		t.Fatalf("query %q, playing %v", a.playlistQuery, a.pl.Playing())
	}
}

// Playlist filtering stays with its source while navigating between sources.
func TestPlaylistFiltersStayWithSource(t *testing.T) {
	a := editTestApp()
	a.playlistFilter = 2
	a.playlistQuery = "saved"
	other := addFake(a, newFakeSource("other"))
	a.activate(other)
	if a.playlistQuery != "" || a.playlistFilter != 0 {
		t.Fatal("filter leaked to another source")
	}
	a.activate(a.libs[0])
	if a.playlistQuery != "saved" || a.playlistFilter != 2 {
		t.Fatal("source lost its filter")
	}
}

// With MM_FRAMES set, renders the grouped sidebar, the library, and Explore at narrow widths.
func TestSidebarLibraryFrames(t *testing.T) {
	dir := framesDir(t)
	a := editTestApp()
	a.mineLoaded = true
	fake(a).info.Name = "YouTube Music"
	fake(a).info.Navigation = []music.NavItem{{Builtin: "home"}, {Title: "Charts", Page: "charts", Group: "explore", Icon: "chart"}, {Title: "New Releases", Page: "new", Group: "explore", Icon: "star"}, {Title: "Moods & Genres", Page: "moods", Group: "explore", Icon: "radio"}, {Builtin: "stations"}, {Builtin: "liked"}, {Builtin: "savedAlbums"}, {Title: "Artists", Page: "artists", Group: "library", Icon: "artist"}}
	a.setPins(a.id, []sidebarPin{playlistPin(a.mine[0]), playlistPin(a.mine[1]), playlistPin(a.saved[0]), {Kind: "artist", ID: "artist", Name: "The Polar Lights"}, {Kind: "album", ID: "album", Name: "Polar Night"}})
	a.route = route{page: pagePlaylists, src: "fake"}
	tt := ui.NewTester(a.view, 1180, 780)
	settled(tt)
	saveFrame(t, dir, "sidebar-library.png", tt.Image())
	narrow := ui.NewTester(a.view, 880, 660)
	settled(narrow)
	saveFrame(t, dir, "sidebar-library-narrow.png", narrow.Image())
	a.browses["charts"] = &browseData{page: &music.Browse{Title: "Charts", Sections: []music.Section{{Title: "Top Playlists", Playlists: a.saved}}}}
	a.route = route{page: pageExplore, src: "fake"}
	tt.Frame()
	settled(tt)
	saveFrame(t, dir, "sidebar-explore.png", tt.Image())
}

// A successful cloud deletion removes its shortcut; a refused deletion keeps it.
func TestDeletedPlaylistPinTracksCloudResult(t *testing.T) {
	for _, refuse := range []bool{false, true} {
		a := editTestApp()
		ch := posts(a)
		fake(a).remove = func(string) error {
			if refuse {
				return music.ErrUnavailable
			}
			return nil
		}
		a.deleting = deleteDialog{open: true, id: "10", name: "通勤"}
		a.deleteChosen()
		runPost(t, ch)
		pinned := a.isPinned(a.library, sidebarPin{Kind: "playlist", ID: "10"})
		if pinned != refuse {
			t.Fatalf("refused %v, pinned %v", refuse, pinned)
		}
	}
}

// Selecting a library sort changes only the displayed ordering.
func TestPlaylistSortControl(t *testing.T) {
	a := editTestApp()
	a.mineLoaded = true
	a.route = route{page: pagePlaylists, src: "fake"}
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.Click("Sort Playlists"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Name Z–A"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.playlistSort != 2 || a.mine[0].ID != "10" {
		t.Fatal("sort did not apply or changed library order")
	}
}
