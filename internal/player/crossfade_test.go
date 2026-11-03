package player

import (
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// pausedInPreparation plays a and b with a crossfade of 5 s, and pauses a
// while its crossfade gets b ready, whose answer comes once a paused.
func pausedInPreparation(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t, Saved{}, func(f *fakeServices) { f.settings.Crossfade = 5 })
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	// b is found and preloaded first, as a plays on: the crossfade's is
	// then the one preparation of b left.
	h.p.Time(11*time.Second, 200*time.Second)
	h.wait("b preloaded", func(*Player) bool { return h.stream.has("preload url:yt:b") })
	asked, answer := make(chan struct{}), make(chan struct{})
	var once sync.Once
	h.svc.with(func(f *fakeServices) {
		f.prepare = func(tr media.Track) (Source, error) {
			if tr.ID == "yt:b" {
				once.Do(func() { close(asked) })
				<-answer
			}
			return streamOf(tr), nil
		}
	})
	h.p.Time(196*time.Second, 200*time.Second)
	select {
	case <-asked:
	case <-time.After(5 * time.Second):
		t.Fatalf("the crossfade didn't prepare b; the stream did %q", h.stream.copy())
	}
	h.p.Pause()
	h.wait("a paused", func(p *Player) bool { return p.paused && !p.s.Playing })
	close(answer)
	h.svc.with(func(f *fakeServices) { f.prepare = nil })
	h.wait("b's answer", func(p *Player) bool { return !p.crossfading || h.stream.count("crossfade") > 0 })
	return h
}

// stillPaused fails unless a is the current track, paused, and b didn't
// fade in.
func stillPaused(t *testing.T, h *harness) {
	t.Helper()
	h.on(func(p *Player) {
		cur, _ := p.s.Current()
		if cur.Track.ID != "yt:a" || p.s.Playing || !p.paused || h.stream.sounding() != "" {
			t.Errorf("a pause undone: current %s, playing %v, paused %v, sounding %q", cur.Track.ID, p.s.Playing, p.paused, h.stream.sounding())
		}
	})
	if n := h.stream.count("crossfade"); n != 0 {
		t.Errorf("b faded in while a was paused: %q", h.stream.copy())
	}
}

// TestCrossfadeHeldByPause keeps a pause made while the crossfade gets the
// next track ready: nothing plays, and a time told while paused, as a seek
// tells, doesn't prepare it again.
func TestCrossfadeHeldByPause(t *testing.T) {
	h := pausedInPreparation(t)
	stillPaused(t, h)
	prepared := func() (n int) {
		h.svc.with(func(f *fakeServices) {
			for _, id := range f.prepared {
				if id == "yt:b" {
					n++
				}
			}
		})
		return n
	}
	before := prepared()
	h.p.Time(197*time.Second, 200*time.Second)
	h.on(func(*Player) {})
	time.Sleep(20 * time.Millisecond)
	h.on(func(*Player) {})
	stillPaused(t, h)
	if n := prepared(); n != before {
		t.Errorf("b prepared %d times more while a was paused", n-before)
	}
}

// TestCrossfadeAfterPause crossfades into the next track once the one
// paused plays again, while there's time.
func TestCrossfadeAfterPause(t *testing.T) {
	h := pausedInPreparation(t)
	h.p.Play()
	h.wait("a playing again", playing("a"))
	h.p.Time(197*time.Second, 200*time.Second)
	h.wait("b crossfading in", playing("b"))
	if got := h.stream.with("crossfade"); !slices.Equal(got, []string{"crossfade url:yt:b 5s"}) {
		t.Errorf("crossfaded %q", got)
	}
}

// TestEndAfterPause plays the next track as the one paused ends, once it
// plays again, as without a crossfade.
func TestEndAfterPause(t *testing.T) {
	h := pausedInPreparation(t)
	h.p.Play()
	h.wait("a playing again", playing("a"))
	h.p.Ended()
	h.wait("b playing", playing("b"))
	if got := h.stream.lastLoad(); got != "load url:yt:b 0s true" {
		t.Errorf("b loaded as %q", got)
	}
	if n := h.stream.count("crossfade"); n != 0 {
		t.Errorf("crossfaded too: %q", h.stream.copy())
	}
}
