package player

import "testing"

// TestTell has the windows told what an AI assistant did, after what it
// asked of the player (#477).
func TestTell(t *testing.T) {
	h := newHarness(t, Saved{})
	h.p.Pause()
	h.p.Tell(Notice{Message: "An AI assistant paused"})
	h.wait("the notice", func(*Player) bool { return len(h.notices()) == 1 })
	if n := h.notices()[0]; n.Message != "An AI assistant paused" || n.Error {
		t.Errorf("told %+v", n)
	}
}
