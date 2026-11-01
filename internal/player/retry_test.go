package player

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// retries counts the loads that played the stream of id again.
func retries(h *harness, id string) int { return h.stream.count("load url:yt:" + id + "?retry=") }

// useRetries has the stream of id break off until it ran out of tries,
// each played again where it was, after from tries before.
func useRetries(t *testing.T, h *harness, id string, from int) {
	t.Helper()
	for i := range retryWaits {
		h.p.Error("the stream failed")
		h.wait(fmt.Sprintf("try %d", from+i+1), func(p *Player) bool { return retries(h, id) == from+i+1 && p.loaded })
	}
}

// longWaits has the tries wait an hour for the test, which sees the state
// while they wait. Restored after the player closes, as cleanups run last
// registered first: call it before newHarness.
func longWaits(t *testing.T, n int) {
	waits := retryWaits
	retryWaits = make([]time.Duration, n)
	for i := range retryWaits {
		retryWaits[i] = time.Hour
	}
	t.Cleanup(func() { retryWaits = waits })
}

// TestStreamRetryHolds starts the tries over once the stream plays on,
// and only then prepares the next track again, which the engine let go of.
func TestStreamRetryHolds(t *testing.T) {
	h := newHarness(t, Saved{})
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	h.p.Time(42*time.Second, 200*time.Second)
	h.wait("b preloaded", func(*Player) bool { return h.stream.count("preload url:yt:b") == 1 })
	useRetries(t, h, "a", 0)
	// Short of retryHold, the stream doesn't hold yet.
	h.p.Time(42*time.Second+retryHold-time.Second, 200*time.Second)
	h.on(func(p *Player) {
		if p.retry.n != len(retryWaits) {
			t.Errorf("%d tries left after %s", len(retryWaits)-p.retry.n, retryHold-time.Second)
		}
	})
	if n := h.stream.count("preload"); n != 1 {
		t.Errorf("the next track loaded again before the stream held: %q", h.stream.copy())
	}
	h.p.Time(42*time.Second+retryHold, 200*time.Second)
	h.wait("b preloaded again", func(*Player) bool { return h.stream.count("preload url:yt:b") == 2 })
	h.p.Error("the stream failed")
	h.wait("a tried once more", func(p *Player) bool { return retries(h, "a") == len(retryWaits)+1 && p.loaded })
	if got := h.stream.lastLoad(); !strings.HasSuffix(got, " 52s true") {
		t.Errorf("tried with %q", got)
	}
	if n := h.notices(); len(n) != 0 {
		t.Errorf("notices %+v", n)
	}
}

// TestStreamRetryEachPlay gives each play of an entry its tries: played
// again, it doesn't skip at its first break.
func TestStreamRetryEachPlay(t *testing.T) {
	t.Run("repeat one", func(t *testing.T) {
		h := newHarness(t, Saved{})
		h.p.PlayTracks(tracks("a", "b"), 0)
		h.wait("a playing", playing("a"))
		h.on(func(p *Player) { p.s.Repeat = RepeatOne })
		h.p.Time(5*time.Second, 200*time.Second)
		useRetries(t, h, "a", 0)
		loads := h.stream.count("load")
		h.p.Ended()
		h.wait("a again", func(p *Player) bool { return h.stream.count("load") == loads+1 && playing("a")(p) })
		h.p.Time(5*time.Second, 200*time.Second)
		useRetries(t, h, "a", len(retryWaits))
	})
	t.Run("previous", func(t *testing.T) {
		h := newHarness(t, Saved{})
		h.p.PlayTracks(tracks("a", "b"), 0)
		h.wait("a playing", playing("a"))
		h.p.Time(5*time.Second, 200*time.Second)
		useRetries(t, h, "a", 0)
		h.p.Next()
		h.wait("b playing", playing("b"))
		h.p.Previous()
		h.wait("a playing again", playing("a"))
		h.p.Time(5*time.Second, 200*time.Second)
		useRetries(t, h, "a", len(retryWaits))
	})
}

// TestStreamErrorAtStart tries a stream that never played once, at once,
// as one YouTube refused seldom plays a moment later.
func TestStreamErrorAtStart(t *testing.T) {
	longWaits(t, 3)
	h := newHarness(t, Saved{}, func(f *fakeServices) { f.failure = "YouTube refused to send its audio" })
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	// The engine tells where the stream opens, and that it failed.
	h.p.Time(0, 0)
	h.p.Error("the stream failed")
	h.wait("a tried", func(p *Player) bool { return retries(h, "a") == 1 && p.loaded })
	h.p.Time(0, 0)
	h.p.Error("the stream failed")
	h.wait("b playing", playing("b"))
	if n := h.notices(); len(n) != 1 || n[0].Message != "Couldn't play Song a: YouTube refused to send its audio" {
		t.Errorf("notices %+v", n)
	}
}

