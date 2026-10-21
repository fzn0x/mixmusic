package native

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/player"
)

func TestMini(t *testing.T) {
	s := newTestShell(t)
	tt := ui.NewTester(s.miniView, 420, 132)
	tt.Frame()
	if !tt.HasText("nothing playing") {
		t.Errorf("the idle mini player shows %q", tt.Texts())
	}
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	tt.Frame()
	for _, want := range []string{testTracks[0].Title, "M83", "0:00 / 3:31"} {
		if !tt.HasText(want) {
			t.Errorf("the mini player doesn't show %q: %q", want, tt.Texts())
		}
	}
	click(t, s, tt, "Pause")
	s.settle(t, func(st player.State) bool { return !st.Playing })
	tt.Frame()
	click(t, s, tt, "Play")
	s.settle(t, func(st player.State) bool { return st.Playing })
	tt.Frame()
	click(t, s, tt, "Next")
	s.settle(t, func(st player.State) bool { return st.Index == 1 })
	tt.Frame()
	click(t, s, tt, "Previous")
	s.settle(t, func(st player.State) bool { return st.Index == 0 })
	tt.Frame()
	click(t, s, tt, "Like")
	for deadline := time.Now().Add(5 * time.Second); !s.a.IsLiked(testTracks[0].ID); time.Sleep(5 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the like button didn't like the track")
		}
	}
	s.drain()
	tt.Frame()
	if err := tt.Click("Remove from liked songs"); err != nil {
		t.Errorf("the like button doesn't show the track is liked: %v", err)
	}
}

func TestOpen(t *testing.T) {
	s := newTestShell(t)
	s.panel = ""
	s.open("/lyrics")
	if s.panel != "lyrics" {
		t.Errorf("mixmusic://lyrics shows the panel %q", s.panel)
	}
	s.open("/album/al-1?from=link")
	if got := s.router.Path(); got != "/album/al-1" {
		t.Errorf("mixmusic://album/al-1 went to %q", got)
	}
	s.open("/search?q=daft+punk")
	if got := s.router.Path(); got != "/search" || s.query != "daft punk" {
		t.Errorf("mixmusic://search?q=daft+punk went to %q, the search box holding %q", got, s.query)
	}
}
