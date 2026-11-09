package app

import (
	"slices"

	"github.com/egoist/mygo/ui"
	"mygo-music/internal/music"
)

// sidebarPin is a local shortcut, not a subscription or cloud-library edit.
type sidebarPin struct {
	Kind     string `json:"kind"`
	ID       string `json:"id"`
	Name     string `json:"name"`
	ImageURL string `json:"imageUrl,omitempty"`
}

func (p sidebarPin) key() string { return p.Kind + ":" + p.ID }
func (p sidebarPin) route(source string) route {
	kind := pagePlaylist
	if p.Kind == "artist" {
		kind = pageArtist
	} else if p.Kind == "album" {
		kind = pageAlbum
	}
	return route{page: kind, src: source, id: p.ID}
}
func playlistPin(p music.Playlist) sidebarPin {
	return sidebarPin{"playlist", p.ID, p.Name, p.ImageURL}
}
func albumPin(p music.Album) sidebarPin   { return sidebarPin{"album", p.ID, p.Name, p.ImageURL} }
func artistPin(p music.Artist) sidebarPin { return sidebarPin{"artist", p.ID, p.Name, p.ImageURL} }

func starterPins(lib *library) []sidebarPin {
	var pins []sidebarPin
	for _, pl := range append(slices.Clone(lib.mine), lib.saved...) {
		if len(pins) == 6 {
			break
		}
		pins = append(pins, playlistPin(pl))
	}
	return pins
}
func (a *app) seedPins(lib *library) {
	if len(lib.mine)+len(lib.saved) == 0 {
		return
	}
	if _, exists := a.settings.SidebarPins[lib.id]; exists {
		return
	}
	a.setPins(lib.id, starterPins(lib))
}
func (a *app) pinsFor(lib *library) []sidebarPin {
	pins, exists := a.settings.SidebarPins[lib.id]
	if !exists {
		pins = starterPins(lib)
	}
	pins = slices.Clone(pins)
	for i, pin := range pins {
		if pin.Kind == "playlist" {
			for _, pl := range append(slices.Clone(lib.mine), lib.saved...) {
				if pl.ID == pin.ID {
					pins[i] = playlistPin(pl)
					break
				}
			}
		} else if pin.Kind == "artist" {
			if d := lib.artists[pin.ID]; d != nil && d.page != nil {
				pins[i] = artistPin(d.page.Artist)
			}
		} else if pin.Kind == "album" {
			if d := lib.albums[pin.ID]; d != nil && d.album != nil {
				pins[i] = albumPin(d.album.Album)
			}
		}
	}
	return pins
}
func (a *app) setPins(source string, pins []sidebarPin) {
	if a.settings.SidebarPins == nil {
		a.settings.SidebarPins = map[string][]sidebarPin{}
	}
	if pins == nil {
		pins = []sidebarPin{}
	}
	a.settings.SidebarPins[source] = slices.Clone(pins)
	a.settings.save()
}
func (a *app) isPinned(lib *library, pin sidebarPin) bool {
	return slices.ContainsFunc(a.pinsFor(lib), func(p sidebarPin) bool { return p.key() == pin.key() })
}
func (a *app) togglePin(lib *library, pin sidebarPin) {
	if pin.ID == "" || pin.Name == "" {
		return
	}
	pins := a.pinsFor(lib)
	at := slices.IndexFunc(pins, func(p sidebarPin) bool { return p.key() == pin.key() })
	if at >= 0 {
		pins = slices.Delete(pins, at, at+1)
	} else {
		pins = append(pins, pin)
	}
	a.setPins(lib.id, pins)
}
func (a *app) movePin(lib *library, from, to string) {
	pins := a.pinsFor(lib)
	first := slices.IndexFunc(pins, func(p sidebarPin) bool { return p.key() == from })
	last := slices.IndexFunc(pins, func(p sidebarPin) bool { return p.key() == to })
	if first < 0 || last < 0 || first == last {
		return
	}
	pin := pins[first]
	pins = slices.Delete(pins, first, first+1)
	pins = slices.Insert(pins, last, pin)
	a.setPins(lib.id, pins)
}
func (a *app) pinMenu(m *ui.Menu, pin sidebarPin) {
	label := T("Pin to Sidebar")
	if a.isPinned(a.library, pin) {
		label = T("Unpin from Sidebar")
	}
	if m.Item(label).Chosen() {
		a.togglePin(a.library, pin)
	}
}
func (a *app) pinButton(c *ui.Context, pin sidebarPin) {
	label := T("Pin to Sidebar")
	color := a.pal.muted
	if a.isPinned(a.library, pin) {
		label = T("Unpin from Sidebar")
		color = a.pal.accent
	}
	if iconButton(c, iconStar, 17, color, label).Clicked() {
		a.togglePin(a.library, pin)
	}
}

func (a *app) groupedNavigation(c *ui.Context) {
	items := a.sidebarItems()
	for _, group := range []string{"discover", "library"} {
		title := T("Discover")
		if group == "library" {
			title = T("Library")
		}
		var entries []sidebarItem
		for _, item := range items {
			library := item.group == "library"
			if library == (group == "library") {
				entries = append(entries, item)
			}
		}
		if len(entries) == 0 {
			continue
		}
		ui.Text(c.Key(group), title).FontSize(11).FontWeight(600).TextColor(a.pal.faint).Margin(10, 10, 4, 0)
		for _, item := range entries {
			if item.route.page == pagePlaylists && a.can("createPlaylist") {
				ui.Row(c.Key("library-playlists")).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Grow(1).MinWidth(0).Children(func() { a.navItem(c, item.icon, item.label, item.route) })
					if iconButton(c.Key("new"), iconPlus, 13, a.pal.faint, T("New Playlist")).Size(24, 28).Clicked() {
						a.newPlaylist(nil)
					}
				})
			} else {
				a.navItem(c, item.icon, item.label, item.route)
			}
		}
	}
}

