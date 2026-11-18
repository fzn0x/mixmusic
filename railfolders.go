package main

// The folders of the sidebar's Your mixes (internal/library/folders.go):
// after Liked songs, Downloads and the pins in no folder, and before the
// mixes in none. Each shows a header, with its name and how many
// playlists it holds, over what it holds, the pinned ones first. A click
// on the header, or Enter, opens or closes it, which settings.json keeps;
// a folder opens by itself while the search finds what it holds, or its
// name, which shows all of it, and once it holds the page open. A mix or
// a playlist dragged onto a header goes in that folder, and one of a
// folder dragged onto an entry of none comes out of it. Headers move
// among themselves, and the entries of a folder among themselves, by
// drag or with Alt and the arrows. Move to folder, in the menu of a mix
// or a playlist, puts it in one, and Remove from folder takes it out.

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/library"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/locales"
)

// folderState is how the folders open, beyond what settings.json keeps.
type folderState struct {
	// revealed is the page open, and the folder holding it, when the
	// folders last looked for it (revealFolder).
	revealed string
	// shut are the folders the user closed while searching for shutFor,
	// which the search would open.
	shut    map[string]bool
	shutFor string
}

// shutWhile returns the folders the user closed while searching for
// query: none, once the search changed.
func (f *folderState) shutWhile(query string) map[string]bool {
	if f.shut == nil || f.shutFor != query {
		f.shut, f.shutFor = map[string]bool{}, query
	}
	return f.shut
}

// folderEntry is the header of the folder f, whose entries are what it
// holds: the pinned first, in the order of the pins, then the others in
// the folder's order. byID has the mixes, by their IDs.
func (s *shell) folderEntry(f library.Folder, byID map[string]media.Playlist) sideEntry {
	col := s.svc.Collection
	head := sideEntry{key: sideFolder + f.ID, title: f.Name, folder: f.ID}
	var pins, rest []sideEntry
	for _, m := range f.Items {
		source, _, err := media.ParseID(m.ID)
		if err != nil {
			continue
		}
		e := sideEntry{key: sideFiled + m.ID, title: m.Title, count: -1, to: playlistPath(m.ID), source: source, mark: m,
			folder: f.ID, pinned: s.pinned(m.ID)}
		switch {
		case source == media.SourceLocal:
			p, ok := byID[m.ID]
			if !ok {
				continue
			}
			e.title, e.count, e.mark = p.Title, p.TrackCount, playlistMark(p)
		case e.title == "":
			e.title = m.ID
		}
		if e.pinned {
			pins = append(pins, e)
		} else {
			rest = append(rest, e)
		}
	}
	slices.SortStableFunc(pins, func(a, b sideEntry) int { return cmp.Compare(col.PinAt(a.mark.ID), col.PinAt(b.mark.ID)) })
	head.filed = append(pins, rest...)
	head.count = len(head.filed)
	return head
}

// sideShows reports whether the search for query shows the entry e: its
// name holds the search's words, or, of a folder, the name of what it
// holds.
func sideShows(e sideEntry, query string) bool {
	return library.Matches(e.title, query) || slices.ContainsFunc(e.filed, func(f sideEntry) bool { return library.Matches(f.title, query) })
}

// folderShown returns what the folder of the header e shows while the
// sidebar searches for query: all it holds when its name matches.
func folderShown(e sideEntry, query string) []sideEntry {
	if library.Matches(e.title, query) {
		return e.filed
	}
	var out []sideEntry
	for _, f := range e.filed {
		if library.Matches(f.title, query) {
			out = append(out, f)
		}
	}
	return out
}

// sideFind returns the entry of key among all, in a folder too.
func sideFind(all []sideEntry, key string) (sideEntry, bool) {
	for _, e := range all {
		if e.key == key {
			return e, true
		}
		for _, f := range e.filed {
			if f.key == key {
				return f, true
			}
		}
	}
	return sideEntry{}, false
}

// folderView is what a frame of Your mixes shows, for the folders: the
// page open, the search, all the entries and the keys of those shown,
// the folders' headers apart from the others' rows, and the folders
// closed.
type folderView struct {
	path, query string
	all         []sideEntry
	keys        []string
	heads, rows []string
	closed      map[string]bool
}

