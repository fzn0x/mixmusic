package library

import (
	"cmp"
	"slices"
	"strings"

	"github.com/fzn0x/mixmusic/internal/media"
)

// Ordering the library: the user's mixes keep the order the user puts
// them in, a new one first, and list in it or in a sort of theirs. What
// the user pins leads every list it's in, in the order of the pins, and
// what they hide of a service's library stays out of MixMusic's. Pins and
// hidden items are kept by ID, and dropped only once their service listed
// its whole library, under the account they were seen under, without them
// (KeepListed): a list that still loads, failed, came in part or is of
// another account drops none.

// The kinds of what is pinned or hidden.
const (
	MarkPlaylist = "playlist"
	MarkAlbum    = "album"
	MarkArtist   = "artist"
)

// The sorts of the mixes, as settings.json names them: the user's own
// order, the last changed or made first, by title, and by how many tracks
// they hold.
const (
	SortOwn     = "order"
	SortChanged = "changed"
	SortAdded   = "added"
	SortTitle   = "title"
	SortTracks  = "tracks"
)

// Mark is an item the user pinned or hid, by its ID: a mix, as "mx:…", or
// a playlist, an album or an artist of a service. Title names it where
// the item itself isn't at hand, as in the sidebar. Account is the account
// of its service it was listed under, if known: only a whole list of that
// account drops it.
type Mark struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Title   string `json:"title,omitempty"`
	Account string `json:"account,omitempty"`
}

// markIndex is where each pin is among the pins, and what is hidden, by
// their IDs.
type markIndex struct {
	pinned map[string]int
	hidden map[string]bool
}

// indexMarks notes the IDs of what is pinned and hidden, as the file has
// them, as indexDisliked does.
func (l *Library) indexMarks() {
	l.indexing.Lock()
	defer l.indexing.Unlock()
	d := l.file.Get()
	ix := markIndex{pinned: make(map[string]int, len(d.Pins)), hidden: make(map[string]bool, len(d.Hidden))}
	for i, m := range d.Pins {
		ix.pinned[m.ID] = i
	}
	for _, m := range d.Hidden {
		ix.hidden[m.ID] = true
	}
	l.marks.Store(&ix)
}

// Pins returns what the user pinned, in the order they put it.
func (l *Library) Pins() []Mark { return orEmpty(l.file.Get().Pins) }

// PinAt returns where the item id is among the pins, or -1.
func (l *Library) PinAt(id string) int {
	if ix := l.marks.Load(); ix != nil {
		if i, ok := ix.pinned[id]; ok {
			return i
		}
	}
	return -1
}

// IsPinned reports whether the item id is pinned.
func (l *Library) IsPinned(id string) bool { return l.PinAt(id) >= 0 }

// SetPinned pins an item first among the pins, or unpins it. Pinned
// again, it keeps its place, with the title given. A pinned item shows
// again, if it was hidden.
func (l *Library) SetPinned(m Mark, pinned bool) error {
	err := l.file.Update(func(d *Data) error {
		i := slices.IndexFunc(d.Pins, func(x Mark) bool { return x.ID == m.ID })
		switch {
		case !pinned && i >= 0:
			d.Pins = slices.Delete(d.Pins, i, i+1)
		case pinned && i >= 0:
			d.Pins[i] = keptMark(d.Pins[i], m)
		case pinned:
			d.Pins = slices.Insert(d.Pins, 0, m)
		}
		if pinned {
			d.Hidden = slices.DeleteFunc(d.Hidden, func(x Mark) bool { return x.ID == m.ID })
		}
		return nil
	})
	l.indexMarks()
	return err
}

// keptMark is was, a mark kept, as m names it now: its account stays
// when m knows none.
func keptMark(was, m Mark) Mark {
	if m.Account == "" {
		m.Account = was.Account
	}
	if m.Title == "" {
		m.Title = was.Title
	}
	return m
}

// MovePin moves the pin of id onto the pin of onto, as a drag drops it,
// among the pins of among, which a search shows: every other pin keeps
// its place (MovedAmong).
func (l *Library) MovePin(id, onto string, among []string) error {
	err := l.file.Update(func(d *Data) error {
		d.Pins = movedAmong(d.Pins, func(m Mark) string { return m.ID }, among, id, onto)
		return nil
	})
	l.indexMarks()
	return err
}

// Hidden returns the items the user hid, the last hidden first.
func (l *Library) Hidden() []Mark { return orEmpty(l.file.Get().Hidden) }

// IsHidden reports whether the item id is hidden.
func (l *Library) IsHidden(id string) bool {
	if ix := l.marks.Load(); ix != nil {
		return ix.hidden[id]
	}
	return false
}

// SetHidden hides an item of a service's library, which unpins it, or
// shows it again.
func (l *Library) SetHidden(m Mark, hidden bool) error {
	err := l.file.Update(func(d *Data) error {
		i := slices.IndexFunc(d.Hidden, func(x Mark) bool { return x.ID == m.ID })
		switch {
		case !hidden && i >= 0:
			d.Hidden = slices.Delete(d.Hidden, i, i+1)
		case hidden && i >= 0:
			d.Hidden[i] = keptMark(d.Hidden[i], m)
		case hidden:
			d.Hidden = slices.Insert(d.Hidden, 0, m)
		}
		if hidden {
			d.Pins = slices.DeleteFunc(d.Pins, func(x Mark) bool { return x.ID == m.ID })
		}
		return nil
	})
	l.indexMarks()
	return err
}

