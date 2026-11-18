package main

// The sidebar's Your mixes (libraryorder.go's ways): Liked songs and
// Downloads, what the user pinned, of their mixes and of the services'
// libraries, then the folders the user made (railfolders.go), then their
// mixes as the Library sorts them, those in no folder. Its search keeps
// those whose names hold its words. The user drags an entry onto another,
// or moves the one with the focus with Alt+Up and Alt+Down: the pins move
// among themselves, the mixes among themselves while they show in their
// own order, and Liked songs and Downloads anywhere among them. Those two
// hide from their menus, and show again from the menu of Your mixes;
// settings.json keeps where they are (settings.SidebarPlace).

import (
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/library"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/locales"
)

// sideEntry is an entry of the sidebar's Your mixes: a place, as Liked
// songs, a pin, a folder's header, a mix or a playlist in a folder, or a
// mix. Its key is the place's, or "pin:", "folder:", "filed:" or "mix:"
// and the ID of what it opens, or of the folder.
type sideEntry struct {
	key, title string
	count      int // -1 for none
	to         string
	source     media.Source
	mark       library.Mark
	pinned     bool
	hidden     bool
	// folder is the ID of the folder of a header, or of the folder an
	// entry is in; filed are a header's entries (folderEntry).
	folder string
	filed  []sideEntry
}

// The kinds of the entries, by their keys' starts.
const (
	sidePin    = "pin:"
	sideFolder = "folder:"
	sideFiled  = "filed:"
	sideMix    = "mix:"
	sidePlace  = ""
)

// sideKind is the kind of the entry of key: sidePin, sideFolder,
// sideFiled, sideMix or sidePlace.
func sideKind(key string) string {
	switch {
	case strings.HasPrefix(key, sidePin):
		return sidePin
	case strings.HasPrefix(key, sideFolder):
		return sideFolder
	case strings.HasPrefix(key, sideFiled):
		return sideFiled
	case strings.HasPrefix(key, sideMix):
		return sideMix
	}
	return sidePlace
}

// sideEntries returns the entries of Your mixes, as the package's comment
// tells: the hidden ones too, which others may follow.
func (s *shell) sideEntries() []sideEntry {
	col := s.svc.Collection
	mixes := col.Playlists()
	byID := make(map[string]media.Playlist, len(mixes))
	for _, p := range mixes {
		byID[p.ID] = p
	}
	var out []sideEntry
	for _, m := range col.Pins() {
		source, _, err := media.ParseID(m.ID)
		if err != nil || col.FolderOf(m.ID) != "" {
			continue
		}
		if source == media.SourceLocal {
			if p, ok := byID[m.ID]; ok {
				out = append(out, sideEntry{key: sidePin + m.ID, title: p.Title, count: p.TrackCount, to: playlistPath(p.ID),
					source: source, mark: playlistMark(p), pinned: true})
			}
			continue
		}
		e := sideEntry{key: sidePin + m.ID, title: m.Title, count: -1, source: source, mark: m, pinned: true}
		switch m.Kind {
		case library.MarkAlbum:
			e.to = albumPath(m.ID)
		case library.MarkArtist:
			e.to = artistPath(m.ID)
		default:
			e.to = playlistPath(m.ID)
		}
		if e.title == "" {
			e.title = m.ID
		}
		out = append(out, e)
	}
	for _, f := range col.Folders() {
		out = append(out, s.folderEntry(f, byID))
	}
	for _, p := range mixes {
		if !s.pinned(p.ID) && col.FolderOf(p.ID) == "" {
			out = append(out, sideEntry{key: sideMix + p.ID, title: p.Title, count: p.TrackCount, to: playlistPath(p.ID),
				source: media.SourceLocal, mark: playlistMark(p)})
		}
	}
	return s.placed(out)
}