// folderViewOf is the folderView of a frame showing the entries of keys,
// of all.
func (s *shell) folderViewOf(path, query string, all []sideEntry, keys []string) folderView {
	v := folderView{path: path, query: query, all: all, keys: keys, closed: map[string]bool{}}
	for _, k := range keys {
		if sideKind(k) == sideFolder {
			v.heads = append(v.heads, k)
		} else {
			v.rows = append(v.rows, k)
		}
	}
	for _, id := range s.set.CollapsedFolders {
		v.closed[id] = true
	}
	return v
}

// folderOpen reports whether the folder of id shows what it holds.
func (s *shell) folderOpen(v folderView, id string) bool {
	if v.query != "" {
		return !s.order.folders.shutWhile(v.query)[id]
	}
	return !v.closed[id]
}

// toggleFolder closes the folder of id, if open, or opens it: in
// settings.json, or, while the sidebar searches for query, until the
// search changes.
func (s *shell) toggleFolder(id string, open bool, query string) {
	if query != "" {
		s.order.folders.shutWhile(query)[id] = open
		return
	}
	s.setCollapsed(id, open)
}

// setCollapsed closes the folder of id, or opens it, in settings.json,
// which forgets the folders gone.
func (s *shell) setCollapsed(id string, closed bool) {
	there := map[string]bool{}
	for _, f := range s.svc.Collection.Folders() {
		there[f.ID] = true
	}
	ids := slices.DeleteFunc(slices.Clone(s.set.CollapsedFolders), func(x string) bool { return x == id || !there[x] })
	if closed {
		ids = append(ids, id)
	}
	s.set.CollapsedFolders = ids
	s.save()
}

// revealFolder opens the folder that holds the page at path, once, as it
// comes to hold it: the user may close it again.
func (s *shell) revealFolder(all []sideEntry, path string) {
	held := ""
	for _, e := range all {
		if slices.ContainsFunc(e.filed, func(f sideEntry) bool { return f.to == path }) {
			held = e.folder
			break
		}
	}
	st := &s.order.folders
	if key := path + "\x00" + held; key != st.revealed {
		st.revealed = key
		if held != "" && slices.Contains(s.set.CollapsedFolders, held) {
			s.setCollapsed(held, false)
		}
	}
}

