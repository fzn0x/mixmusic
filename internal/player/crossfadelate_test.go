package player

import (
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// holdFade has the fake engine hold its crossfades, or end them.
func (s *fakeStream) holdFade(on bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fading = on
}

// fadeAsked returns how often the player asked whether a crossfade plays.
func (s *fakeStream) fadeAsked() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.fadeAsks
}

// TestCrossfadeEndsWithTheEngines crossfades into b, whose 5.1 audio is
// made ready during the fade, as the engine holds the fade past its
// seconds: while b's audio is late, or while paused. b's 5.1 audio, which
// loads b again, waits for the engine's fade to end, rather than cut the
// track fading out off (#549).
func TestCrossfadeEndsWithTheEngines(t *testing.T) {
	for _, paused := range []bool{false, true} {
		h, r := newReadyHarness(t, func(f *fakeServices) {
			f.settings.Surround = true
			f.settings.Crossfade = 1
			f.prepare = func(t media.Track) (Source, error) { return surroundOf(t), nil }
		})
		r.setReady("url:yt:a/5.1")
		h.p.PlayTracks(tracks("a", "b"), 0)
		h.wait("a playing", playing("a"))
		h.stream.holdFade(true)
		h.p.Time(199*time.Second, 200*time.Second)
		h.wait("b crossfaded", func(*Player) bool { return h.stream.has("crossfade url:yt:b 1s") })
		if paused {
			h.p.Pause()
			h.wait("b paused", func(p *Player) bool { return p.paused })
		}
		loads := h.stream.count("load")
		r.call(h, 0, "url:yt:b/5.1").answer <- nil
		h.wait("b's 5.1 audio ready", func(p *Player) bool { return p.readyURL == "url:yt:b/5.1" })
		// The fade's second passes: the player asks the engine, which still
		// fades.
		h.wait("the fade's end asked", func(*Player) bool {
			return h.stream.fadeAsked() > 0 || h.stream.count("load") != loads
		})
		if h.stream.count("load") != loads {
			t.Fatalf("paused %v: b's 5.1 audio cut off the fade the engine held: %q", paused, h.stream.copy())
		}
		h.on(func(p *Player) {
			if !p.crossfading {
				t.Errorf("paused %v: the crossfade ended before the engine's", paused)
			}
		})
		if paused {
			h.p.Play()
		}
		h.stream.holdFade(false)
		h.wait("b in 5.1", func(*Player) bool { return h.stream.lastLoad() == "load url:yt:b/5.1 0s true" })
		h.on(func(p *Player) {
			if p.crossfading {
				t.Errorf("paused %v: the crossfade didn't end with the engine's", paused)
			}
		})
	}
}
