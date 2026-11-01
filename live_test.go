package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
)

// liveRadio is a live video, as YouTube Music lists it: of no length.
var liveRadio = media.Track{ID: "yt:liveRadio01", Source: media.SourceYouTube, Title: "beats to test to",
	Artists: []media.Ref{{Name: "A Radio"}}, Video: true}

// TestLiveInThePlayerBar shows a live stream in the player bar and the
// mini player as live, with no time, no length and no seeking.
func TestLiveInThePlayerBar(t *testing.T) {
	s := newTestShell(t)
	s.panel = ""
	s.p.PlayTracks([]media.Track{liveRadio}, 0)
	s.settle(t, func(st player.State) bool { return st.Playing && st.Live })
	s.p.Time(90*time.Second, 0)
	s.settle(t, func(st player.State) bool { return st.Position == 90*time.Second })
	tt := tester(t, s)
	if _, ok := tt.Find("Live: no seeking"); !ok || !tt.HasText("live") || tt.HasText("1:30") || tt.HasText("0:00") {
		t.Errorf("the player bar of a live stream shows %q", tt.Texts())
	}
	// The bar doesn't seek, nor do the arrow keys.
	if r, ok := tt.Find("Live: no seeking"); ok {
		tt.Press(r.X+r.W/4, r.Y+r.H/2)
		tt.Release(r.X+r.W/4, r.Y+r.H/2)
	}
	tt.Key(0, ui.KeyLeft)
	s.drain()
	tt.Frame()
	if st := s.p.State(); st.Position != 90*time.Second {
		t.Errorf("a live stream at %v after a seek", st.Position)
	}

	mini := ui.NewTester(s.miniView, 420, 132)
	mini.Frame()
	if !mini.HasText("live") || mini.HasText("1:30 / 0:00") {
		t.Errorf("the mini player of a live stream shows %q", mini.Texts())
	}
	if _, ok := mini.Find("Progress"); ok {
		t.Error("the mini player shows a live stream's progress")
	}
}