// placed returns entries with the places among them, each after the entry
// its settings say it follows, or first: Liked songs first and Downloads
// after it, until the user moved them.
func (s *shell) placed(entries []sideEntry) []sideEntry {
	places := map[string]sideEntry{
		settings.PlaceLiked: {key: settings.PlaceLiked, title: s.lk.words(locales.RailLikedSongs()), count: len(s.svc.Collection.Liked()), to: "/library/liked"},
	}
	if s.kept.Any() {
		places[settings.PlaceDownloads] = sideEntry{key: settings.PlaceDownloads, title: s.lk.words(locales.RailDownloads()), count: s.kept.Status().Tracks, to: "/library/downloads"}
	}
	var pending []settings.SidebarPlace
	for _, p := range s.set.SidebarPlaces {
		if _, ok := places[p.Key]; ok {
			pending = append(pending, p)
		}
	}
	for _, def := range []settings.SidebarPlace{{Key: settings.PlaceLiked}, {Key: settings.PlaceDownloads, After: settings.PlaceLiked}} {
		if _, ok := places[def.Key]; ok && !slices.ContainsFunc(pending, func(p settings.SidebarPlace) bool { return p.Key == def.Key }) {
			pending = append(pending, def)
		}
	}
	out := slices.Clone(entries)
	at := func(key string) int { return slices.IndexFunc(out, func(e sideEntry) bool { return e.key == key }) }
	head := 0
	for len(pending) > 0 {
		var later []settings.SidebarPlace
		for _, p := range pending {
			e := places[p.Key]
			e.hidden = p.Hidden
			i := at(p.After)
			waits := i < 0 && p.After != "" && slices.ContainsFunc(pending, func(o settings.SidebarPlace) bool { return o.Key == p.After })
			switch {
			case waits:
				later = append(later, p)
				continue
			case i < 0:
				// First, or where what it followed is gone.
				out = slices.Insert(out, head, e)
				head++
			default:
				out = slices.Insert(out, i+1, e)
				if i < head {
					head++
				}
			}
		}
		if len(later) == len(pending) {
			// They follow each other: first, then.
			for i := range later {
				later[i].After = ""
			}
		}
		pending = later
	}
	return out
}

// sideMovable reports whether the entry from may go onto the entry onto:
// a pin among the pins, a mix among the mixes while they show in their
// own order, a place anywhere, and the folders and what they hold as
// folderMovable tells.
func (s *shell) sideMovable(from, onto string) bool {
	a, b := sideKind(from), sideKind(onto)
	switch {
	case a == sideFolder || b == sideFolder || a == sideFiled || b == sideFiled:
		return s.folderMovable(from, onto)
	case a == sidePlace || b == sidePlace:
		return true
	case a == sidePin && b == sidePin:
		return true
	case a == sideMix && b == sideMix:
		return s.set.MixesSort == settings.MixesByOrder
	}
	return false
}

// sideMove moves the entry from onto the entry onto, of all the entries,
// among those shown: a place moved, or a pin or a mix moved onto a place,
// which takes the place the other way, is kept in settings.json, and the
// others in the library, the folders and what they hold as folderMove
// tells.
func (s *shell) sideMove(all []sideEntry, shown []string, from, onto string) {
	col := s.svc.Collection
	among := func(kind string) []string {
		var ids []string
		for _, k := range shown {
			if sideKind(k) == kind {
				ids = append(ids, strings.TrimPrefix(k, kind))
			}
		}
		return ids
	}
	var err error
	switch a, b := sideKind(from), sideKind(onto); {
	case a == sideFolder || b == sideFolder || a == sideFiled || b == sideFiled:
		err = s.folderMove(all, shown, from, onto)
	case a == sidePlace || b == sidePlace:
		if a != sidePlace {
			from, onto = onto, from
		}
		keys := make([]string, len(all))
		for i, e := range all {
			keys[i] = e.key
		}
		s.keepPlaces(all, library.MovedAmong(keys, shown, from, onto))
	case a == sidePin:
		err = col.MovePin(strings.TrimPrefix(from, sidePin), strings.TrimPrefix(onto, sidePin), among(sidePin))
	default:
		err = col.MovePlaylist(strings.TrimPrefix(from, sideMix), strings.TrimPrefix(onto, sideMix), among(sideMix))
	}
	if err != nil {
		s.notes = append(s.notes, noticeOf(err))
	}
}

// keepPlaces keeps where the places of all are in keys, the entries'
// keys in a new order, each after the entry before it: hidden as they
// were.
func (s *shell) keepPlaces(all []sideEntry, keys []string) {
	hidden := map[string]bool{}
	for _, e := range all {
		hidden[e.key] = e.hidden
	}
	var places []settings.SidebarPlace
	for i, k := range keys {
		if sideKind(k) != sidePlace {
			continue
		}
		p := settings.SidebarPlace{Key: k, Hidden: hidden[k]}
		if i > 0 {
			p.After = keys[i-1]
		}
		places = append(places, p)
	}
	// Those of a later version, and those not shown now, stay.
	for _, p := range s.set.SidebarPlaces {
		if !slices.ContainsFunc(places, func(o settings.SidebarPlace) bool { return o.Key == p.Key }) {
			places = append(places, p)
		}
	}
	s.set.SidebarPlaces = places
	s.save()
}

// hidePlace hides the place of key from the sidebar, or shows it again
// where it was.
func (s *shell) hidePlace(all []sideEntry, key string, hidden bool) {
	for i := range all {
		if all[i].key == key {
			all[i].hidden = hidden
		}
	}
	keys := make([]string, len(all))
	for i, e := range all {
		keys[i] = e.key
	}
	s.keepPlaces(all, keys)
}

