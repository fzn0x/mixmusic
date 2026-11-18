package main

// Editing Home: Edit home, at its top, lists its shelves in the order Home
// shows them. The user drags one onto another, which it takes the place
// of, or moves it a place up or down with Alt+Up and Alt+Down, as a mix's
// tracks move (mixorder.go's way, for shelves); hides it, or removes one
// they added; brings hidden ones back from the list under them; adds a
// shelf; and Reset puts Home back as it comes. Each change is kept at once
// in settings.json (homelayout.go). Edit tabs arranges the Library's tabs
// with the same rows (librarytabs.go).

import (
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/locales"
)

// homeArrange is Home's own state: whether it shows as the user edits it,
// the shelf the keys moved last, which takes the focus at its new place,
// whether Reset asks, and the items of the shelves made of the library
// and the downloads (homeshelves.go).
type homeArrange struct {
	editing, resetting bool
	focus              string
	memos              map[string]*homeMemo
}

// arrangeDrag is a row of a list the user arranges dragged: the list's
// name, and the row's key.
type arrangeDrag struct{ list, key string }

// arrangement is a list the user arranges, as Edit home's shelves and
// Edit tabs' tabs (librarytabs.go): its name, the arrangement it changes,
// which set keeps, and the rows it shows; focus is the key of the row the
// keys moved last, which takes the focus at its new place.
type arrangement struct {
	list  string
	full  []settings.Arranged
	shown []settings.Arranged
	set   func(layout []settings.Arranged)
	focus *string
}

// setHomeShelves keeps layout as Home's arrangement.
func (s *shell) setHomeShelves(layout []settings.HomeShelf) {
	if layout == nil {
		layout = []settings.HomeShelf{}
	}
	s.set.HomeShelves = layout
	s.save()
}

// homeArrangement returns the arrangement an edit of Home starts from,
// with every shelf Home shows now in its place (arranged), and whether
// the edit lists the shelf of a key: one Home shows now, one the user
// added, or one whose service still loads or failed, which may still
// have it. A shelf of a service that answered with others is gone.
func homeArrangement(layout []settings.HomeShelf, groups []*homeGroup) (full []settings.HomeShelf, listed func(key string) bool) {
	var rows []homeRow
	shows := map[string]bool{}
	for _, g := range groups {
		if g.added {
			rows = append(rows, homeRow{key: g.key, title: g.title})
			shows[g.key] = true
			continue
		}
		if !g.done {
			continue
		}
		for _, sh := range g.shelves {
			rows = append(rows, homeRow{key: sh.key, title: sh.title})
			shows[sh.key] = true
		}
	}
	// service is the group of the service of key, nil for the user's own.
	service := func(key string) *homeGroup {
		if g := groupOf(groups, key); g != nil && g.source != "" && !g.added {
			return g
		}
		return nil
	}
	gone := func(key string) bool {
		g := service(key)
		return g != nil && g.done && g.err == nil && len(g.shelves) > 0
	}
	listed = func(key string) bool {
		if shows[key] {
			return true
		}
		g := service(key)
		return g != nil && (!g.done || g.err != nil)
	}
	return arranged(layout, rows, gone), listed
}

// homeSourceLabel names where the shelves of source come from, as Home's
// pivots do: "Yours" for the user's own.
func homeSourceLabel(source media.Source) string {
	for _, f := range homeFilters {
		if f.source == source {
			return f.label()
		}
	}
	return homeFilters[1].label()
}

// homeEditor is Home as the user edits it, as the package's comment
// tells.
func (s *shell) homeEditor(c *ui.Context) {
	k := &s.lk
	// The menu of Add a shelf lists what was fetched as it opened.
	s.fetchShelfChoices()
	layout := s.set.HomeShelves
	groups := s.homeGroups(layout)
	full, listed := homeArrangement(layout, groups)
	var shown, hidden []settings.HomeShelf
	for _, e := range full {
		switch {
		case e.Hidden:
			hidden = append(hidden, e)
		case listed(e.Key):
			shown = append(shown, e)
		}
	}
	ui.Row(c).Gap(8).Wrap().Margin(0, 0, 16).Children(func() {
		if s.button(c, "home-done", locales.HomeEditDone(), true).Clicked() {
			s.home.editing = false
		}
		s.button(c, "home-add", locales.HomeEditAddShelf(), false).Menu(func(m *ui.Menu) { s.addShelfMenu(m, full) })
		if s.button(c, "home-reset", locales.HomeEditReset(), false).Disabled(len(layout) == 0).Clicked() {
			s.home.resetting = true
		}
	})
	ui.Text(c, locales.HomeEditHint()).
		FontSize(k.bodySize()).TextColor(k.muted).MaxWidth(640).Margin(0, 0, 24)
	a := arrangement{list: "home", full: full, shown: shown, set: s.setHomeShelves, focus: &s.home.focus}
	ui.Column(c.Key("home-shelves")).Gap(4).Margin(0, 0, k.sections()).Children(func() {
		for i, e := range shown {
			s.homeEditRow(c, a, i, e)
		}
	})
	for _, g := range groups {
		if !g.done && !g.added {
			s.loading(c)
			break
		}
	}
	if len(hidden) > 0 {
		s.sectionTitle(c, locales.HomeEditHidden())
		ui.Column(c.Key("home-hidden")).Gap(4).Children(func() {
			for _, e := range hidden {
				s.arrangeHiddenRow(c, a, e, homeSourceLabel(keySource(e.Key)))
			}
		})
	}
	if s.confirm(c, "home-reset", &s.home.resetting, locales.HomeEditReset(),
		locales.HomeEditResetBody(), locales.HomeEditResetButton()) {
		s.setHomeShelves(nil)
	}
}

