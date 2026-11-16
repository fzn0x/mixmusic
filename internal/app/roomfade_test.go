package app

import (
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/room/roomtest"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// Crossfade plays url from its start at once, as the engine's next deck
// does, in place of a fade.
func (e *roomEngine) Crossfade(url string, _ time.Duration, _ audio.Cue) error {
	return e.Load(url, audio.Cue{Autoplay: true})
}

// TestRoomCrossfadeWaits crossfades on a host whose guest never has the
// next track ready: the track that ends plays on while the next waits for
// the guest, which the guests are told as a track that loads, at its
// start; then the fade starts.
func TestRoomCrossfadeWaits(t *testing.T) {
	n := roomtest.New()
	desk := newRoomApp(t, n, "Desk")
	lap := newRoomApp(t, n, "Lap")
	setRules(t, desk.App, func(s *settings.Settings) { s.Crossfade = 6 })
	dp := desk.GoPlayer()
	dp.PlayTracks(tracksOf("a", "b"), 0)
	waitPlayer(t, dp, "a playing", func(s player.State) bool { return s.Playing })
	if err := desk.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	joinRoom(t, lap, desk)
	// The guest stops following, as a computer too busy to: it never
	// tells it has b ready.
	lap.room.mu.Lock()
	f := lap.room.guest.follow
	lap.room.mu.Unlock()
	f.stop()
	dp.Seek(300*time.Second - 5*time.Second)
	waitPlayer(t, dp, "the fade into b asked", func(s player.State) bool { return s.Index == 1 })
	until(t, "b held", func() bool { h, _ := desk.Rooms().Hosted(); return h.Waiting })
	if url, playing := desk.engine.state(); url != "url:"+vid("a") || !playing {
		t.Fatalf("while b waits the engine plays %q (%v)", url, playing)
	}
	st := roomHostPlayer{desk.App}.State()
	if st.Track == nil || st.Track.ID != vid("b") || !st.Loading || st.Position != 0 {
		t.Fatalf("while b waits, the guests are told %+v", st)
	}
	held := time.Now()
	until(t, "the fade started", func() bool { url, _ := desk.engine.state(); return url == "url:"+vid("b") })
	// Half the fade at most, less the lead, and the gate's 2 s at most.
	if waited := time.Since(held); waited > 2500*time.Millisecond {
		t.Fatalf("b waited %v", waited)
	}
	if st := (roomHostPlayer{desk.App}).State(); st.Loading {
		t.Fatalf("b plays, and the guests are told %+v", st)
	}
}
