package main

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
)

// Putting a mix in order: the user drags a track of it onto another, which
// it takes the place of, or moves it a place up or down with Alt+Up and
// Alt+Down, as the queue's entries are dragged (reorderable). The mix is
// saved at once. A playlist of the user's on YouTube Music moves there too,
// and one on Spotify they may change, there (collab.go).

// rowDrag is a track of a list dragged, by the list's key and its place
// in the list.
type rowDrag struct {
	list  string
	index int
}

// rowMoves is what the keys moved last: the row of the list it went to,
// which takes the focus as it's built, while focus is set. last ends once
// the move a service was asked for last ended, which the next waits for.
type rowMoves struct {
	list  string
	index int
	focus bool
	last  chan struct{}
}

// movable lets the user move the track at i of the list of key, of n
// tracks, as the package's comment tells: move moves it from its place to
// another's. A line of the accent on the row the pointer drags it over
// shows where it goes: above, from below it, and below, from above.
// lead, the row's play button, takes the focus once the keys moved the
// track there, so that they move it on.
func (s *shell) movable(row, lead ui.Element, key string, i, n int, move func(from, to int)) {
	row.Drag(rowDrag{key, i})
	if row.Dragging() {
		row.Opacity(0.4)
	}
	if d, over := ui.DragOver[rowDrag](row); over && d.list == key && d.index != i {
		if d.index > i {
			row.InsetShadow(0, 2, 0, 0, s.lk.accent)
		} else {
			row.InsetShadow(0, -2, 0, 0, s.lk.accent)
		}
	}
	if d, ok := ui.Drop[rowDrag](row); ok && d.list == key && d.index != i {
		move(d.index, i)
	}
	step := func(to int) {
		if to >= 0 && to < n {
			move(i, to)
			s.moves.list, s.moves.index, s.moves.focus = key, to, true
		}
	}
	row.OnShortcut(ui.Alt, ui.KeyUp, func() { step(i - 1) })
	row.OnShortcut(ui.Alt, ui.KeyDown, func() { step(i + 1) })
	if s.moves.focus && s.moves.list == key && s.moves.index == i {
		s.moves.focus = false
		lead.Focus().ScrollIntoView()
	}
}

// playlistMove is how the page of key moves the tracks of p, as read in
// r: a mix's, and those of a playlist of the user's on YouTube Music, or
// one on Spotify they may change, once it's read whole, while the list
// shows them in their own order. It's nil for the others, which don't
// move.
func (s *shell) playlistMove(key string, r *playlistRead, p media.Playlist) func(from, to int) {
	switch {
	case r.reading || !s.inOwnOrder(key):
		return nil
	case p.Source == media.SourceLocal:
		return func(from, to int) { s.moveInMix(r, p.ID, from, to) }
	case s.spotifyMovable(r, p):
		return func(from, to int) { s.moveOnSpotifyPlaylist(key, r, p, from, to) }
	case p.Source == media.SourceYouTube && p.Editable && s.signedIn(media.SourceYouTube) && placed(r.p.Tracks):
		return func(from, to int) { s.moveOnYouTubePlaylist(key, r, p, from, to) }
	}
	return nil
}

// inOwnOrder reports whether the list of key shows all its tracks, in
// their own order: sorted or searched, a track has no place to move to.
func (s *shell) inOwnOrder(key string) bool {
	tool := s.tools[key]
	return tool == nil || tool.sort == "order" && strings.TrimSpace(tool.query) == ""
}

// movedTo returns tracks with the one at from moved to to, in a new slice.
func movedTo(tracks []media.Track, from, to int) []media.Track {
	t := tracks[from]
	return slices.Insert(slices.Delete(slices.Clone(tracks), from, from+1), to, t)
}

// moveInMix moves the track at from of the mix id to to, and the page,
// whose reading of it is r, shows it there at once.
func (s *shell) moveInMix(r *playlistRead, id string, from, to int) {
	if from < 0 || from >= len(r.p.Tracks) || to < 0 || to >= len(r.p.Tracks) {
		return
	}
	if err := s.svc.Collection.MoveInPlaylist(id, from, to); err != nil {
		s.notes = append(s.notes, noticeOf(err))
		return
	}
	r.p.Tracks = movedTo(r.p.Tracks, from, to)
}

// moveOnYouTube moves a track of a playlist of the user's on YouTube
// Music, before another of it, or to its end before the zero track; tests
// answer for YouTube.
var moveOnYouTube = func(s *shell, ctx context.Context, playlistID string, t, before media.Track) error {
	return s.svc.Collection.MoveInYouTubePlaylist(ctx, playlistID, t, before)
}

// moveOnYouTubePlaylist moves the track at from of p, a playlist of the
// user's on YouTube Music the page of key shows, as read in r, to to: the
// page shows it there at once, and YouTube hears of it after the moves
// before. Refused, the page reads the playlist again, as YouTube keeps it.
func (s *shell) moveOnYouTubePlaylist(key string, r *playlistRead, p media.Playlist, from, to int) {
	if from < 0 || from >= len(r.p.Tracks) || to < 0 || to >= len(r.p.Tracks) {
		return
	}
	tracks := movedTo(r.p.Tracks, from, to)
	r.p.Tracks = tracks
	t, before := tracks[to], media.Track{}
	if to+1 < len(tracks) {
		before = tracks[to+1]
	}
	prev, done := s.moves.last, make(chan struct{})
	s.moves.last = done
	go func() {
		defer close(done)
		if prev != nil {
			<-prev
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := moveOnYouTube(s, ctx, p.ID, t, before); err != nil {
			s.update(func() {
				s.notes = append(s.notes, noticeOf(err))
				s.refetch(key)
			})
		}
	}()
}

// placed reports whether each of tracks has its place in the playlist it
// was read from, which a move on YouTube Music names it by.
func placed(tracks []media.Track) bool {
	return !slices.ContainsFunc(tracks, func(t media.Track) bool { return t.EntryID == "" })
}
