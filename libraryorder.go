package main

// Ordering the library (internal/library/order.go): the user's mixes list
// in the order the user puts them, or in a sort of theirs, which Library ›
// Mixes chooses and every list of mixes follows. Their search keeps the
// mixes whose names hold its words. The user drags a mix onto another,
// which it takes the place of, or moves the one with the focus with Alt
// and the arrows, while the mixes show in their own order, searched or
// not: a move made while searching keeps the place of every mix the
// search hides. What the user pins leads each list it's in, with a pin
// mark, and pins move among themselves the same way, in any sort. The
// playlists of the services search and sort alike, and what the user
// hides of them waits under Hidden, to show again.

import (
	"cmp"
	"slices"
	"strings"
	"sync"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/library"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/locales"
)

// libraryOrder is how the user searches the lists of their library, and
// the item the keys moved last, which takes the focus at its new place.
type libraryOrder struct {
	mixes string
	side  string
	// playlists are the searches of the services' playlists, by their
	// keys, and views the views made last of them.
	playlists map[string]*string
	views     map[string]*playlistsMemo
	focus     string
	// folders is how the folders of the sidebar's Your mixes open
	// (railfolders.go).
	folders folderState
}

// playlistsMemo is the view made last of a service's playlists: of the
// list of, searched for query, in the sort of sort and descending, with
// the pins and the hidden of the library at version. A library of
// thousands sorts once, not in every frame.
type playlistsMemo struct {
	of         []media.Playlist
	query      string
	sort       string
	descending bool
	version    uint64
	view       []media.Playlist
}

// playlistsQuery is the search of the playlists of the service of key.
func (o *libraryOrder) playlistsQuery(key string) *string {
	if o.playlists == nil {
		o.playlists = map[string]*string{}
	}
	q := o.playlists[key]
	if q == nil {
		q = new(string)
		o.playlists[key] = q
	}
	return q
}

// orderDrag is an item of a list of the library dragged, by the list's
// name and its key.
type orderDrag struct{ list, key string }

// The marks of what the user pins or hides.
func playlistMark(p media.Playlist) library.Mark {
	return library.Mark{ID: p.ID, Kind: library.MarkPlaylist, Title: p.Title}
}

func albumMark(a media.Album) library.Mark {
	return library.Mark{ID: a.ID, Kind: library.MarkAlbum, Title: a.Title}
}

func artistMark(a media.Artist) library.Mark {
	return library.Mark{ID: a.ID, Kind: library.MarkArtist, Title: a.Name}
}

// pinned reports whether the item id is pinned.
func (s *shell) pinned(id string) bool { return s.svc.Collection.PinAt(id) >= 0 }

// pinItem is the item of a menu that pins mark's item to the top of the
// lists it's in, or unpins it.
func (s *shell) pinItem(m *ui.Menu, mark library.Mark) {
	on := !s.pinned(mark.ID)
	label := locales.LibraryUnpin()
	if on {
		label = locales.LibraryPinToTop()
	}
	if m.Item(label).Chosen() {
		s.setPinned(mark, on)
	}
}

// withPin is the menu of a page, base, with the item that pins its
// collection, mark's, or unpins it.
func (s *shell) withPin(base func(m *ui.Menu), mark library.Mark) func(m *ui.Menu) {
	return func(m *ui.Menu) {
		if base != nil {
			base(m)
		}
		s.pinItem(m, mark)
	}
}

// setPinned pins mark's item, or unpins it.
func (s *shell) setPinned(mark library.Mark, on bool) {
	if err := s.svc.Collection.SetPinned(mark, on); err != nil {
		s.notes = append(s.notes, noticeOf(err))
	}
}

// hideItem is the item of a menu of a playlist or an album of a
// service's library that hides it from MixMusic's: nothing changes on the
// service.
func (s *shell) hideItem(m *ui.Menu, mark library.Mark) {
	if m.Item(locales.LibraryHideFromLibrary()).Chosen() {
		s.setHidden(mark, true)
	}
}

// setHidden hides mark's item from the library, or shows it again.
func (s *shell) setHidden(mark library.Mark, on bool) {
	if err := s.svc.Collection.SetHidden(mark, on); err != nil {
		s.notes = append(s.notes, noticeOf(err))
		return
	}
	if on {
		s.notes = append(s.notes, notice(locales.LibraryHiddenNotice(mark.Title)))
	}
}

// pinMark is the mark beside the title of what is pinned.
func (s *shell) pinMark(c *ui.Context) {
	ui.Icon(c, icon("pin")).FontSize(13).TextColor(s.lk.accent).Shrink(0)
}

