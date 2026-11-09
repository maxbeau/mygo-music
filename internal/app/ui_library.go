package app

import (
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"
	"mygo-music/internal/music"
)

// Explore contains a source's declared discovery pages, loaded one at a time.
func exploreTabs(lib *library) []music.NavItem {
	var tabs []music.NavItem
	if lib == nil || !lib.can("browse") {
		return tabs
	}
	for _, item := range lib.src.Info().Navigation {
		if item.Group == "explore" && item.Page != "" {
			tabs = append(tabs, item)
		}
	}
	return tabs
}
func exploreID(lib *library, id string) string {
	tabs := exploreTabs(lib)
	for _, tab := range tabs {
		if tab.Page == id {
			return id
		}
	}
	if len(tabs) > 0 {
		return tabs[0].Page
	}
	return ""
}
func (a *app) loadExplore(lib *library, id string, again bool) {
	if id = exploreID(lib, id); id != "" {
		a.loadBrowse(lib, id, again)
	}
}
func (a *app) exploreHeader(c *ui.Context, tabs []music.NavItem, id string) {
	a.heading(c, T("Explore"), "")
	ui.Row(c).Gap(8).Margin(0, 0, 22, 0).Children(func() {
		for _, tab := range tabs {
			selected := tab.Page == id
			bg, fg := a.pal.hover, a.pal.text
			if selected {
				bg, fg = a.pal.accent, a.pal.onAccent
			}
			if pillButton(c.Key(tab.Page), navIcon(tab.Icon), tab.Title, bg, fg).Clicked() {
				a.open(route{page: pageExplore, id: tab.Page})
			}
		}
	})
}
func (a *app) explorePage(c *ui.Context) {
	lib := a.library
	tabs := exploreTabs(lib)
	id := exploreID(lib, a.route.id)
	d := lib.browses[id]
	header := func(c *ui.Context) { a.exploreHeader(c, tabs, id) }
	if id == "" {
		a.centered(c, T("Nothing here yet"), nil)
		return
	}
	if d == nil {
		a.loadExplore(lib, id, false)
		d = lib.browses[id]
	}
	if d.page == nil {
		a.scrollPage(c, func() {
			header(c)
			if d.loading {
				a.playlistCardsSkeleton(c, a.pageShimmer(c), a.columns(c, 156), 156, false)
			} else {
				ui.Text(c, d.err).FontSize(13).TextColor(a.pal.muted)
				a.remedy(c)
				if ui.Button(c, T("Retry")).Clicked() {
					a.loadExplore(lib, id, true)
				}
			}
		})
		return
	}
	a.browseRows(c, d.page, id, header)
}

// playlistLibraryItems returns an owned copy so sorting never changes source order.
func playlistLibraryItems(lib *library, filter int, query string, order int) []music.Playlist {
	var result []music.Playlist
	if filter != 2 {
		result = append(result, lib.mine...)
	}
	if filter != 1 {
		result = append(result, lib.saved...)
	}
	query = strings.ToLower(strings.TrimSpace(query))
	result = slices.DeleteFunc(result, func(pl music.Playlist) bool {
		return query != "" && !strings.Contains(strings.ToLower(pl.Name+" "+pl.Owner), query)
	})
	if order > 0 {
		slices.SortStableFunc(result, func(a, b music.Playlist) int {
			n := strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
			if order == 2 {
				return -n
			}
			return n
		})
	}
	return result
}
func (a *app) playlistsPage(c *ui.Context) {
	lib := a.library
	a.scrollPage(c, func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Grow(1).Children(func() { a.heading(c, T("Playlists"), "") })
			if lib.can("createPlaylist") && pillButton(c.Key("create"), iconPlus, T("New Playlist"), a.pal.accent, a.pal.onAccent).Clicked() {
				a.newPlaylist(nil)
			}
		})
		width, _ := c.Size()
		inner := width - sidebarWidth - 2*pagePad
		wide := inner >= 760
		search := func(width float32) {
			ui.TextInput(c.Key("filter-query"), &a.playlistQuery).Bind(&a.playlistSearchBox).Placeholder(T("Search Playlists")).Label(T("Search Playlists")).Width(width)
		}
		ui.Row(c).Gap(12).AlignItems(ui.Center).Margin(0, 0, 16, 0).Children(func() {
			segmented(c.Key("filter"), &a.playlistFilter, T("All"), T("Created by Me"), T("Saved by Me"))
			ui.Spacer(c)
			if wide {
				search(210)
			}
			labels := []string{T("Default Order"), T("Name A–Z"), T("Name Z–A")}
			value := labels[a.playlistSort]
			if ui.Select(c.Key("sort"), &value, labels).Label(T("Sort Playlists")).Width(145).Changed() {
				a.playlistSort = slices.Index(labels, value)
			}
		})
		if !wide {
			ui.Box(c).Margin(0, 0, 16, 0).Children(func() { search(inner) })
		}
		if lib.signedOut() {
			ui.Text(c, T("Sign in to see your playlists")).FontSize(13).TextColor(a.pal.muted)
			if ui.Button(c, T("Sign In")).Clicked() {
				a.promptSignIn(lib)
			}
			return
		}
		if lib.mineErr != "" {
			ui.Text(c, lib.mineErr).FontSize(13).TextColor(a.pal.muted)
			if ui.Button(c, T("Retry")).Clicked() {
				a.loadMine(lib)
			}
		}
		if lib.mineLoading && len(lib.mine)+len(lib.saved) == 0 {
			a.playlistCardsSkeleton(c, a.pageShimmer(c), a.columns(c, 156), 156, false)
			return
		}
		lists := playlistLibraryItems(lib, a.playlistFilter, a.playlistQuery, a.playlistSort)
		if len(lists) == 0 {
			a.empty(c, T("No matching playlists"))
			return
		}
		ui.Grid(c).Columns(a.columns(c, 156)).GapX(20).GapY(28).Children(func() {
			for _, pl := range lists {
				a.playlistCard(c.Key(pl.ID), &pl)
			}
		})
	})
}