// homeEditRow is the row of the shelf e, at i of the shelves a shows,
// which removes a shelf the user added, and hides the others.
func (s *shell) homeEditRow(c *ui.Context, a arrangement, i int, e settings.HomeShelf) {
	from := homeSourceLabel(keySource(e.Key))
	s.arrangeRow(c, a, i, e, locales.HomeEditRowLabel(e.Title, from), from, func() {
		if addedKind(e.Key) {
			if s.iconButton(c, "home-remove-"+e.Key, "trash", locales.HomeEditRemove(e.Title), false, 16).Clicked() {
				a.set(withoutShelf(a.full, e.Key))
			}
		} else if s.iconButton(c, "home-hide-"+e.Key, "close", locales.HomeEditHide(e.Title), false, 16).Clicked() {
			a.set(withHidden(a.full, e.Key, true))
		}
	})
}

// arrangeRow is the row of e, at i of the rows a shows, which drags onto
// another and moves with Alt+Up and Alt+Down while it has the focus: label
// is what screen readers call it, sub what shows under its title, if
// anything, and end builds its buttons.
func (s *shell) arrangeRow(c *ui.Context, a arrangement, i int, e settings.Arranged, label, sub string, end func()) {
	k := &s.lk
	row := ui.Row(c.Key(a.list + "-row-" + e.Key)).Height(56).PaddingStart(16).PaddingEnd(8).Gap(12).Radius(k.radiusSm).
		Background(k.surface2).Label(label).Focusable().Cursor(ui.CursorGrab)
	if row.Hovered() {
		row.Background(k.surface2.Mix(k.text, 0.06))
	}
	row.Drag(arrangeDrag{a.list, e.Key})
	if row.Dragging() {
		row.Opacity(0.4)
	}
	if d, over := ui.DragOver[arrangeDrag](row); over && d.list == a.list && d.key != e.Key {
		// The accent's line shows where it goes: below, from above, and
		// above, from below.
		if shelfAt(a.shown, d.key) < i {
			row.InsetShadow(0, -2, 0, 0, k.accent)
		} else {
			row.InsetShadow(0, 2, 0, 0, k.accent)
		}
	}
	if d, ok := ui.Drop[arrangeDrag](row); ok && d.list == a.list && d.key != e.Key {
		a.set(movedOnto(a.full, d.key, e.Key))
	}
	step := func(to int) {
		if to >= 0 && to < len(a.shown) {
			a.set(movedOnto(a.full, e.Key, a.shown[to].Key))
			*a.focus = e.Key
		}
	}
	row.OnShortcut(ui.Alt, ui.KeyUp, func() { step(i - 1) })
	row.OnShortcut(ui.Alt, ui.KeyDown, func() { step(i + 1) })
	if *a.focus == e.Key {
		*a.focus = ""
		row.Focus().ScrollIntoView()
	}
	row.Children(func() {
		s.grip(c)
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, e.Title).FontSize(k.bodySize()).SingleLine().MinWidth(0)
			if sub != "" {
				ui.Text(c, sub).FontSize(k.smallSize()).TextColor(k.muted).SingleLine()
			}
		})
		end()
	})
}

// arrangeHiddenRow is the row of e, which the user hid of the list a, and
// which Show brings back where it was; sub shows under its title, if
// anything.
func (s *shell) arrangeHiddenRow(c *ui.Context, a arrangement, e settings.Arranged, sub string) {
	k := &s.lk
	ui.Row(c.Key(a.list + "-hidden-" + e.Key)).Height(56).PaddingStart(16).PaddingEnd(8).Gap(12).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, e.Title).FontSize(k.bodySize()).TextColor(k.muted).SingleLine().MinWidth(0)
			if sub != "" {
				ui.Text(c, sub).FontSize(k.smallSize()).TextColor(k.muted).SingleLine()
			}
		})
		if s.button(c, a.list+"-show-"+e.Key, locales.HomeEditShow(), false).Label(locales.HomeEditShowTitle(e.Title)).Clicked() {
			a.set(withHidden(a.full, e.Key, false))
		}
	})
}

