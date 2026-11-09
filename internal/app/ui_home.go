package app

import (
	"math/rand/v2"
	"strconv"

	"github.com/egoist/mygo/ui"
	"mygo-music/internal/music"
)

// sectionEntries keeps mixed shelves in service order, with a legacy fallback.
func sectionEntries(s music.Section) []music.BrowseEntry {
	if len(s.Entries) > 0 {
		return s.Entries
	}
	var out []music.BrowseEntry
	for i := range s.Tracks {
		out = append(out, music.BrowseEntry{Track: &s.Tracks[i]})
	}
	for i := range s.Albums {
		out = append(out, music.BrowseEntry{Album: &s.Albums[i]})
	}
	for i := range s.Artists {
		out = append(out, music.BrowseEntry{Artist: &s.Artists[i]})
	}
	for i := range s.Playlists {
		out = append(out, music.BrowseEntry{Playlist: &s.Playlists[i]})
	}
	for i := range s.Stations {
		out = append(out, music.BrowseEntry{Station: &s.Stations[i]})
	}
	for i := range s.Links {
		out = append(out, music.BrowseEntry{Link: &s.Links[i]})
	}
	return out
}
func homeTracks(page *music.Browse) []music.Track {
	if page == nil {
		return nil
	}
	var tracks []music.Track
	seen := map[string]bool{}
	for _, s := range page.Sections {
		for _, e := range sectionEntries(s) {
			if e.Track != nil && !e.Track.Unavailable && !seen[e.Track.ID] {
				tracks = append(tracks, *e.Track)
				seen[e.Track.ID] = true
			}
		}
	}
	return tracks
}
func homeMixes(page *music.Browse) []music.BrowseEntry {
	if page == nil {
		return nil
	}
	var entries []music.BrowseEntry
	seen := map[string]bool{}
	for _, s := range page.Sections {
		for _, e := range sectionEntries(s) {
			key := ""
			if e.Station != nil {
				key = "radio:" + e.Station.ID
			}
			if e.Playlist != nil && !e.Playlist.Mine {
				key = "playlist:" + e.Playlist.ID
			}
			if key != "" && !seen[key] {
				entries = append(entries, e)
				seen[key] = true
			}
		}
	}
	return entries
}

