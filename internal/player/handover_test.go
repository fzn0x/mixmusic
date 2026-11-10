package player

import (
	"slices"
	"strings"
	"testing"
	"time"
)

// queueIDs are the IDs of the queue's tracks, without "yt:".
func (h *harness) queueIDs() []string {
	var out []string
	for _, it := range h.p.State().Queue {
		out = append(out, strings.TrimPrefix(it.Track.ID, "yt:"))
	}
	return out
}

// uid is the UID of the entry of the track id.
func (h *harness) uid(id string) string {
	h.t.Helper()
	for _, it := range h.p.State().Queue {
		if it.Track.ID == "yt:"+id {
			return it.UID
		}
	}
	h.t.Fatalf("no entry of %s in %v", id, h.queueIDs())
	return ""
}

// TestTakeOver replaces the queue with one another computer handed over,
// and plays its track from where it was there, or holds it paused, with
// the other's shuffle and repeat.
func TestTakeOver(t *testing.T) {
	h := newHarness(t, Saved{})
	h.p.PlayTracks(tracks("x"), 0)
	h.wait("x playing", playing("x"))

	h.p.TakeOver(tracks("a", "b", "c"), 1, 61*time.Second, true, true, RepeatOne)
	h.wait("b playing", playing("b"))
	if got := h.stream.lastLoad(); got != "load url:yt:b 1m1s true" {
		t.Errorf("loaded %q", got)
	}
	st := h.p.State()
	if !slices.Equal(h.queueIDs(), []string{"a", "b", "c"}) || st.Index != 1 || !st.Shuffle || st.Repeat != RepeatOne {
		t.Errorf("queue %v at %d, shuffle %v, repeat %q", h.queueIDs(), st.Index, st.Shuffle, st.Repeat)
	}
	h.on(func(p *Player) {
		if !p.played[p.s.Queue[0].UID] || p.played[p.s.Queue[2].UID] {
			t.Errorf("played %v", p.played)
		}
	})
	// The order it came in is its own.
	h.p.SetShuffle(false)
	h.on(func(*Player) {})
	if got := h.queueIDs(); !slices.Equal(got, []string{"a", "b", "c"}) {
		t.Errorf("unshuffled %v", got)
	}

	h.p.TakeOver(tracks("d", "e"), 9, 5*time.Second, false, false, "")
	h.wait("e held", func(p *Player) bool {
		cur, ok := p.s.Current()
		return ok && cur.Track.ID == "yt:e" && !p.s.Playing && p.s.Source != nil
	})
	if got := h.stream.lastLoad(); got != "load url:yt:e 5s false" {
		t.Errorf("loaded %q", got)
	}
	if st := h.p.State(); st.Shuffle || st.Repeat != RepeatOff || st.Position != 5*time.Second {
		t.Errorf("shuffle %v, repeat %q, at %v", st.Shuffle, st.Repeat, st.Position)
	}
	h.p.TakeOver(nil, 0, 0, true, false, RepeatAll)
	h.on(func(*Player) {})
	if got := h.queueIDs(); !slices.Equal(got, []string{"d", "e"}) {
		t.Errorf("nothing handed over changed the queue to %v", got)
	}
}

// TestEntries plays, moves and takes out entries by their UIDs, as another
// computer names them: only those after the current one move or go.
func TestEntries(t *testing.T) {
	h := newHarness(t, Saved{})
	h.p.PlayTracks(tracks("a", "b", "c", "d", "e"), 1)
	h.wait("b playing", playing("b"))
	step := func(what string, do func(), want ...string) {
		t.Helper()
		do()
		h.on(func(*Player) {})
		if got := h.queueIDs(); !slices.Equal(got, want) {
			t.Errorf("%s: %v, want %v", what, got, want)
		}
		if cur, _ := h.p.State().Current(); cur.Track.ID != "yt:b" {
			t.Errorf("%s: %s is current", what, cur.Track.ID)
		}
	}
	step("e onto c", func() { h.p.MoveEntry(h.uid("e"), h.uid("c")) }, "a", "b", "e", "c", "d")
	step("c onto d", func() { h.p.MoveEntry(h.uid("c"), h.uid("d")) }, "a", "b", "e", "d", "c")
	step("a, before", func() { h.p.MoveEntry(h.uid("a"), h.uid("e")) }, "a", "b", "e", "d", "c")
	step("onto the current", func() { h.p.MoveEntry(h.uid("e"), h.uid("b")) }, "a", "b", "e", "d", "c")
	step("onto one gone", func() { h.p.MoveEntry(h.uid("e"), "gone") }, "a", "b", "e", "d", "c")
	step("d out", func() { h.p.RemoveEntry(h.uid("d")) }, "a", "b", "e", "c")
	step("the current out", func() { h.p.RemoveEntry(h.uid("b")) }, "a", "b", "e", "c")
	step("a, before, out", func() { h.p.RemoveEntry(h.uid("a")) }, "a", "b", "e", "c")
	step("one gone out", func() { h.p.RemoveEntry("gone") }, "a", "b", "e", "c")
	h.p.PlayEntry("gone")
	h.p.PlayEntry(h.uid("c"))
	h.wait("c playing", playing("c"))
	if st := h.p.State(); st.Index != 3 {
		t.Errorf("c plays at %d", st.Index)
	}
}