// pinMenuIcon is the pin mark of the items of menus, made once.
var pinMenuIcon = sync.OnceValue(func() ui.MenuIcon {
	if i, err := mygo.NewMenuIconSVG(icon("pin"), true); err == nil {
		return i
	}
	return nil
})

// orderable lets the user move the item of key, among the items of keys a
// list, named list, shows: dragged onto another of them, which it takes
// the place of, or with Alt and the arrows while it has the focus, which
// stays on it. can tells which item may go onto which, and move moves it.
// tiles tells a list of tiles, which lines wrap, from a column of rows.
func (s *shell) orderable(el ui.Element, list, key string, keys []string, tiles bool, can func(from, onto string) bool, move func(from, onto string)) {
	i := slices.Index(keys, key)
	el.Drag(orderDrag{list, key})
	if el.Dragging() {
		el.Opacity(0.4)
	}
	if d, over := ui.DragOver[orderDrag](el); over && d.list == list && d.key != key && can(d.key, key) {
		// The accent's line shows where it goes: before, from after, and
		// after, from before.
		before := slices.Index(keys, d.key) > i
		switch {
		case tiles:
			el.InsetShadow(0, 0, 0, 3, s.lk.accent)
		case before:
			el.InsetShadow(0, 2, 0, 0, s.lk.accent)
		default:
			el.InsetShadow(0, -2, 0, 0, s.lk.accent)
		}
	}
	if d, ok := ui.Drop[orderDrag](el); ok && d.list == list && d.key != key && can(d.key, key) {
		move(d.key, key)
	}
	step := func(by int) func() {
		return func() {
			if to := i + by; to >= 0 && to < len(keys) && can(key, keys[to]) {
				move(key, keys[to])
				s.order.focus = list + " " + key
			}
		}
	}
	el.OnShortcut(ui.Alt, ui.KeyUp, step(-1))
	el.OnShortcut(ui.Alt, ui.KeyDown, step(1))
	if tiles {
		el.OnShortcut(ui.Alt, ui.KeyLeft, step(-1))
		el.OnShortcut(ui.Alt, ui.KeyRight, step(1))
	}
	if s.order.focus == list+" "+key {
		s.order.focus = ""
		el.Focus().ScrollIntoView()
	}
}

// natural is whether a sort of the mixes lists the other way round at
// first: the newest first, and the most tracks first.
func naturalDescending(sort string) bool {
	return sort == settings.MixesByChanged || sort == settings.MixesByAdded || sort == settings.MixesByTracks
}

// wayLabel names the way a list sorted by sort goes.
func wayLabel(sort string, descending bool) string {
	switch {
	case sort == settings.MixesByChanged || sort == settings.MixesByAdded:
		if descending {
			return locales.LibrarySortNewest()
		}
		return locales.LibrarySortOldest()
	case sort == settings.MixesByTitle:
		if descending {
			return locales.LibrarySortZToA()
		}
		return locales.LibrarySortAToZ()
	case descending:
		return locales.LibrarySortMost()
	}
	return locales.LibrarySortFewest()
}

// sortTools are a list's search and sort: the box searches for query,
// under key, with placeholder and label; the choice of labels of sorts
// sets *sort, and its way *descending, which save keeps; shown of total
// match a search.
func (s *shell) sortTools(c *ui.Context, key, placeholder, label string, query *string, sorts, labels []string, sort *string, descending *bool, shown, total int) {
	k := &s.lk
	ui.Row(c.Key(key+"-tools")).Gap(12).Wrap().AlignItems(ui.Center).Margin(0, 0, 24).Children(func() {
		s.filterBox(c, key+"-search", placeholder, label, query, false, func() { *query = "" })
		chosen := labels[max(slices.Index(sorts, *sort), 0)]
		if s.selectOf(c.Key(key+"-sort"), &chosen, labels).Width(200).Label(locales.TrackToolsSortBy()).Changed() {
			if i := slices.Index(labels, chosen); i >= 0 {
				*sort, *descending = sorts[i], naturalDescending(sorts[i])
				s.save()
			}
		}
		if *sort != settings.MixesByOrder && *sort != settings.PlaylistsByService {
			way := wayLabel(*sort, *descending)
			glyph := "sort-up"
			if *descending {
				glyph = "sort-down"
			}
			b := s.buttonBase(c.Key(key + "-way")).Height(36).PaddingX(12).Gap(6).Radius(k.round(36)).Label(way).Cursor(ui.CursorPointer)
			if b.Hovered() {
				b.Background(k.hover)
			}
			if b.Clicked() {
				*descending = !*descending
				s.save()
			}
			b.Children(func() {
				ui.Icon(c, icon(glyph)).FontSize(16).TextColor(k.text)
				ui.Text(c, way).FontSize(k.smallSize()).TextColor(k.text).SingleLine()
			})
		}
		if strings.TrimSpace(*query) != "" {
			ui.Text(c, locales.TrackToolsShown(shown, total)).FontSize(k.smallSize()).TextColor(k.muted).FontFeatures("tnum")
		}
	})
}