// discoveryHome bounds each shelf to one card row. Full contents remain
// reachable from See All, so a large first shelf cannot bury other choices.
func (a *app) discoveryHome(c *ui.Context) {
	lib := a.library
	quick := homeTracks(lib.home)
	a.loadHomeRelated(lib)
	mixes := homeMixes(lib.home)
	type block struct{ kind, section int }
	blocks := []block{{0, 0}}
	if len(a.pinsFor(lib)) > 0 {
		blocks = append(blocks, block{1, 0})
	}
	if len(quick) > 0 {
		blocks = append(blocks, block{2, 0})
	}
	if len(mixes) > 0 {
		blocks = append(blocks, block{3, 0})
	}
	if lib.homeRelated != nil && lib.homeRelated.page != nil {
		blocks = append(blocks, block{4, 0})
	}
	for i, s := range lib.home.Sections {
		if len(sectionEntries(s)) > 0 {
			blocks = append(blocks, block{5, i})
		}
	}
	top, bottom := a.insets(c)
	// Offscreen rows are not built, including their edge marks. Assume content
	// continues under both bars until a visible start/end mark says otherwise.
	a.underTop, a.underBottom = true, true
	ui.List(c.Key(a.route), &a.trackList, len(blocks), func(i int) {
		if i == 0 {
			a.startMark(c)
		}
		if i == len(blocks)-1 {
			defer a.endMark(c)
		}
		ui.Column(c.Key(i)).PaddingX(12).Children(func() {
			b := blocks[i]
			switch b.kind {
			case 0:
				a.homeDashboardHeader(c, quick)
			case 1:
				a.homeShortcuts(c)
			case 2:
				a.homeQuickPicks(c, quick)
			case 3:
				a.homeShelf(c, T("Mixed for You"), mixes, func() { a.openHomeCollection("mixes") })
			case 4:
				var entries []music.BrowseEntry
				for _, s := range lib.homeRelated.page.Sections {
					entries = append(entries, sectionEntries(s)...)
				}
				if len(entries) > 0 {
					a.homeShelf(c, Tf("Because you listened to %s", lib.homeRelatedName), entries, func() { a.openRelated(lib, lib.homeRelatedSeed) })
				}
			case 5:
				sec := &lib.home.Sections[b.section]
				if sec.Title == "" && len(sec.Links) > 0 {
					ui.Row(c).Gap(10).Margin(0, 0, 16, 0).Children(func() {
						for _, link := range sec.Links {
							if pillButton(c.Key(link.Page), iconForward, link.Title, a.pal.hover, a.pal.text).Clicked() {
								a.open(route{page: pageBrowse, id: link.Page})
							}
						}
					})
					break
				}
				a.homeShelf(c, sec.Title, sectionEntries(*sec), func() {
					a.open(route{page: pageSection, id: "", query: strconv.Itoa(b.section)})
				})
			}
		})
	}).Grow(1).Padding(top+pageGap, pagePad-12, bottom+32, pagePad-12).ScrollbarInsets(top, 0, bottom, 0)
}
func (a *app) homeDashboardHeader(c *ui.Context, quick []music.Track) {
	ui.Row(c).AlignItems(ui.Center).Gap(18).Margin(0, 0, 12, 0).Children(func() {
		ui.Box(c).Grow(1).Shrink(1).MinWidth(0).Children(func() { a.homeHello(c, c.Now()) })
		if len(quick) > 0 && a.can("radio") && a.can("stationNext") {
			if pillButton(c.Key("surprise"), iconShuffle, T("Surprise Me"), a.pal.accent, a.pal.onAccent).Clicked() {
				a.startSongRadio(quick[rand.IntN(len(quick))])
			}
		}
	})
}
func (a *app) homeBlockTitle(c *ui.Context, title string, more func()) {
	ui.Row(c).AlignItems(ui.Baseline).Gap(12).Margin(0, 0, 12, 0).Children(func() {
		ui.Text(c, title).FontSize(18).FontWeight(700).MaxLines(1).Grow(1).Shrink(1).MinWidth(0)
		if more != nil {
			button := ui.ButtonBase(c.Key("all")).Padding(3, 0).FocusRing(false).Label(T("See All"))
			button.Children(func() { ui.Text(c, T("See All")).FontSize(12).TextColor(a.pal.muted) })
			if button.Clicked() {
				more()
			}
		}
	})
}
func (a *app) homeShortcuts(c *ui.Context) {
	pins := a.pinsFor(a.library)
	if len(pins) > 4 {
		pins = pins[:4]
	}
	a.homeBlockTitle(c, T("Favorites at Hand"), nil)
	cols := a.columns(c, 180)
	cols = min(cols, len(pins))
	ui.Grid(c).Columns(cols).GapX(10).GapY(8).Margin(0, 0, 22, 0).Children(func() {
		for _, pin := range pins {
			b := ui.ButtonBase(c.Key(pin.key())).Height(56).Padding(8).Gap(10).Radius(10).AlignItems(ui.Center).Justify(ui.Start).FocusRing(false).Label(pin.Name)
			fill := a.pal.hover
			if b.Hovered() {
				fill = a.pal.press
			}
			b.Background(fill)
			b.Children(func() {
				a.cover(c, pin.ImageURL, 80, 36, 6)
				ui.Text(c, pin.Name).FontSize(12.5).FontWeight(600).SingleLine().Shrink(1).MinWidth(0)
			})
			if b.Clicked() {
				a.open(pin.route(a.id))
			}
		}
	})
}
func (a *app) homeQuickPicks(c *ui.Context, tracks []music.Track) {
	a.homeBlockTitle(c, T("Quick Picks"), func() { a.openHomeCollection("quick") })
	limit := min(len(tracks), 8)
	cols := 2
	w, _ := c.Size()
	if w-sidebarWidth-2*pagePad < 540 {
		cols = 1
		limit = min(limit, 4)
	}
	rows := (limit + cols - 1) / cols
	ui.Grid(c).Columns(cols).GapX(20).GapY(0).Margin(0, 0, 22, 0).Children(func() {
		for col := 0; col < cols; col++ {
			ui.Column(c.Key(col)).Children(func() {
				for row := 0; row < rows; row++ {
					at := col*rows + row
					if at < limit {
						a.homeQuickRow(c.Key(at), tracks, at)
					}
				}
			})
		}
	})
}
func (a *app) homeShelf(c *ui.Context, title string, entries []music.BrowseEntry, more func()) {
	if len(entries) == 0 {
		return
	}
	a.homeBlockTitle(c, title, more)
	cols := a.columns(c, 156)
	ui.Grid(c).Columns(cols).GapX(20).Margin(0, 0, 24, 0).Children(func() {
		for i, e := range entries[:min(len(entries), cols)] {
			a.browseEntryCard(c.Key(i), e)
		}
	})
}
func (a *app) browseEntryCard(c *ui.Context, e music.BrowseEntry) {
	switch {
	case e.Track != nil:
		t := e.Track
		ui.Box(c).ContextMenu(a.songItems(t, "")).Children(func() {
			play := func() { a.pl.PlayTrack([]music.Track{*t}, 0) }
			if a.coverCard(c, t.Name, t.Album.ImageURL, t.ArtistNames(), play) {
				play()
			}
		})
	case e.Album != nil:
		a.albumCard(c, e.Album)
	case e.Artist != nil:
		a.artistCard(c, e.Artist)
	case e.Playlist != nil:
		a.playlistCard(c, e.Playlist)
	case e.Station != nil:
		a.stationCard(c, e.Station)
	case e.Link != nil:
		l := e.Link
		if a.coverCard(c, l.Title, l.ImageURL, l.Subtitle, nil) {
			a.open(route{page: pageBrowse, id: l.Page})
		}
	}
}

