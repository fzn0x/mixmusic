package player

import (
	"slices"
	"testing"
)

// TestAddToQueuePlaysWhenNothingIs plays what Add to queue adds while no
// track is current, from the first the queue doesn't pass over, and only
// queues it once one is, before the radio's tracks; Enqueue, the plugins'
// and the AI assistants', plays nothing.
func TestAddToQueuePlaysWhenNothingIs(t *testing.T) {
	h := newHarness(t, Saved{}, func(f *fakeServices) {
		f.settings.SkipDisliked = true
		f.disliked = map[string]bool{"yt:a": true}
	})
	h.p.Enqueue(tracks("e"))
	h.on(func(*Player) {})
	if s := h.p.State(); s.Index >= 0 || s.Playing || !slices.Equal(queueIDs(s), []string{"e"}) {
		t.Fatalf("Enqueue played: queue %v at %d", queueIDs(s), s.Index)
	}

	// e waits in the queue; a, disliked, is passed over, and b plays.
	h.p.AddToQueue(tracks("a", "b"))
	h.wait("b playing", playing("b"))
	if got, want := queueIDs(h.p.State()), []string{"e", "a", "b"}; !slices.Equal(got, want) {
		t.Errorf("queue %v, want %v", got, want)
	}

	h.on(func(p *Player) {
		p.s.Queue = append(slices.Clone(p.s.Queue), Item{UID: "r1", Track: track("r"), Radio: true})
	})
	h.p.AddToQueue(tracks("c"))
	h.on(func(*Player) {})
	s := h.p.State()
	if got, want := queueIDs(s), []string{"e", "a", "b", "c", "r*"}; !slices.Equal(got, want) {
		t.Errorf("queue %v, want %v", got, want)
	}
	if current(s) != "b" {
		t.Errorf("Add to queue moved the current track to %s", current(s))
	}
	if it := s.Queue[3]; !it.Chosen {
		t.Errorf("a track added by itself isn't one the user chose: %+v", it)
	}
}