// grip is the dots that tell a row drags.
func (s *shell) grip(c *ui.Context) {
	ink := s.lk.muted
	ui.Box(c).Size(10, 16).Shrink(0).PassThrough().Draw(func(p *ui.Painter, r ui.Rect) {
		for row := range 3 {
			for col := range 2 {
				p.Fill(ui.Rect{X: r.X + 1 + float32(col)*5, Y: r.Y + 2 + float32(row)*5, W: 3, H: 3}, ink, 1.5)
			}
		}
	})
}

// fetchShelfChoices fetches what Add a shelf offers of the services the
// user signed in to: their playlists, and the artists they follow there.
func (s *shell) fetchShelfChoices() {
	accounts := s.accounts()
	col := s.svc.Collection
	if accounts.YouTube.SignedIn {
		s.youtubePlaylists()
		s.youtubeArtists()
	}
	if accounts.Spotify.SignedIn {
		fetch(s, "spotify:playlists", col.SpotifyPlaylists)
		fetch(s, "spotify:artists", col.SpotifyArtists)
	}
}

// addShelfMenu is the menu of Add a shelf: one of the user's mixes, a
// playlist of their library on a service, an artist's new releases, what
// they played lately or most this month, or their downloads. Each is
// checked while Home shows it, and choosing it again takes it off; one
// added goes first, where the user sees it.
func (s *shell) addShelfMenu(m *ui.Menu, full []settings.HomeShelf) {
	item := func(m *ui.Menu, key, title string) {
		i := shelfAt(full, key)
		on := i >= 0 && !full[i].Hidden
		if !m.Item(title).Checked(on).Chosen() {
			return
		}
		switch {
		case !on:
			s.setHomeShelves(withAdded(full, key, title))
		case addedKind(key):
			s.setHomeShelves(withoutShelf(full, key))
		default:
			s.setHomeShelves(withHidden(full, key, true))
		}
	}
	m.Submenu(locales.HomeEditOneMix(), func(m *ui.Menu) {
		mixes := s.svc.Collection.Playlists()
		if len(mixes) == 0 {
			m.Item(locales.HomeEditNoMixes()).Disabled(true)
		}
		for _, p := range mixes {
			item(m, homePlaylist+p.ID, p.Title)
		}
	})
	accounts := s.accounts()
	playlists := func(title string, source media.Source, l *loaded[[]media.Playlist]) {
		m.Submenu(title, func(m *ui.Menu) {
			switch {
			case !l.done:
				m.Item(locales.HomeEditLoadingPlaylists()).Disabled(true)
			case !listCame(l.err):
				m.Item(locales.HomeEditLoadFailed()).Disabled(true)
			case len(l.value) == 0:
				m.Item(locales.HomeEditNoPlaylists()).Disabled(true)
			}
			hidden := 0
			for _, p := range l.value {
				if tracksHidden(p) {
					hidden++
					continue
				}
				item(m, homePlaylist+p.ID, p.Title)
			}
			if hidden > 0 {
				m.Item(locales.HomeEditSpotifyHidden()).Disabled(true)
			}
			// Of a list that came in part, those that came, and how many.
			s.shortListItem(m, string(source)+":playlists", "playlists", source, l.err)
		})
	}
	if accounts.YouTube.SignedIn {
		playlists(locales.HomeEditYouTubePlaylist(), media.SourceYouTube, s.youtubePlaylists())
	}
	if accounts.Spotify.SignedIn {
		playlists(locales.HomeEditSpotifyPlaylist(), media.SourceSpotify, fetch(s, "spotify:playlists", s.svc.Collection.SpotifyPlaylists))
	}
	m.Submenu(locales.HomeEditArtistReleases(), func(m *ui.Menu) {
		artists := s.followedEverywhere()
		if len(artists) == 0 {
			m.Item(locales.HomeEditFollowArtist()).Disabled(true)
		}
		for _, a := range artists {
			item(m, homeArtist+a.ID, locales.HomeEditNewFrom(a.Name))
		}
	})
	m.Separator()
	item(m, homeRecent, locales.HomeEditRecent())
	item(m, homeTopMonth, locales.HomeEditMostPlayed())
	item(m, homeDownloads, locales.RailDownloads())
}

// followedEverywhere returns the artists the user follows, in MixMusic,
// then on the services they signed in to, as those were read, whole or in
// part, each once.
func (s *shell) followedEverywhere() []media.Artist {
	artists := s.svc.Collection.FollowedArtists()
	seen := map[string]bool{}
	for _, a := range artists {
		seen[a.ID] = true
	}
	for _, key := range []string{"youtube:artists", "spotify:artists"} {
		if l, ok := s.loads[key].(*loaded[[]media.Artist]); ok && l.done && listCame(l.err) && s.signedIn(keySource(key)) {
			for _, a := range l.value {
				if !seen[a.ID] {
					seen[a.ID] = true
					artists = append(artists, a)
				}
			}
		}
	}
	return artists
}
