package player

import (
	"slices"
	"testing"
)

// TestPlayNextAt moves a queued entry right after the current one, without
// a copy: a radio entry becomes one the user chose, the current entry and
// one already next stay, and the queue's own order follows under shuffle.
func TestPlayNextAt(t *testing.T) {
	h := newHarness(t, Saved{})
	h.p.PlayTracks(tracks("a", "b", "c", "d"), 1)
	h.wait("b playing", playing("b"))
	h.on(func(p *Player) {
		p.s.Queue = slices.Clone(p.s.Queue)
		p.s.Queue = append(p.s.Queue, Item{UID: "r1", Track: track("r", "R"), Radio: true, Skip: SkipDisliked})
	})

	h.p.PlayNextAt(1) // the current one stays
	h.p.PlayNextAt(2) // already next
	h.on(func(*Player) {})
	if got := queueIDs(h.p.State()); !slices.Equal(got, []string{"a", "b", "c", "d", "r*"}) {
		t.Fatalf("queue %v", got)
	}

	h.p.PlayNextAt(3) // d
	h.on(func(*Player) {})
	if got := queueIDs(h.p.State()); !slices.Equal(got, []string{"a", "b", "d", "c", "r*"}) {
		t.Fatalf("queue %v", got)
	}

	h.p.PlayNextAt(4) // the radio entry
	h.on(func(*Player) {})
	s := h.p.State()
	if got := queueIDs(s); !slices.Equal(got, []string{"a", "b", "r", "d", "c"}) {
		t.Fatalf("queue %v", got)
	}
	if it := s.Queue[2]; it.Radio || !it.Chosen || it.Skip != "" {
		t.Errorf("the moved radio entry is %+v", it)
	}
	if current(s) != "b" {
		t.Errorf("the current track is %s", current(s))
	}

	// An entry before the current one moves after it too.
	h.p.PlayNextAt(0)
	h.on(func(*Player) {})
	s = h.p.State()
	if got := queueIDs(s); !slices.Equal(got, []string{"b", "a", "r", "d", "c"}) || s.Index != 0 {
		t.Errorf("queue %v at %d", got, s.Index)
	}

	h.p.PlayNextAt(-1)
	h.p.PlayNextAt(9)
	h.on(func(*Player) {})
	if got := queueIDs(h.p.State()); len(got) != 5 {
		t.Errorf("queue %v", got)
	}
}

// TestPlayNextAtShuffled keeps the moved entry right after the current one
// in the queue's own order, so turning shuffle off leaves it there.
func TestPlayNextAtShuffled(t *testing.T) {
	h := newHarness(t, Saved{})
	h.p.PlayTracks(tracks("a", "b", "c", "d", "e"), 0)
	h.wait("a playing", playing("a"))
	h.p.SetShuffle(true)
	h.on(func(*Player) {})
	var at int
	h.on(func(p *Player) {
		for i, it := range p.s.Queue {
			if it.Track.ID == "yt:e" {
				at = i
			}
		}
	})
	h.p.PlayNextAt(at)
	h.on(func(*Player) {})
	if got := queueIDs(h.p.State()); got[1] != "e" || len(got) != 5 {
		t.Fatalf("shuffled queue %v", got)
	}
	h.p.SetShuffle(false)
	h.on(func(*Player) {})
	if got := queueIDs(h.p.State()); !slices.Equal(got, []string{"a", "e", "b", "c", "d"}) {
		t.Errorf("unshuffled queue %v", got)
	}
}
