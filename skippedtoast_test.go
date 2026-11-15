package main

import (
	"testing"

	"github.com/fzn0x/mixmusic/internal/player"
)

// TestSkippedToastUndoes offers undo in the toast of a part skipped.
func TestSkippedToastUndoes(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 820)
	show(s, tt, "/")
	undone := false
	s.notes = append(s.notes, player.Notice{Message: "Skipped the intro", Undo: func() { undone = true }})
	tt.Frame()
	if !tt.HasText("Skipped the intro") {
		t.Fatalf("no toast: %q", tt.Texts())
	}
	click(t, s, tt, "undo")
	if !undone {
		t.Error("undo did nothing")
	}
}
