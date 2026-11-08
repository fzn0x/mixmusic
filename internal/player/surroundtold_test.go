package player

import (
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// sourceOf is the current entry's source, on the player's goroutine.
func sourceOf(p *Player) Source {
	if p.s.Source == nil {
		return Source{}
	}
	return *p.s.Source
}

// TestSurroundFoundLater offers the 5.1 audio YouTube told of after a
// track started, which plays it in stereo: with 5.1 off, the track only
// shows it has some; with 5.1 on, it plays it from where it is, as turning
// 5.1 on does. What is told of another stream changes nothing.
func TestSurroundFoundLater(t *testing.T) {
	h := newHarness(t, Saved{})
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	loads := h.stream.count("load")

	// Of another stream: nothing.
	h.p.SurroundFound("url:yt:z", "url:yt:z/5.1")
	h.on(func(p *Player) {
		if got := sourceOf(p).SurroundURL; got != "" {
			t.Errorf("told of another stream, a has %q", got)
		}
	})

	// 5.1 off: a has it, and plays on in stereo.
	h.p.Time(3*time.Second, 200*time.Second)
	h.p.SurroundFound("url:yt:a", "url:yt:a/5.1")
	h.wait("a has 5.1 audio", func(p *Player) bool { return sourceOf(p).SurroundURL == "url:yt:a/5.1" })
	if n := h.stream.count("load"); n != loads {
		t.Errorf("with 5.1 off, it loaded %q", h.stream.lastLoad())
	}
	// The windows, which read the state, show the 5.1 button.
	if st := h.p.State(); st.Source == nil || st.Source.SurroundURL != "url:yt:a/5.1" || !st.Playing {
		t.Errorf("the state offers no 5.1 audio of a: %+v", st.Source)
	}
	// Turned on, it plays it.
	s := h.svc.Settings()
	s.Surround = true
	h.svc.with(func(f *fakeServices) { f.settings = s })
	h.p.SettingsChanged(s)
	h.wait("a in 5.1", func(*Player) bool { return h.stream.lastLoad() == "load url:yt:a/5.1 3s true" })

	// 5.1 on: b, told of later, plays its 5.1 audio from where it is.
	h.p.Next()
	h.wait("b playing", playing("b"))
	if got := h.stream.lastLoad(); got != "load url:yt:b 0s true" {
		t.Fatalf("b loaded %q", got)
	}
	h.p.Time(2*time.Second, 200*time.Second)
	h.p.SurroundFound("url:yt:b", "url:yt:b/5.1")
	h.wait("b in 5.1", func(*Player) bool { return h.stream.lastLoad() == "load url:yt:b/5.1 2s true" })
	// Told again, it loads nothing more.
	loads = h.stream.count("load")
	h.p.SurroundFound("url:yt:b", "url:yt:b/5.1")
	h.on(func(*Player) {})
	if n := h.stream.count("load"); n != loads {
		t.Errorf("told again, it loaded %q", h.stream.lastLoad())
	}
}

// TestSurroundFoundWhileItLoads has a track whose stream YouTube told of
// 5.1 audio of while the app was still preparing it take that audio as it
// loads: with 5.1 on, it starts in 5.1.
func TestSurroundFoundWhileItLoads(t *testing.T) {
	prepared := make(chan struct{})
	release := make(chan struct{})
	h := newHarness(t, Saved{}, func(f *fakeServices) {
		f.settings.Surround = true
		f.prepare = func(t media.Track) (Source, error) {
			if t.ID == "yt:c" {
				close(prepared)
				<-release
			}
			return streamOf(t), nil
		}
	})
	h.p.PlayTracks(tracks("c"), 0)
	<-prepared
	h.p.SurroundFound("url:yt:c", "url:yt:c/5.1")
	h.on(func(*Player) {})
	close(release)
	h.wait("c playing", playing("c"))
	if got := h.stream.lastLoad(); got != "load url:yt:c/5.1 0s true" {
		t.Errorf("c loaded %q", got)
	}
	h.on(func(p *Player) {
		if got := sourceOf(p).SurroundURL; got != "url:yt:c/5.1" {
			t.Errorf("c has %q", got)
		}
	})
}
