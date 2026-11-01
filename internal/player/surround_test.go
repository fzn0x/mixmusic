package player

import (
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// surroundOf is the Source of a track of 5.1 surround audio beside its
// stereo audio, as the app prepares it.
func surroundOf(t media.Track) Source {
	s := streamOf(t)
	s.SurroundURL = "url:" + t.ID + "/5.1"
	return s
}

// TestSurroundSwitches turns 5.1 surround on and off while tracks play:
// the current track plays its 5.1 audio, or its stereo audio, from where
// it was, and later tracks start in the audio asked for.
func TestSurroundSwitches(t *testing.T) {
	h := newHarness(t, Saved{}, func(f *fakeServices) {
		f.prepare = func(t media.Track) (Source, error) {
			if t.ID == "yt:c" {
				return streamOf(t), nil // of no 5.1 audio
			}
			return surroundOf(t), nil
		}
	})
	h.p.PlayTracks(tracks("a", "b", "c"), 0)
	h.wait("a playing", playing("a"))
	if got := h.stream.lastLoad(); got != "load url:yt:a 0s true" {
		t.Fatalf("with 5.1 off, loaded %q", got)
	}
	setSurround := func(on bool) {
		s := h.svc.Settings()
		s.Surround = on
		h.svc.with(func(f *fakeServices) { f.settings = s })
		h.p.SettingsChanged(s)
	}

	// On, at 42 s of a: its 5.1 audio from there.
	h.p.Time(42*time.Second, 200*time.Second)
	setSurround(true)
	h.wait("a in 5.1", func(p *Player) bool { return h.stream.lastLoad() == "load url:yt:a/5.1 42s true" })
	h.wait("a playing", playing("a"))

	// Off while paused: its stereo audio, paused.
	h.p.Toggle()
	h.wait("a paused", func(p *Player) bool { return !p.s.Playing })
	h.p.Time(50*time.Second, 200*time.Second)
	setSurround(false)
	h.wait("a in stereo", func(p *Player) bool { return h.stream.lastLoad() == "load url:yt:a 50s false" })
	loads := h.stream.count("load")
	setSurround(false) // no change, no load
	h.on(func(*Player) {})
	if n := h.stream.count("load"); n != loads {
		t.Errorf("%d loads after setting what was set", n-loads)
	}

	// On: the next track starts in 5.1, and one of no 5.1 audio in stereo,
	// which turning 5.1 off leaves playing.
	setSurround(true)
	h.wait("a in 5.1", func(p *Player) bool { return h.stream.lastLoad() == "load url:yt:a/5.1 50s false" })
	h.p.Next()
	h.wait("b playing", playing("b"))
	if got := h.stream.lastLoad(); got != "load url:yt:b/5.1 0s true" {
		t.Errorf("the next track loaded %q", got)
	}
	h.p.Next()
	h.wait("c playing", playing("c"))
	if got := h.stream.lastLoad(); got != "load url:yt:c 0s true" {
		t.Errorf("a track of no 5.1 audio loaded %q", got)
	}
	loads = h.stream.count("load")
	setSurround(false)
	h.on(func(*Player) {})
	if n := h.stream.count("load"); n != loads {
		t.Errorf("a track of no 5.1 audio loaded again: %q", h.stream.lastLoad())
	}
}

// TestSurroundPreloads preloads and crossfades into the next track in the
// audio asked for.
func TestSurroundPreloads(t *testing.T) {
	h := newHarness(t, Saved{}, func(f *fakeServices) {
		f.settings.Surround = true
		f.settings.Crossfade = 5
		f.prepare = func(t media.Track) (Source, error) { return surroundOf(t), nil }
	})
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	if got := h.stream.lastLoad(); got != "load url:yt:a/5.1 0s true" {
		t.Fatalf("with 5.1 on, loaded %q", got)
	}
	h.p.Time(prefetchAfter+time.Second, 200*time.Second)
	h.wait("b preloaded", func(*Player) bool { return h.stream.has("preload url:yt:b/5.1") })
	h.p.Time(196*time.Second, 200*time.Second)
	h.wait("b crossfaded", func(*Player) bool { return h.stream.has("crossfade url:yt:b/5.1 5s") })
}
