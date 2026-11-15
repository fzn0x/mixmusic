package main

import (
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
)

// TestTrackLengthAgrees: the track that plays shows one length in the
// player bar, its row of the queue and the mini player, though its stream
// is a fraction of a second shorter than the length YouTube Music lists,
// as アイドル's 213.261 s is of its 3:34 (#718).
func TestTrackLengthAgrees(t *testing.T) {
	s := newTestShell(t)
	idol := testTrack("yt:idol", "アイドル", "YOASOBI", "アイドル", media.SourceYouTube)
	idol.DurationMs = 214000
	s.p.PlayTracks([]media.Track{idol}, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	stream := 213261 * time.Millisecond
	s.p.Time(time.Second, stream)
	s.settle(t, func(st player.State) bool { return st.Duration == stream })

	s.panel = "queue"
	tt := tester(t, s)
	texts := tt.Texts()
	if n := countText(texts, "3:34"); n < 2 || slices.Contains(texts, "3:33") {
		t.Errorf("the player bar and the queue's row show %d of 3:34, want both: %q", n, texts)
	}

	mini := ui.NewTester(s.miniView, 420, 132)
	mini.Frame()
	if !mini.HasText("0:01 / 3:34") {
		t.Errorf("the mini player doesn't show 0:01 / 3:34: %q", mini.Texts())
	}
}
