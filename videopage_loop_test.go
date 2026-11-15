package main

import "testing"

// TestPageLoopStalls fails a clip that loops once it plays for more than
// 5 seconds with no new frame, and never one that doesn't play.
func TestPageLoopStalls(t *testing.T) {
	eval := pageLogic(t)
	var stall float64
	eval("mvLogic.STALL", &stall)
	if stall != 5 {
		t.Errorf("a clip stalls after %v seconds", stall)
	}
	for _, c := range []struct {
		frameAt, now float64
		want         bool
	}{
		{0, 60000, false},
		{1000, 5900, false},
		{1000, 6000, false},
		{1000, 6100, true},
	} {
		var got bool
		eval("mvLogic.stalled("+jsNum(c.frameAt)+", "+jsNum(c.now)+")", &got)
		if got != c.want {
			t.Errorf("a frame at %v, at %v: stalled %v", c.frameAt, c.now, got)
		}
	}
}
