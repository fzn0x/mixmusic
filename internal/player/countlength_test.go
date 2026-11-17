package player

import (
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// recordingServices keeps the tracks the player counted, as it recorded
// them: with the length they had.
type recordingServices struct {
	Services
	mu   *sync.Mutex
	seen *[]media.Track
}

func (s recordingServices) RecordPlay(t media.Track) {
	s.mu.Lock()
	defer s.mu.Unlock()
	*s.seen = append(*s.seen, t)
}

// lengthHarness is a harness whose counted tracks are in recorded; live
// makes the tracks of "live" IDs live streams.
func lengthHarness(t *testing.T, live bool) (*harness, func() []media.Track) {
	t.Helper()
	var mu sync.Mutex
	var seen []media.Track
	h := newHarnessWith(t, Saved{}, func(f *fakeServices) Services {
		var inner Services = f
		if live {
			inner = liveServices{f}
		}
		return recordingServices{inner, &mu, &seen}
	})
	return h, func() []media.Track {
		mu.Lock()
		defer mu.Unlock()
		return append([]media.Track(nil), seen...)
	}
}

// recordsLength plays a of no length for countAfter, once the stream told
// its length, and expects the one play to carry that length.
func recordsLength(t *testing.T, h *harness, recorded func() []media.Track, start func()) {
	t.Helper()
	start()
	h.wait("a playing", playing("a"))
	h.p.Time(countAfter, 3*time.Minute)
	h.wait("a counted", func(*Player) bool { return len(recorded()) == 1 })
	got := recorded()[0]
	if got.ID != "yt:a" || got.DurationMs != 180000 {
		t.Errorf("recorded %s of %d ms, want yt:a of 180000", got.ID, got.DurationMs)
	}
}

// TestShelfPlaysRecordStreamLength records the length the stream told for
// a track a shelf listed without one, whichever way the shelf starts it.
func TestShelfPlaysRecordStreamLength(t *testing.T) {
	starts := map[string]func(h *harness){
		"a song of a shelf": func(h *harness) { h.p.PlayTracks(noLength("a", "b"), 0) },
		"a song with radio": func(h *harness) { h.p.PlayWithRadio(noLength("a")[0]) },
		"a collection":      func(h *harness) { h.p.PlayAll(noLength("a", "b"), false) },
		"a list":            func(h *harness) { h.p.PlayList(noLength("a", "b"), false) },
	}
	for name, start := range starts {
		t.Run(name, func(t *testing.T) {
			h, recorded := lengthHarness(t, false)
			recordsLength(t, h, recorded, func() { start(h) })
		})
	}
}

// TestNextShelfSongRecordsStreamLength records the length of a track that
// the queue moved on to, as it does of one started from a shelf.
func TestNextShelfSongRecordsStreamLength(t *testing.T) {
	h, recorded := lengthHarness(t, false)
	h.p.PlayTracks(noLength("x", "a"), 0)
	h.wait("x playing", playing("x"))
	h.next()
	recordsLength(t, h, recorded, func() {})
}

// TestListedLengthKept leaves the length a list gave as it is, whatever
// the stream tells.
func TestListedLengthKept(t *testing.T) {
	h, recorded := lengthHarness(t, false)
	a := track("a", "A")
	a.DurationMs = 200000
	h.p.PlayTracks([]media.Track{a}, 0)
	h.wait("a playing", playing("a"))
	h.p.Time(countAfter, 3*time.Minute)
	h.wait("a counted", func(*Player) bool { return len(recorded()) == 1 })
	if got := recorded()[0].DurationMs; got != 200000 {
		t.Errorf("recorded %d ms, want the listed 200000", got)
	}
}

// TestLiveShelfSongStaysAtZero records a live stream of no length at 0:
// it has no length to give (#544 counts its time).
func TestLiveShelfSongStaysAtZero(t *testing.T) {
	h, recorded := lengthHarness(t, true)
	h.p.PlayTracks([]media.Track{liveTrack("liveRadio01")}, 0)
	h.wait("the live stream playing", playing("liveRadio01"))
	h.p.Time(countAfter, 3*time.Minute)
	h.wait("counted", func(*Player) bool { return len(recorded()) == 1 })
	if got := recorded()[0].DurationMs; got != 0 {
		t.Errorf("recorded %d ms, want 0", got)
	}
}
