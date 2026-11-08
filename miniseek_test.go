package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
)

// TestMiniSeek seeks from the mini player's progress: a click, with the
// time under the pointer above it, and its arrow keys.
func TestMiniSeek(t *testing.T) {
	s := playing(t)
	tt := ui.NewTester(s.miniView, 420, 132)
	tt.Frame()
	r, ok := tt.Find("Seek")
	if !ok || r.Y+r.H != 132 || r.W != 420 {
		t.Fatalf("the progress is at %+v (%v), not along the foot", r, ok)
	}
	if a, _ := tt.Access("Seek"); a.Role != ui.RoleSlider || a.Value != "0:00 of 3:31" {
		t.Errorf("screen readers see the progress as %+v", a)
	}
	tt.Move(r.X+r.W/4, r.Y+r.H/2)
	tt.Frame()
	if !tt.HasText("0:52") {
		t.Errorf("hovering a quarter of the progress doesn't show its time: %q", tt.Texts())
	}
	tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
	s.drain()
	s.settle(t, func(st player.State) bool { return st.Position > 100*time.Second && st.Position < 110*time.Second })
	tt.Move(4, 4)
	tt.Frame()
	if tt.HasText("0:52") {
		t.Errorf("the time stays once the pointer left the progress: %q", tt.Texts())
	}
	tabTo(t, tt, "Seek", "")
	was := s.p.State().Position
	tt.Key(0, ui.KeyRight)
	s.settle(t, func(st player.State) bool { return st.Position > was })
}

// TestMiniSeekLive shows no progress for a live stream, which doesn't
// seek.
func TestMiniSeekLive(t *testing.T) {
	s := newTestShell(t)
	s.p.PlayTracks([]media.Track{liveRadio}, 0)
	s.settle(t, func(st player.State) bool { return st.Live && st.Playing })
	tt := ui.NewTester(s.miniView, 420, 132)
	tt.Frame()
	if _, ok := tt.Find("Seek"); ok {
		t.Error("a live stream seeks in the mini player")
	}
}
