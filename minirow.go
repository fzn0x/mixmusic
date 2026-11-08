package main

import "github.com/egoist/mygo/ui"

// The mini player's transport row (#371): shuffle, previous, play, next
// and repeat, then the like, dislike and volume buttons. The artwork gives
// up its room to it (miniRoom), so that every button fits at every size
// the window takes; its gaps grow with the room it has. Where a row has no
// room for a button, as in a window narrower than the window takes, the
// mini player's menu holds it, rather than the row running out of the
// window.

// miniOptional is a button of the transport row that can go into the mini
// player's menu, as a bit of a set.
type miniOptional uint8

const (
	miniShuffle miniOptional = 1 << iota
	miniRepeat
	miniLike
	miniDislike
)

// The widths of the transport row's buttons: those that are always there,
// previous, play, next and the volume, and the others.
const (
	miniAlways  = 30 + 40 + 30 + 32
	miniToggles = 28 // shuffle and repeat
	miniRating  = 32 // like and dislike
)

// miniRow is how the transport row lays out: the buttons it leaves out,
// the gap between the transport's buttons, and the gap after them.
type miniRow struct {
	hidden        miniOptional
	gap, groupGap float32
}

// shows reports whether the row shows the button b.
func (r miniRow) shows(b miniOptional) bool { return r.hidden&b == 0 }

// miniRowOf lays out the transport row in a width w, with the like and
// dislike buttons when rated: its gaps go from 2 and 4 to 6 and 12 as it
// has room. Where the narrowest gaps leave no room for every button,
// repeat goes first, then shuffle, the dislike and the like.
func miniRowOf(w float32, rated bool) miniRow {
	var r miniRow
	if !rated {
		r.hidden = miniLike | miniDislike
	}
	for _, b := range []miniOptional{0, miniRepeat, miniShuffle, miniDislike, miniLike} {
		r.hidden |= b
		need, gaps := float32(miniAlways), 2
		for _, o := range []miniOptional{miniShuffle, miniRepeat} {
			if r.shows(o) {
				need += miniToggles
				gaps++
			}
		}
		for _, o := range []miniOptional{miniLike, miniDislike} {
			if r.shows(o) {
				need += miniRating
			}
		}
		// The gap after the transport counts twice: it parts two groups.
		extra := w - need - float32(gaps)*2 - 4
		if extra < 0 && b != miniLike {
			continue
		}
		share := max(extra, 0) / float32(gaps+2)
		r.gap, r.groupGap = 2+min(share, 4), 4+min(2*share, 8)
		return r
	}
	return r
}

// miniHiddenItems are the menu's items of the buttons the transport row
// has no room for, as the frame before laid it out, before a separator.
func (s *shell) miniHiddenItems(m *ui.Menu) {
	r := miniRow{hidden: s.mini.hidden}
	cur, ok := s.st.Current()
	hid := false
	if !r.shows(miniShuffle) {
		hid = true
		if m.Item("Shuffle").Checked(s.st.Shuffle).Chosen() {
			s.p.ToggleShuffle()
		}
	}
	if !r.shows(miniRepeat) {
		hid = true
		_, label := repeatOf(s.st.Repeat)
		if m.Item(label).Chosen() {
			s.p.CycleRepeat()
		}
	}
	if ok && !r.shows(miniLike) {
		hid = true
		liked := s.a.IsLiked(cur.Track.ID)
		if m.Item("Like").Checked(liked).Chosen() {
			s.setLiked(cur.Track, !liked)
		}
	}
	if ok && !r.shows(miniDislike) {
		hid = true
		disliked := s.a.IsDisliked(cur.Track.ID)
		if m.Item("Dislike").Checked(disliked).Chosen() {
			s.setDisliked(cur.Track, !disliked)
		}
	}
	if hid {
		m.Separator()
	}
}

// miniMenu is the mini player's menu, on a right click: the buttons its
// window has no room for, then the speed of what plays.
func (s *shell) miniMenu(m *ui.Menu) {
	s.miniHiddenItems(m)
	s.speedItems(m)
}
