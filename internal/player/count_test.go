package player

import (
	"slices"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// noLength is the tracks of ids as a shelf of YouTube Music's home lists
// them, as Quick picks does: without their length.
func noLength(ids ...string) []media.Track {
	out := tracks(ids...)
	for i := range out {
		out[i].DurationMs = 0
	}
	return out
}

// played is the tracks the player counted, by ID.
func (h *harness) played() []string {
	var out []string
	h.svc.with(func(f *fakeServices) { out = slices.Clone(f.played) })
	return out
}

// noneCounted fails the test when the player counted a play.
func (h *harness) noneCounted(when string) {
	h.t.Helper()
	h.on(func(p *Player) {
		if i := p.indexOf(p.counted); i >= 0 {
			h.t.Errorf("%s counted %s", p.s.Queue[i].Track.ID, when)
		}
	})
}

// TestCountWithoutLength counts a track listed without its length once it
// played countAfter, or half the length its stream tells: not as its
// stream opens, which tells no length yet, nor when skipped a moment
// later.
func TestCountWithoutLength(t *testing.T) {
	h := newHarness(t, Saved{})
	h.p.PlayTracks(noLength("a", "b", "c"), 0)
	h.wait("a playing", playing("a"))
	h.noneCounted("as it started")
	h.p.Time(2*time.Second, 0)
	h.noneCounted("after 2 seconds")
	h.next()
	h.noneCounted("as it started")
	h.p.Time(countAfter-time.Second, 0)
	h.noneCounted("short of countAfter")
	h.p.Time(countAfter, 0)
	h.wait("b counted", func(*Player) bool { return slices.Equal(h.played(), []string{"yt:b"}) })
	h.next()
	h.p.Time(21*time.Second, 40*time.Second)
	h.wait("c counted", func(*Player) bool { return slices.Equal(h.played(), []string{"yt:b", "yt:c"}) })
}

// TestFailedTrackNotCounted counts neither a track listed without its
// length whose stream fails as it opens, nor the next one as it starts.
func TestFailedTrackNotCounted(t *testing.T) {
	h := newHarness(t, Saved{})
	h.stream.silence("a")
	h.p.PlayTracks(noLength("a", "b"), 0)
	// The engine tells the time of a stream that opens, then that it
	// failed, and again for its one try.
	h.wait("a loaded", func(p *Player) bool { return p.loaded })
	h.p.Time(0, 0)
	h.p.Error("the stream failed")
	h.wait("a tried", func(p *Player) bool { return retries(h, "a") == 1 && p.loaded })
	h.p.Time(0, 0)
	h.p.Error("the stream failed")
	h.wait("b playing", playing("b"))
	h.noneCounted("as it started")
	h.p.Time(2*time.Second, 0)
	h.noneCounted("after 2 seconds")
}