func (a *app) pinnedSidebar(c *ui.Context) {
	lib := a.library
	pins := a.pinsFor(lib)
	ui.Row(c.Key("pinned-header")).AlignItems(ui.Center).Margin(14, 8, 4, 2).Children(func() {
		ui.Text(c, T("Pinned")).FontSize(11).FontWeight(600).TextColor(a.pal.faint).Grow(1)
		plus := iconButton(c.Key("pin-add"), iconPlus, 13, a.pal.faint, T("Add a Pin")).Size(24, 24)
		plus.Menu(func(m *ui.Menu) {
			if current, ok := a.currentPin(); ok {
				if m.Item(T("Pin Current Page")).Chosen() {
					if !a.isPinned(lib, current) {
						a.togglePin(lib, current)
					}
				}
				m.Separator()
			}
			for _, pl := range append(slices.Clone(lib.mine), lib.saved...) {
				pin := playlistPin(pl)
				if m.Item(pl.Name).Checked(a.isPinned(lib, pin)).Chosen() {
					a.togglePin(lib, pin)
				}
			}
			if len(lib.mine)+len(lib.saved) == 0 {
				m.Item(T("Pin a playlist, album or artist from its page")).Disabled(true)
			}
		})
	})
	if len(pins) == 0 {
		ui.Text(c, T("Pin favorites from their pages")).FontSize(11.5).TextColor(a.pal.faint).Padding(6, 10).MaxLines(2)
		return
	}
	limit := len(pins)
	if !a.pinsExpanded {
		limit = min(limit, 6)
	}
	for _, pin := range pins[:limit] {
		b := ui.ButtonBase(c.Key(pin.key())).Height(34).PaddingX(8).Gap(9).Radius(8).AlignItems(ui.Center).Justify(ui.Start).FocusRing(false).Label(pin.Name).Drag(pin)
		if a.route == pin.route(lib.id) && !a.nowPlaying {
			b.Background(a.pal.press)
		} else if b.Hovered() {
			b.Background(a.pal.hover)
		}
		if over, ok := ui.DragOver[sidebarPin](b); ok && over.key() != pin.key() {
			b.Border(1, a.pal.accent)
		}
		if dropped, ok := ui.Drop[sidebarPin](b); ok {
			a.movePin(lib, dropped.key(), pin.key())
		}
		b.Children(func() {
			a.cover(c, pin.ImageURL, 80, 22, 5)
			ui.Text(c, pin.Name).FontSize(13).SingleLine().Shrink(1).MinWidth(0)
		})
		b.ContextMenu(func(m *ui.Menu) {
			a.pinMenu(m, pin)
			at := slices.IndexFunc(pins, func(p sidebarPin) bool { return p.key() == pin.key() })
			if at > 0 && m.Item(T("Move Up")).Chosen() {
				a.movePin(lib, pin.key(), pins[at-1].key())
			}
			if at+1 < len(pins) && m.Item(T("Move Down")).Chosen() {
				a.movePin(lib, pin.key(), pins[at+1].key())
			}
			if pin.Kind == "playlist" {
				for _, pl := range append(slices.Clone(lib.mine), lib.saved...) {
					if pl.ID == pin.ID {
						m.Separator()
						a.playlistActions(m, pl)
						break
					}
				}
			}
		})
		if b.Clicked() {
			a.open(pin.route(lib.id))
		}
	}
	if len(pins) > 6 {
		label := T("Show More")
		if a.pinsExpanded {
			label = T("Show Less")
		}
		if ui.ButtonBase(c.Key("more-pins")).Height(28).PaddingX(10).Label(label).Children(func() { ui.Text(c, label).FontSize(12).TextColor(a.pal.muted) }).Clicked() {
			a.pinsExpanded = !a.pinsExpanded
		}
	}
}
func (a *app) currentPin() (sidebarPin, bool) {
	switch a.route.page {
	case pagePlaylist:
		if pl := a.playlists[a.route.id]; pl != nil {
			return playlistPin(*pl), true
		}
	case pageAlbum:
		if d := a.albums[a.route.id]; d != nil && d.album != nil {
			return albumPin(d.album.Album), true
		}
	case pageArtist:
		if d := a.artists[a.route.id]; d != nil && d.page != nil {
			return artistPin(d.page.Artist), true
		}
	}
	return sidebarPin{}, false
}
func (a *app) playlistActions(m *ui.Menu, pl music.Playlist) {
	if mine := a.editable(pl.ID); mine != nil {
		a.playlistMenu(m, mine)
	} else if a.can("savePlaylist") && a.isSaved(&pl) {
		if m.Item(T("Remove from Saved")).Chosen() {
			a.toggleSaved(pl)
		}
	}
}
func (a *app) playlistCardMenu(m *ui.Menu, pl music.Playlist) {
	a.pinMenu(m, playlistPin(pl))
	m.Separator()
	a.playlistActions(m, pl)
}