// mixSortLabels label settings.MixesSorts.
func mixSortLabels() []string {
	return []string{locales.LibrarySortYourOrder(), locales.LibrarySortChanged(), locales.LibrarySortAdded(),
		locales.LibrarySortTitle(), locales.LibrarySortTracks()}
}

// mixTiles is Library › Mixes: the search and the sort of the mixes, New
// mix, then the mixes, the pinned ones first, as the package's comment
// tells.
func (s *shell) mixTiles(c *ui.Context) {
	all := s.svc.Collection.Playlists()
	query := strings.TrimSpace(s.order.mixes)
	var shown []media.Playlist
	for _, p := range all {
		if library.Matches(p.Title, query) {
			shown = append(shown, p)
		}
	}
	if len(all) > 0 {
		s.sortTools(c, "mixes", locales.LibraryMixesSearchPlaceholder(), locales.LibraryMixesSearchLabel(), &s.order.mixes,
			settings.MixesSorts, mixSortLabels(), &s.set.MixesSort, &s.set.MixesDescending, len(shown), len(all))
	}
	// The pinned mixes move among themselves, in any sort, and the others
	// among themselves, in their own order.
	var pins, mixes []string
	for _, p := range shown {
		if s.pinned(p.ID) {
			pins = append(pins, p.ID)
		} else {
			mixes = append(mixes, p.ID)
		}
	}
	own := s.set.MixesSort == settings.MixesByOrder
	s.tileWrap(c, func() {
		if query == "" && s.newMixTile(c) {
			s.newMix()
		}
		for _, p := range shown {
			list, keys := "mixes", mixes
			if s.pinned(p.ID) {
				list, keys = "mix-pins", pins
			} else if !own {
				s.playlistTileOf(c, "mix-"+p.ID, p, nil, nil)
				continue
			}
			s.playlistTileOf(c, "mix-"+p.ID, p, nil, func(tile ui.Element) {
				s.orderable(tile, list, p.ID, keys, true, func(string, string) bool { return true }, func(from, onto string) {
					s.moveMix(list == "mix-pins", from, onto, keys)
				})
			})
		}
	})
	if query != "" && len(shown) == 0 {
		s.emptyLine(c, locales.LibraryNoMixMatch(query))
	}
}

// moveMix moves the mix from onto the mix onto, among the mixes of among
// a list shows: among the pins, if pins, else in the user's own order.
func (s *shell) moveMix(pins bool, from, onto string, among []string) {
	var err error
	if pins {
		err = s.svc.Collection.MovePin(from, onto, among)
	} else {
		err = s.svc.Collection.MovePlaylist(from, onto, among)
	}
	if err != nil {
		s.notes = append(s.notes, noticeOf(err))
	}
}

// libraryItems end the menu of a tile of a service's library, of the
// playlist p: Pin to top or Unpin, Move to folder (railfolders.go), and
// Hide from library. Those of YouTube Music's own follow.
func (s *shell) libraryItems(key string) func(m *ui.Menu, p media.Playlist) {
	return func(m *ui.Menu, p media.Playlist) {
		m.Separator()
		s.pinItem(m, playlistMark(p))
		s.folderItems(m, playlistMark(p))
		s.hideItem(m, playlistMark(p))
		if key == "youtube" {
			s.youtubeLibraryItems(m, p)
		}
	}
}

// playlistSortLabels label settings.PlaylistsSorts, with the name of the
// service, whose own order is the first.
func playlistSortLabels(service string) []string {
	return []string{locales.LibrarySortService(service), locales.LibrarySortTitle(), locales.LibrarySortTracks()}
}