// TestStreamRetryWaits shows a try that waits as a track that loads, and
// lets a seek or another track take over.
func TestStreamRetryWaits(t *testing.T) {
	longWaits(t, 1)
	broken := func(t *testing.T) *harness {
		h := newHarness(t, Saved{})
		h.p.PlayTracks(tracks("a", "b"), 0)
		h.wait("a playing", playing("a"))
		h.p.Time(42*time.Second, 200*time.Second)
		// As the engine tells of a stream that failed.
		h.p.Playing(false)
		h.p.Error("the stream failed")
		h.wait("a waiting", func(p *Player) bool { return p.s.Loading })
		return h
	}
	t.Run("loading", func(t *testing.T) {
		h := broken(t)
		plays := h.stream.count("play")
		h.p.Toggle()
		h.on(func(p *Player) {
			if !p.s.Buffering || p.loaded || p.s.Error != "" || p.s.Position != 42*time.Second {
				t.Errorf("waiting: buffering %v, loaded %v, error %q, position %v", p.s.Buffering, p.loaded, p.s.Error, p.s.Position)
			}
		})
		if got := h.stream.count("play"); got != plays {
			t.Errorf("the toggle played the stream that failed: %q", h.stream.copy())
		}
		h.svc.with(func(f *fakeServices) {
			if last := f.reports[len(f.reports)-1]; last.Playing || !last.Loading {
				t.Errorf("reported as %+v", last)
			}
		})
	})
	t.Run("seek", func(t *testing.T) {
		h := broken(t)
		h.p.Seek(50 * time.Second)
		h.wait("a at 50s", func(p *Player) bool { return h.stream.lastLoad() == "load url:yt:a 50s true" && playing("a")(p) })
	})
	t.Run("next", func(t *testing.T) {
		h := broken(t)
		h.p.Next()
		h.wait("b playing", playing("b"))
		if n := retries(h, "a"); n != 0 {
			t.Errorf("a tried %d times after Next", n)
		}
	})
}

// TestStreamRetryPaused tries a stream that broke off while paused, as
// after a seek, paused: it stays paused.
func TestStreamRetryPaused(t *testing.T) {
	h := newHarness(t, Saved{})
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	h.p.Time(42*time.Second, 200*time.Second)
	h.p.Pause()
	h.wait("a paused", func(p *Player) bool { return !p.s.Playing })
	h.p.Playing(false)
	h.p.Error("the stream failed")
	h.wait("a tried", func(p *Player) bool { return retries(h, "a") == 1 && p.loaded })
	if got := h.stream.lastLoad(); !strings.HasSuffix(got, " 42s false") {
		t.Errorf("tried with %q", got)
	}
	h.on(func(p *Player) {
		if p.s.Playing || p.s.Loading {
			t.Errorf("tried paused: playing %v, loading %v", p.s.Playing, p.s.Loading)
		}
	})
	// Play plays it from there.
	h.p.Play()
	h.wait("a playing again", playing("a"))
}

// TestStreamRetryNotFound counts a try that finds no stream, and fails
// once the tries ran out, with why the last one did rather than why the
// stream failed before.
func TestStreamRetryNotFound(t *testing.T) {
	h := newHarness(t, Saved{}, func(f *fakeServices) {
		f.failure = "MixMusic could not reach YouTube's audio servers"
		f.streamErr = errors.New("YouTube asks to confirm this is not a bot.")
	})
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	h.p.Time(42*time.Second, 200*time.Second)
	h.p.Error("the stream failed")
	h.wait("b playing", playing("b"))
	n := h.notices()
	if len(n) != 1 || n[0].Message != "Couldn't play Song a: YouTube asks to confirm this is not a bot." {
		t.Errorf("notices %+v", n)
	}
	if got := retries(h, "a"); got != 0 {
		t.Errorf("%d tries loaded", got)
	}
	h.svc.with(func(f *fakeServices) {
		if f.streamed != len(retryWaits) {
			t.Errorf("looked for the stream %d times", f.streamed)
		}
	})
}

// TestStreamErrorOfALoad ignores what the engine tells of a stream it no
// longer has: a load took over.
func TestStreamErrorOfALoad(t *testing.T) {
	found := make(chan struct{})
	release := sync.OnceFunc(func() { close(found) })
	h := newHarness(t, Saved{}, func(f *fakeServices) {
		f.prepare = func(t media.Track) (Source, error) {
			if t.ID == "yt:b" {
				<-found
			}
			return streamOf(t), nil
		}
	})
	t.Cleanup(release)
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	h.p.Next()
	h.wait("b loading", func(p *Player) bool { return p.s.Index == 1 && p.s.Loading })
	h.p.Error("the stream failed")
	h.on(func(p *Player) {
		if p.retry.n != 0 {
			t.Errorf("b tried for a's stream")
		}
	})
	release()
	h.wait("b playing", playing("b"))
}
