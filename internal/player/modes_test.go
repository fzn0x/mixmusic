package player

import "testing"

// TestSetRepeatAndMuted repeats and mutes as asked, whatever was before.
func TestSetRepeatAndMuted(t *testing.T) {
	h := newHarness(t, Saved{})
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	for _, r := range []Repeat{RepeatOne, RepeatOne, RepeatAll, "twice", RepeatOff} {
		h.p.SetRepeat(r)
		h.on(func(*Player) {})
		want := r
		if r == "twice" {
			want = RepeatOff
		}
		if got := h.p.State().Repeat; got != want {
			t.Errorf("asked %q: %q", r, got)
		}
	}
	h.p.SetMuted(true)
	h.p.SetMuted(true)
	h.on(func(*Player) {})
	if !h.p.State().Muted {
		t.Error("not muted")
	}
	h.p.SetMuted(false)
	h.on(func(*Player) {})
	if h.p.State().Muted {
		t.Error("still muted")
	}
}