// mixes lists liked songs, the downloads, the pins, the folders and the
// user's mixes, with their counts, as the package's comment tells.
func (s *shell) mixes(c *ui.Context, path string) {
	k := &s.lk
	all := s.sideEntries()
	query := strings.TrimSpace(s.order.side)
	s.revealFolder(all, path)
	var shown []sideEntry
	var keys []string
	searchable := false
	for _, e := range all {
		searchable = searchable || sideKind(e.key) != sidePlace
		if !e.hidden && sideShows(e, query) {
			shown = append(shown, e)
			keys = append(keys, e.key)
		}
	}
	ui.Column(c).Grow(1).MinHeight(mixesLeast).Children(func() {
		head := ui.Row(c.Key("your-mixes")).Justify(ui.SpaceBetween).Margin(0, 0, 8)
		// Its menu shows again the places hidden.
		var hidden []sideEntry
		for _, e := range all {
			if e.hidden {
				hidden = append(hidden, e)
			}
		}
		if len(hidden) > 0 {
			head.ContextMenu(func(m *ui.Menu) {
				for _, e := range hidden {
					if m.Item(locales.RailShowInSidebar(e.title)).Chosen() {
						s.hidePlace(all, e.key, false)
					}
				}
			})
		}
		head.Children(func() {
			ui.Text(c, k.words(locales.RailYourMixes())).FontSize(k.bodySize()).TextColor(k.accent)
			if s.iconButton(c, "new-mix", "plus", locales.RailNewMix(), false, 18).Clicked() {
				s.newMix()
			}
		})
		if searchable {
			s.sideSearch(c)
		}
		ui.Scroll(c).Grow(1).MinHeight(0).Children(func() {
			v := s.folderViewOf(path, query, all, keys)
			for _, e := range shown {
				if sideKind(e.key) == sideFolder {
					s.folderRows(c, e, v)
					continue
				}
				s.sideEntryLink(c, e, path, all, keys, v.rows)
			}
			if query != "" && len(shown) == 0 {
				ui.Text(c, locales.RailNothingMatches(query)).FontSize(k.smallSize()).TextColor(k.muted).PaddingY(7)
			}
		})
	})
}

// sideSearch is the box over Your mixes that searches it.
func (s *shell) sideSearch(c *ui.Context) {
	k := &s.lk
	box := ui.Row(c).Height(32).Shrink(0).Gap(8).PaddingX(10).Margin(0, 0, 6).Radius(k.round(32)).Cursor(ui.CursorText)
	var in ui.Element
	box.Children(func() {
		ui.Icon(c, icon("search")).FontSize(14).TextColor(k.muted)
		in = ui.TextInputBase(c.Key("side-search"), &s.order.side).Placeholder(k.sentence(locales.RailSearch())).Grow(1).MinWidth(0).
			Label(locales.LibraryMixesSearchLabel()).FontSize(k.smallSize())
		if s.order.side != "" {
			in.OnShortcut(0, ui.KeyEscape, func() { s.order.side = "" })
		}
	})
	switch {
	case in.Focused():
		box.Border(2, k.accent)
	case k.tonal:
		box.Background(k.surface2)
	default:
		box.Border(1, k.outline)
	}
}

// sideEntryLink is the link of the entry e of Your mixes, which opens
// what it names, and moves as the package's comment tells, among the
// entries of keys shown, of all: with Alt and the arrows, onto those of
// rows, the entries beside it of its folder, or of none.
func (s *shell) sideEntryLink(c *ui.Context, e sideEntry, path string, all []sideEntry, keys, rows []string) {
	k := &s.lk
	label := e.title
	if e.pinned {
		label = locales.LibraryPinnedTile(e.title)
	}
	link := s.buttonBase(c.Key(e.key)).Justify(ui.SpaceBetween).Gap(8).Padding(7, 0).Label(label).Cursor(ui.CursorPointer)
	if link.Clicked() {
		s.navigate(e.to)
	}
	link.ContextMenu(func(m *ui.Menu) {
		if sideKind(e.key) == sidePlace {
			if m.Item(locales.RailHideFromSidebar()).Chosen() {
				s.hidePlace(all, e.key, true)
			}
			return
		}
		s.pinItem(m, e.mark)
		if e.mark.Kind == library.MarkPlaylist {
			s.folderItems(m, e.mark)
		}
	})
	s.orderable(link, "side", e.key, rows, false, s.sideMovable, func(from, onto string) { s.sideMove(all, keys, from, onto) })
	ink := k.text
	if link.Hovered() || path == e.to {
		ink = k.accent
	}
	link.Children(func() {
		if e.pinned {
			s.pinMark(c)
		}
		ui.Text(c, e.title).FontSize(k.bodySize()).TextColor(ink).SingleLine().Grow(1).MinWidth(0)
		if e.count >= 0 {
			ui.Text(c, count(e.count)).FontSize(k.smallSize()).TextColor(k.accent).FontFeatures("tnum")
		} else {
			s.sourceMark(c, e.source, false)
		}
	})
}
