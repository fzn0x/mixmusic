package player

import (
	"slices"
	"sync"
	"testing"
)

// TestVolumeAndMuteTellSubscribers shows the volume and the mute telling
// the subscribers that the player's state changed, as the shuffle and the
// repeat do: the programs that control MixMusic hear of a change made in
// the app (#591's review).
func TestVolumeAndMuteTellSubscribers(t *testing.T) {
	h := newHarness(t, Saved{})
	var mu sync.Mutex
	var got []Event
	h.p.Subscribe(func(e Event) {
		mu.Lock()
		defer mu.Unlock()
		got = append(got, e)
	})
	count := func() int {
		mu.Lock()
		defer mu.Unlock()
		return len(slices.DeleteFunc(slices.Clone(got), func(e Event) bool { return e != StateChange }))
	}
	h.p.SetVolume(0.3)
	h.p.Await()
	if n := count(); n != 1 {
		t.Errorf("the volume told %d changes, want 1: %v", n, got)
	}
	h.p.ToggleMute()
	h.p.Await()
	if n := count(); n != 2 {
		t.Errorf("the mute told %d changes, want 2: %v", n, got)
	}
	h.p.SetMuted(false)
	h.p.Await()
	if n := count(); n != 3 {
		t.Errorf("the unmute told %d changes, want 3: %v", n, got)
	}
	// A mute that changes nothing tells nothing.
	h.p.SetMuted(false)
	h.p.Await()
	if n := count(); n != 3 {
		t.Errorf("a mute that changed nothing told %d changes: %v", n, got)
	}
}