// KeepListed takes a whole list of a service's library: its items of
// kind, of source, as account lists them. The pins and hidden items of
// that kind and source seen under account that it lacks go, unless keep
// keeps them, as the artists the user follows in MixMusic; those it holds
// take its titles, and account if they had none. A list that came in
// part, or of another account, must not come here.
func (l *Library) KeepListed(source media.Source, kind, account string, listed []Mark, keep func(id string) bool) error {
	if account == "" {
		return nil
	}
	titles := make(map[string]string, len(listed))
	for _, m := range listed {
		titles[m.ID] = m.Title
	}
	// changed tells what the list changes of marks: nil when nothing.
	changed := func(marks []Mark) []Mark {
		var out []Mark
		dirty := false
		for _, m := range marks {
			if s, _, err := media.ParseID(m.ID); err != nil || s != source || m.Kind != kind || m.Account != "" && m.Account != account {
				out = append(out, m)
				continue
			}
			title, ok := titles[m.ID]
			switch {
			case ok:
				if m.Account != account || title != "" && title != m.Title {
					dirty = true
					m.Account = account
					if title != "" {
						m.Title = title
					}
				}
				out = append(out, m)
			case m.Account == "" || keep != nil && keep(m.ID):
				out = append(out, m)
			default:
				dirty = true
			}
		}
		if !dirty {
			return nil
		}
		return orEmpty(out)
	}
	d := l.file.Get()
	if changed(d.Pins) == nil && changed(d.Hidden) == nil {
		return nil
	}
	err := l.file.Update(func(d *Data) error {
		if pins := changed(d.Pins); pins != nil {
			d.Pins = pins
		}
		if hidden := changed(d.Hidden); hidden != nil {
			d.Hidden = hidden
		}
		return nil
	})
	l.indexMarks()
	return err
}

// MovePlaylist moves the playlist id onto the playlist onto, in the
// user's own order, as a drag drops it, among the playlists of among,
// which a search shows: every other playlist keeps its place. Neither
// changes, so neither is Recently changed.
func (l *Library) MovePlaylist(id, onto string, among []string) error {
	return l.file.Update(func(d *Data) error {
		if !slices.ContainsFunc(d.Playlists, func(p Playlist) bool { return p.ID == id }) {
			return ErrNoPlaylist
		}
		d.Playlists = movedAmong(d.Playlists, func(p Playlist) string { return p.ID }, among, id, onto)
		return nil
	})
}

// SortedPlaylists returns the playlists, without their tracks, as sort
// orders them, one of the Sort constants, or the other way round when
// descending: the pinned ones first, in the order of the pins. Playlists
// that sort the same keep the user's own order, as does a sort unknown.
func (l *Library) SortedPlaylists(sort string, descending bool) []media.Playlist {
	d := l.file.Get()
	ps := slices.Clone(d.Playlists)
	pinned := func(p Playlist) int { return l.PinAt(media.ID(media.SourceLocal, p.ID)) }
	var by func(a, b Playlist) int
	switch sort {
	case SortChanged:
		by = func(a, b Playlist) int { return a.Updated.Compare(b.Updated) }
	case SortAdded:
		by = func(a, b Playlist) int { return a.Created.Compare(b.Created) }
	case SortTitle:
		by = func(a, b Playlist) int { return media.CompareText(a.Title, b.Title) }
	case SortTracks:
		by = func(a, b Playlist) int { return cmp.Compare(len(a.Tracks), len(b.Tracks)) }
	}
	slices.SortStableFunc(ps, func(a, b Playlist) int {
		pa, pb := pinned(a), pinned(b)
		switch {
		case pa >= 0 || pb >= 0:
			// A pin, -1 when none, leads; the pins go in their order.
			if pa < 0 {
				return 1
			}
			if pb < 0 {
				return -1
			}
			return cmp.Compare(pa, pb)
		case by == nil:
			return 0
		case descending:
			return by(b, a)
		}
		return by(a, b)
	})
	out := make([]media.Playlist, 0, len(ps))
	for _, p := range ps {
		mp := p.media()
		mp.Tracks = []media.Track{}
		out = append(out, mp)
	}
	return out
}

// MovedAmong returns keys with key moved onto onto, as a drag drops it:
// after it, from above it, and before it, from below. It moves among the
// keys of among, which a search shows, alone: each of the others keeps its
// place. Without among, it moves among them all.
func MovedAmong(keys, among []string, key, onto string) []string {
	return movedAmong(keys, func(k string) string { return k }, among, key, onto)
}

// movedAmong is MovedAmong of items, by the keys of key.
func movedAmong[E any](items []E, key func(E) string, among []string, from, onto string) []E {
	// The places of those shown, which they share among themselves.
	var places []int
	shown := map[string]bool{}
	for _, k := range among {
		shown[k] = true
	}
	for i, e := range items {
		if among == nil || shown[key(e)] {
			places = append(places, i)
		}
	}
	at := func(k string) int {
		return slices.IndexFunc(places, func(i int) bool { return key(items[i]) == k })
	}
	i, j := at(from), at(onto)
	if i < 0 || j < 0 || i == j {
		return items
	}
	order := make([]E, len(places))
	for n, p := range places {
		order[n] = items[p]
	}
	e := order[i]
	order = slices.Insert(slices.Delete(order, i, i+1), j, e)
	out := slices.Clone(items)
	for n, p := range places {
		out[p] = order[n]
	}
	return out
}

// Matches reports whether a title holds every word of query, regardless
// of case and accents: what the searches of the user's lists keep.
func Matches(title, query string) bool {
	text := media.Fold(title)
	for _, w := range strings.Fields(media.Fold(query)) {
		if !strings.Contains(text, w) {
			return false
		}
	}
	return true
}
