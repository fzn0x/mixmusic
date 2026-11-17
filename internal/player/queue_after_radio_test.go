package player

import (
	"context"
	"slices"
	"testing"
)

// #579: after Start radio, Add to queue goes ahead of the radio's tracks.
func TestPlayRadioThenEnqueueGoesBeforeRadio(t *testing.T) {
	h := newHarness(t, Saved{}, func(f *fakeServices) {
		f.settings.Autoplay = true
		f.radio = tracks("r1", "r2", "r3", "r4")
	})
	seed := track("a", "Artist a")
	found, _ := h.svc.Radio(context.Background(), seed)
	h.p.PlayRadio(seed, found)
	h.wait("a playing", playing("a"))
	h.p.Enqueue(tracks("e"))
	h.p.PlayNext(tracks("n"))
	h.on(func(*Player) {})
	if got, want := queueIDs(h.p.State()), []string{"a", "n", "e", "r1*", "r2*", "r3*", "r4*"}; !slices.Equal(got, want) {
		t.Errorf("queue = %v, want %v", got, want)
	}
}