// playlistsView returns the playlists of a service's library, of list,
// that the library of key shows: those the user didn't hide, whose titles
// hold the words of query, as the user sorted them, the pinned ones
// first. It makes them again only when one of those changed (the lists
// of a service are replaced as they change, not changed in place).
func (s *shell) playlistsView(key string, list []media.Playlist, query string) []media.Playlist {
	if s.order.views == nil {
		s.order.views = map[string]*playlistsMemo{}
	}
	m := s.order.views[key]
	if m == nil {
		m = &playlistsMemo{}
		s.order.views[key] = m
	}
	same := len(m.of) == len(list) && (len(list) == 0 || &m.of[0] == &list[0])
	version := s.svc.Collection.Version()
	if !same || m.view == nil || m.query != query || m.sort != s.set.PlaylistsSort || m.descending != s.set.PlaylistsDescending || m.version != version {
		m.view = s.playlistsSorted(list, query)
		m.of, m.query, m.sort, m.descending, m.version = list, query, s.set.PlaylistsSort, s.set.PlaylistsDescending, version
	}
	return m.view
}

// playlistsSorted is playlistsView, made anew.
func (s *shell) playlistsSorted(list []media.Playlist, query string) []media.Playlist {
	col := s.svc.Collection
	out := make([]media.Playlist, 0, len(list))
	for _, p := range list {
		if !col.IsHidden(p.ID) && library.Matches(p.Title, query) {
			out = append(out, p)
		}
	}
	var by func(a, b media.Playlist) int
	switch s.set.PlaylistsSort {
	case settings.PlaylistsByTitle:
		by = func(a, b media.Playlist) int { return media.CompareText(a.Title, b.Title) }
	case settings.PlaylistsByTracks:
		by = func(a, b media.Playlist) int { return cmp.Compare(a.TrackCount, b.TrackCount) }
	}
	desc := s.set.PlaylistsDescending
	slices.SortStableFunc(out, func(a, b media.Playlist) int {
		pa, pb := col.PinAt(a.ID), col.PinAt(b.ID)
		switch {
		case pa >= 0 || pb >= 0:
			return comparePins(pa, pb)
		case by == nil:
			return 0
		case desc:
			return by(b, a)
		}
		return by(a, b)
	})
	return out
}

// comparePins orders two items by their places among the pins, -1 for
// none: the pinned first, in the order of the pins.
func comparePins(a, b int) int {
	switch {
	case a < 0 && b < 0:
		return 0
	case a < 0:
		return 1
	case b < 0:
		return -1
	}
	return cmp.Compare(a, b)
}

// pinnedFirst returns items with the pinned ones first, in the order of
// the pins, and the others as they were, without those hide hides.
func pinnedFirst[T any](s *shell, items []T, id func(T) string, hide bool) []T {
	col := s.svc.Collection
	out := make([]T, 0, len(items))
	for _, it := range items {
		if !hide || !col.IsHidden(id(it)) {
			out = append(out, it)
		}
	}
	slices.SortStableFunc(out, func(a, b T) int { return comparePins(col.PinAt(id(a)), col.PinAt(id(b))) })
	return out
}

// hiddenSection is the foot of Library › Spotify or Library › YouTube
// Music, of the service of source: the playlists and albums the user hid
// from MixMusic's library, each with Show again.
func (s *shell) hiddenSection(c *ui.Context, key string, source media.Source) {
	var hidden []library.Mark
	for _, m := range s.svc.Collection.Hidden() {
		if from, _, err := media.ParseID(m.ID); err == nil && from == source {
			hidden = append(hidden, m)
		}
	}
	if len(hidden) == 0 {
		return
	}
	k := &s.lk
	ui.Column(c.Key(key+"-hidden")).Margin(k.sections(), 0, 0).Children(func() {
		s.sectionTitle(c, locales.LibraryHiddenTitle())
		ui.Text(c, locales.LibraryHiddenHint(sourceName(source))).FontSize(k.bodySize()).TextColor(k.muted).MaxWidth(640).Margin(0, 0, 16)
		ui.Column(c).Gap(4).Children(func() {
			for _, m := range hidden {
				kind := locales.CollectionKindPlaylist()
				if m.Kind == library.MarkAlbum {
					kind = locales.CollectionKindAlbum()
				}
				title := m.Title
				if title == "" {
					title = kind
				}
				ui.Row(c.Key("hidden-" + m.ID)).Height(56).PaddingStart(16).PaddingEnd(8).Gap(12).Radius(k.radiusSm).Background(k.surface2).Children(func() {
					ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
						ui.Text(c, title).FontSize(k.bodySize()).SingleLine().MinWidth(0)
						ui.Text(c, kind).FontSize(k.smallSize()).TextColor(k.muted).SingleLine()
					})
					if s.button(c, "show-"+m.ID, locales.LibraryShowAgain(), false).Label(locales.LibraryShowAgainTitle(title)).Clicked() {
						s.setHidden(m, false)
					}
				})
			}
		})
	})
}
