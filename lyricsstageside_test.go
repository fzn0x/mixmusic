package main

import (
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/lyrics"
)

// stageQueueOn plays the test tracks from the second, with the stage shown
// over the window: the first has played, the second plays, and the third and
// those after wait in the queue.
func stageQueueOn(t *testing.T) (*shell, *ui.Tester) { return stageQueueWith(t, nil) }

// stageQueueWith is stageQueueOn whose tracks have the cover src gives, or
// none, for a nil src.
func stageQueueWith(t *testing.T, src artSource) (*shell, *ui.Tester) {
	t.Helper()
	s := newTestShell(t)
	tracks := slices.Clone(testTracks)
	if src != nil {
		for i := range tracks {
			tracks[i].Artwork = []media.Image{{URL: stageCoverURL, Width: 600, Height: 600}}
		}
		s.art.src = src
	}
	for _, tr := range tracks {
		s.loads["lyrics:"+tr.ID] = &loaded[lyrics.Lyrics]{done: true, value: lyrics.Lyrics{Synced: true, Source: "Unison", Language: "en", Lines: stageSong}}
	}
	// Without the queue's panel, only the stage has buttons that play a
	// track by its title.
	s.panel = ""
	s.p.PlayTracks(tracks, 1)
	s.settle(t, func(st player.State) bool {
		cur, ok := st.Current()
		return ok && st.Index == 1 && cur.Track.ID == tracks[1].ID && st.Playing && !st.Loading
	})
	tt := testerOf(t, s, 1280, 820)
	s.toggleStage()
	s.drain()
	tt.Frame()
	tt.Advance(time.Second)
	return s, tt
}

// TestStageQueueCovers shows the covers of the tracks next in the queue,
// those after the one playing and no others, and plays the one that is
// clicked: by its place in the whole queue, which is not its place among the
// covers.
func TestStageQueueCovers(t *testing.T) {
	s, tt := stageQueueOn(t)
	for i, tr := range testTracks {
		_, shows := tt.Find("Play " + tr.Title)
		if want := i >= 2; shows != want {
			t.Errorf("the cover of track %d, %q, shows %v, not %v", i, tr.Title, shows, want)
		}
	}
	if err := tt.Click("Play " + testTracks[4].Title); err != nil {
		t.Fatalf("clicking a cover: %v; the window shows %q", err, tt.Texts())
	}
	s.drain()
	s.settle(t, func(st player.State) bool { return st.Index == 4 })
	tt.Frame()
	if !s.stage.open {
		t.Error("a cover closed the stage")
	}
}

// TestStageQueueKeepsItsRoom keeps the strip of covers where it is when the
// queue runs out, so that the lyrics keep their width and don't wrap anew.
func TestStageQueueKeepsItsRoom(t *testing.T) {
	s, tt := stageQueueOn(t)
	before, ok := tt.Find("Up next")
	if !ok {
		t.Fatalf("the stage has no strip for the queue: %q", tt.Texts())
	}
	s.p.PlayAt(len(testTracks) - 1)
	s.settle(t, func(st player.State) bool { return st.Index == len(testTracks)-1 })
	tt.Frame()
	tt.Advance(time.Second)
	after, ok := tt.Find("Up next")
	if !ok {
		t.Fatal("the strip goes with the last track")
	}
	if after.X != before.X || after.W != before.W || after.W != stageQueueW {
		t.Errorf("the strip was %+v and is %+v at the end of the queue, %v wide", before, after, float32(stageQueueW))
	}
	if _, shows := tt.Find("Play " + testTracks[0].Title); shows {
		t.Error("a cover shows with nothing next in the queue")
	}
}

// TestStageControls plays, pauses and skips from the buttons at the left of
// the cover.
func TestStageControls(t *testing.T) {
	s, tt := stageQueueOn(t)
	press := func(label string) {
		t.Helper()
		if err := tt.Click(label); err != nil {
			t.Fatalf("clicking %q: %v; the window shows %q", label, err, tt.Texts())
		}
		s.drain()
		tt.Frame()
	}
	press("Next track")
	s.settle(t, func(st player.State) bool { return st.Index == 2 })
	press("Previous track")
	s.settle(t, func(st player.State) bool { return st.Index == 1 })

	// The button between them plays and pauses. The window behind the stage
	// has one labeled the same, so it is clicked where it lies: the middle of
	// the column of the three.
	prev, _ := tt.Find("Previous track")
	was := s.p.State().Playing
	tt.ClickAt(prev.X+prev.W/2, prev.Y+prev.H+14+26)
	s.drain()
	s.settle(t, func(st player.State) bool { return st.Playing != was })
	tt.Frame()
	if !s.stage.open {
		t.Error("a button closed the stage")
	}
}

// TestStageSidesInANarrowWindow gives up the covers of the queue, and then the
// buttons, to the lyrics, as the window narrows.
func TestStageSidesInANarrowWindow(t *testing.T) {
	s, _ := stageQueueOn(t)
	for _, c := range []struct {
		w, h             int
		controls, strip  bool
		whatTheWindowHas string
	}{
		{1280, 820, true, true, "a wide window"},
		{700, 600, true, false, "a narrow one"},
		{560, 600, false, false, "a very narrow one"},
	} {
		tt := testerOf(t, s, c.w, c.h)
		tt.Advance(time.Second)
		if _, ok := tt.Find("Next track"); ok != c.controls {
			t.Errorf("%s shows the buttons: %v, not %v", c.whatTheWindowHas, ok, c.controls)
		}
		if _, ok := tt.Find("Up next"); ok != c.strip {
			t.Errorf("%s shows the covers of the queue: %v, not %v", c.whatTheWindowHas, ok, c.strip)
		}
	}
}
