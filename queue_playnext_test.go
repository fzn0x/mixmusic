package main

import (
	"slices"
	"testing"

	"github.com/fzn0x/mixmusic/internal/player"
)

// TestQueueMenuPlayNext moves an entry of Up next right after the track
// playing from its menu, which the playing track's own menu doesn't offer.
func TestQueueMenuPlayNext(t *testing.T) {
	s := newTestShell(t)
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	tt := tester(t, s)
	if err := tt.RightClick(testTracks[0].Title); err != nil {
		t.Fatal(err)
	}
	if slices.Contains(tt.Menu(), "Play next") {
		t.Errorf("the track playing offers Play next: %q", tt.Menu())
	}
	tt.Frame()
	if err := tt.RightClick(testTracks[4].Title); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Play next"); err != nil {
		t.Fatalf("%v: the menu holds %q", err, tt.Menu())
	}
	tt.Frame()
	s.settle(t, func(st player.State) bool {
		return len(st.Queue) == len(testTracks) && st.Queue[1].Track.ID == testTracks[4].ID
	})
}
