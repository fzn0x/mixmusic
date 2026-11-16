package room

// The previews: computers that proved the code or the link and wait to
// join, as their users look at who hosts and what plays. A link may be
// forwarded beyond the friends it was sent to (#562), so a room holds few
// previews at once, maxPreviewsFrom of a source and maxPreviews in all: a
// new one takes the place of the oldest of its source, or, when the room
// holds as many as it takes, of the oldest of the source that holds the
// most. And each waits Timing.Preview at most. The guests, and those that
// come back by their keys, take no preview's place, so that previews never
// crowd them out.

import (
	"errors"
	"time"
)

const (
	maxPreviews     = 32
	maxPreviewsFrom = 4
)

// reasonPreviewEnded: the computer waited too long to join, or others of
// its source, or of the room, took its place.
const reasonPreviewEnded = "preview ended"

// ErrPreviewEnded: the room's preview ended before this computer joined.
var ErrPreviewEnded = errors.New("the room's preview ended before you joined: open its link, or type its code, again")

// previewCaps are the previews the room holds, in all and of a source: as
// many as it holds unproven connections, at least, so that what proves its
// code may wait to join.
func (c caps) previewCaps() (all, from int) {
	all, from = max(maxPreviews, c.unproven), max(maxPreviewsFrom, c.unprovenFrom)
	if c.previews > 0 {
		all = c.previews
	}
	if c.previewsFrom > 0 {
		from = c.previewsFrom
	}
	return all, from
}

// holdPreview takes c, which proved the code or the link, among the
// previews: it ends those whose places it takes, and c once it waited
// Timing.Preview without joining. s.mu is held.
func (s *Server) holdPreview(c *conn) {
	c.held = time.Now()
	all, from := s.adm.caps.previewCaps()
	for {
		n, mine := 0, 0
		held := map[string]int{}
		for o := range s.previews {
			// A computer that waits for the host's user to let it in keeps
			// its place: maxKnocks bound those (letin.go).
			if o == c || s.knocking(o) {
				continue
			}
			n++
			held[o.src]++
			if o.src == c.src {
				mine++
			}
		}
		var gone *conn
		switch {
		case mine >= from:
			gone = s.oldestPreview(func(o *conn) bool { return o.src == c.src }, c)
		case n >= all:
			most := 0
			for _, k := range held {
				most = max(most, k)
			}
			gone = s.oldestPreview(func(o *conn) bool { return held[o.src] == most }, c)
		}
		if gone == nil {
			break
		}
		delete(s.previews, gone)
		go endPreview(gone)
	}
	time.AfterFunc(s.Timing.preview(), func() {
		s.mu.Lock()
		_, waits := s.previews[c]
		if g := s.guests[c.key]; waits && g != nil && g.conn == c {
			// It joined meanwhile.
			waits = false
		}
		if waits && s.knocking(c) {
			// It waits for the host's user, AskFor at most.
			waits = false
		}
		if waits {
			delete(s.previews, c)
		}
		s.mu.Unlock()
		if waits {
			endPreview(c)
		}
	})
}

// oldestPreview is the preview that held longest of those which, but c.
// s.mu is held.
func (s *Server) oldestPreview(which func(*conn) bool, c *conn) *conn {
	var out *conn
	for o := range s.previews {
		if o != c && !s.knocking(o) && which(o) && (out == nil || o.held.Before(out.held)) {
			out = o
		}
	}
	return out
}

// endPreview tells c its preview ended, and closes it.
func endPreview(c *conn) {
	_ = c.f.Send(message{Type: "ended", Reason: reasonPreviewEnded}, time.Second)
	c.close()
}
