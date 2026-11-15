package main

import (
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/player"
)

// TestJumpToHighlight plays the track from its highlight, from the now
// playing page's button and from the command.
func TestJumpToHighlight(t *testing.T) {
	s := markedShell(t)
	tt := testerOf(t, s, 1280, 1600)
	show(s, tt, "/now-playing")
	click(t, s, tt, "Jump to the highlight")
	s.settle(t, func(st player.State) bool { return st.Position == 150*time.Second })
	s.p.Seek(10 * time.Second)
	s.settle(t, func(st player.State) bool { return st.Position == 10*time.Second })
	for _, cmd := range s.commands() {
		if cmd.id == "player.highlight" {
			cmd.run()
		}
	}
	s.settle(t, func(st player.State) bool { return st.Position == 150*time.Second })
}

// TestNoHighlight shows no button of a track without a highlight, and the
// command tells so.
func TestNoHighlight(t *testing.T) {
	s := newTestShell(t)
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	tt := testerOf(t, s, 1280, 1600)
	show(s, tt, "/now-playing")
	if _, ok := tt.Find("Jump to the highlight"); ok {
		t.Error("a track without a highlight offers to jump to it")
	}
	s.jumpToHighlight()
	tt.Frame()
	if !tt.HasText("This track has no highlight") {
		t.Errorf("the command tells nothing: %q", tt.Texts())
	}
}