// The chevron of a closed folder points along the line, as the arrows
// lineIcon turns right to left do.
func init() {
	mirroredIcons["chevronRight"] = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="square" stroke-linejoin="miter"><path transform="matrix(-1 0 0 1 24 0)" d="` + iconPaths["chevronRight"] + `"/></svg>`))
}

// folderRows are the header of the folder of e and, while it's open,
// what it shows of what it holds, as the package's comment tells.
func (s *shell) folderRows(c *ui.Context, e sideEntry, v folderView) {
	k := &s.lk
	open := s.folderOpen(v, e.folder)
	head := s.buttonBase(c.Key(e.key)).Gap(8).Padding(7, 0).Label(locales.FoldersHeader(e.title, e.count)).Expanded(open).
		Cursor(ui.CursorPointer)
	if head.Clicked() {
		s.toggleFolder(e.folder, open, v.query)
		open = !open
	}
	head.ContextMenu(func(m *ui.Menu) { s.folderMenu(m, e) })
	s.orderable(head, "side", e.key, v.heads, false, s.sideMovable, func(from, onto string) { s.sideMove(v.all, v.keys, from, onto) })
	if d, over := ui.DragOver[orderDrag](head); over && d.list == "side" && sideKind(d.key) != sideFolder && s.sideMovable(d.key, e.key) {
		// What goes in the folder outlines it, rather than a line between.
		head.NoShadow().InsetShadow(0, 0, 0, 2, k.accent)
	}
	ink := k.text
	if head.Hovered() {
		ink = k.accent
	}
	chevron := "chevronRight"
	if open {
		chevron = "chevronDown"
	}
	head.Children(func() {
		ui.Icon(c, lineIcon(c, chevron)).FontSize(14).TextColor(k.muted).Shrink(0)
		ui.Text(c, e.title).FontSize(k.bodySize()).TextColor(ink).SingleLine().Grow(1).MinWidth(0)
		ui.Text(c, count(e.count)).FontSize(k.smallSize()).TextColor(k.accent).FontFeatures("tnum")
	})
	if !open {
		return
	}
	shown := folderShown(e, v.query)
	rows := make([]string, len(shown))
	for i, f := range shown {
		rows[i] = f.key
	}
	ui.Column(c.Key(e.key + "-in")).PaddingStart(22).Children(func() {
		for _, f := range shown {
			s.sideEntryLink(c, f, v.path, v.all, v.keys, rows)
		}
	})
}

// folderMenu is the menu of the header of the folder of e: Play folder,
// Rename folder… and Ungroup folder, which keeps what it holds.
func (s *shell) folderMenu(m *ui.Menu, e sideEntry) {
	col := s.svc.Collection
	if m.Item(locales.FoldersPlay()).Disabled(len(e.filed) == 0).Chosen() {
		s.playFolder(e.filed)
	}
	m.Separator()
	if m.Item(locales.FoldersRenameItem()).Chosen() {
		s.promptName(locales.FoldersRenameTitle(), locales.RailName(), e.title, locales.LibraryRename(), func(name string) {
			if err := col.RenameFolder(e.folder, name); err != nil {
				s.notes = append(s.notes, noticeOf(err))
			}
		})
	}
	if m.Item(locales.FoldersUngroup()).Chosen() {
		if err := col.UngroupFolder(e.folder); err != nil {
			s.notes = append(s.notes, noticeOf(err))
		}
	}
}

// folderItems are the items of the menu of a mix or a playlist, of mark,
// that put it in a folder of the sidebar's Your mixes, or a new one, and
// take it out of the one it's in.
func (s *shell) folderItems(m *ui.Menu, mark library.Mark) {
	col := s.svc.Collection
	in := col.FolderOf(mark.ID)
	m.Submenu(locales.FoldersMoveTo(), func(m *ui.Menu) {
		folders := col.Folders()
		for _, f := range folders {
			if m.Item(f.Name).ID(sideFolder+f.ID).Checked(f.ID == in).Chosen() && f.ID != in {
				s.fileIn(f, mark)
			}
		}
		if len(folders) > 0 {
			m.Separator()
		}
		if m.Item(locales.FoldersNewItem()).Chosen() {
			s.promptName(locales.FoldersNewTitle(), locales.RailName(), "", locales.RailCreate(), func(name string) {
				s.newFolder(name, mark)
			})
		}
	})
	if in != "" && m.Item(locales.FoldersRemoveFrom()).Chosen() {
		if err := col.RemoveFromFolder(mark.ID); err != nil {
			s.notes = append(s.notes, noticeOf(err))
		}
	}
}

// withFolders is the menu of a page, base, with folderItems of its
// playlist, mark's.
func (s *shell) withFolders(base func(m *ui.Menu), mark library.Mark) func(m *ui.Menu) {
	return func(m *ui.Menu) {
		if base != nil {
			base(m)
		}
		s.folderItems(m, mark)
	}
}

// fileIn puts mark's playlist in the folder f, and says so.
func (s *shell) fileIn(f library.Folder, mark library.Mark) {
	if err := s.svc.Collection.FileInFolder(f.ID, mark); err != nil {
		s.notes = append(s.notes, noticeOf(err))
		return
	}
	s.notes = append(s.notes, notice(locales.FoldersMoved(mark.Title, f.Name)))
}

// newFolder puts mark's playlist in a new folder named name, or in the
// one of that name, and says so.
func (s *shell) newFolder(name string, mark library.Mark) {
	f, err := s.svc.Collection.NewFolder(name, mark)
	if err != nil {
		s.notes = append(s.notes, noticeOf(err))
		return
	}
	s.notes = append(s.notes, notice(locales.FoldersMoved(mark.Title, f.Name)))
}

// folderMovable is sideMovable of the folders and what they hold: a
// header onto another, a mix or a playlist onto the header of a folder
// other than its own, an entry of a folder onto one of its folder, both
// pinned or both not, or out of it, onto an entry of no folder.
func (s *shell) folderMovable(from, onto string) bool {
	col := s.svc.Collection
	a, b := sideKind(from), sideKind(onto)
	id, to := strings.TrimPrefix(from, a), strings.TrimPrefix(onto, b)
	switch {
	case a == sideFolder:
		return b == sideFolder
	case b == sideFolder:
		return s.fileable(a, id) && col.FolderOf(id) != to
	case a == sideFiled && b == sideFiled:
		return col.FolderOf(id) == col.FolderOf(to) && s.pinned(id) == s.pinned(to)
	}
	return a == sideFiled
}

// fileable reports whether the entry of kind and id goes in a folder: a
// mix, a pinned playlist, or one in another folder.
func (s *shell) fileable(kind, id string) bool {
	switch kind {
	case sideMix, sideFiled:
		return true
	case sidePin:
		col := s.svc.Collection
		pins := col.Pins()
		i := col.PinAt(id)
		return i >= 0 && i < len(pins) && pins[i].Kind == library.MarkPlaylist
	}
	return false
}

// folderMove is sideMove of what folderMovable lets move, of all the
// entries, among the top ones of keys shown.
func (s *shell) folderMove(all []sideEntry, keys []string, from, onto string) error {
	col := s.svc.Collection
	a, b := sideKind(from), sideKind(onto)
	id, to := strings.TrimPrefix(from, a), strings.TrimPrefix(onto, b)
	switch {
	case a == sideFolder:
		var among []string
		for _, k := range keys {
			if sideKind(k) == sideFolder {
				among = append(among, strings.TrimPrefix(k, sideFolder))
			}
		}
		return col.MoveFolder(id, to, among)
	case b == sideFolder:
		e, _ := sideFind(all, from)
		return col.FileInFolder(to, e.mark)
	case b == sideFiled:
		// Among those of its folder the search shows, pinned as it is.
		folder := col.FolderOf(id)
		head, _ := sideFind(all, sideFolder+folder)
		pinned := s.pinned(id)
		var among []string
		for _, f := range folderShown(head, strings.TrimSpace(s.order.side)) {
			if f.pinned == pinned {
				among = append(among, f.mark.ID)
			}
		}
		if pinned {
			return col.MovePin(id, to, among)
		}
		return col.MoveInFolder(folder, id, to, among)
	}
	// Out of its folder: a mix dropped onto a mix takes its place, while
	// they show in their own order.
	if err := col.RemoveFromFolder(id); err != nil {
		return err
	}
	if source, _, _ := media.ParseID(id); b == sideMix && source == media.SourceLocal && s.set.MixesSort == settings.MixesByOrder {
		return col.MovePlaylist(id, to, nil)
	}
	return nil
}

// folderLoads is how many of a folder's playlists Play folder loads at
// once.
const folderLoads = 3

// playFolder plays the playlists of entries, those of a folder, one after
// another, each track once. One that doesn't load is left out, which a
// note names.
func (s *shell) playFolder(entries []sideEntry) {
	entries = slices.Clone(entries)
	go func() {
		ctx := context.Background()
		lists := make([]media.Playlist, len(entries))
		errs := make([]error, len(entries))
		var wg sync.WaitGroup
		slots := make(chan struct{}, folderLoads)
		for i, e := range entries {
			wg.Go(func() {
				slots <- struct{}{}
				defer func() { <-slots }()
				lists[i], errs[i] = s.svc.Catalog.Playlist(ctx, e.mark.ID)
			})
		}
		wg.Wait()
		var tracks []media.Track
		seen := map[string]bool{}
		for i, p := range lists {
			if errs[i] != nil {
				continue
			}
			for _, t := range p.Tracks {
				if !seen[t.ID] {
					seen[t.ID] = true
					tracks = append(tracks, t)
				}
			}
		}
		s.update(func() {
			for i, err := range errs {
				if err != nil {
					s.notes = append(s.notes, player.Notice{Message: locales.FoldersSkipped(entries[i].title, errorText(err)), Error: true})
				}
			}
			if len(tracks) == 0 {
				s.notes = append(s.notes, player.Notice{Message: locales.TilesNothingToPlay()})
				return
			}
			s.p.PlayAll(tracks, false)
		})
	}()
}