// Collections are normal Browse pages so back navigation and song menus work.
func (a *app) openHomeCollection(kind string) {
	const id = "@home:"
	if a.home == nil {
		return
	}
	title := T("Quick Picks")
	entries := homeMixes(a.home)
	if kind == "mixes" {
		title = T("Mixed for You")
	} else {
		entries = nil
		for _, track := range homeTracks(a.home) {
			copy := track
			entries = append(entries, music.BrowseEntry{Track: &copy})
		}
	}
	a.browses[id+kind] = &browseData{page: &music.Browse{Title: title, Sections: []music.Section{{Entries: entries}}}}
	// These pages come from the in-memory home, not the source's browse API.
	a.open(route{page: pageHomeCollection, id: id + kind})
}
func (a *app) loadHomeRelated(lib *library) {
	if lib.homeRelated != nil || !lib.can("related") {
		return
	}
	src, ok := lib.src.(music.RelatedSource)
	if !ok {
		return
	}
	track := a.pl.Current()
	if track == nil || track.Source != lib.id || track.ShowID != "" {
		return
	}
	d := &browseData{loading: true}
	lib.homeRelated = d
	lib.homeRelatedSeed, lib.homeRelatedName = track.ID, track.Name
	id := track.ID
	go func() {
		page, err := src.Related(id)
		a.post(func() {
			if lib.homeRelated != d {
				return
			}
			d.loading = false
			if err == nil {
				d.page = page
			}
		})
	}()
}

// A compact song row has no album column or row number competing for width.
func (a *app) homeQuickRow(c *ui.Context, tracks []music.Track, at int) {
	track := &tracks[at]
	b := ui.ButtonBase(c).Height(48).PaddingX(8).Gap(10).Radius(9).AlignItems(ui.Center).FocusRing(false).Label(track.Name).ContextMenu(a.songItems(track, ""))
	if b.Hovered() {
		b.Background(a.pal.hover)
	}
	if sameTrack(a.pl.Current(), track) {
		b.Background(a.pal.accent.Alpha(0.09))
	}
	b.Children(func() {
		a.cover(c, track.Album.ImageURL, 96, 34, 6)
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(3).Children(func() {
			ui.Text(c, track.Name).FontSize(13).FontWeight(600).SingleLine()
			ui.Text(c, track.ArtistNames()).FontSize(11.5).TextColor(a.pal.muted).SingleLine()
		})
		ui.Text(c, clock(track.Length())).FontSize(11.5).FontFeatures("tnum").TextColor(a.pal.faint).Width(38).TextAlign(ui.End)
	})
	if b.Clicked() {
		a.pl.PlayTrack(tracks, at)
	}
}
