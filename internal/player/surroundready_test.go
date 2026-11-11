package player

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// readyServices are fakeServices whose streams' 5.1 surround audio the app
// makes ready as the test answers: the URLs in ready play at once, and each
// preparation waits for the test's answer, whatever its context, as a slow
// app might, so that the player alone must drop what it no longer wants.
type readyServices struct {
	*fakeServices
	mu    sync.Mutex
	ready map[string]bool
	calls []*readyCall
	ended chan struct{}
}

// readyCall is a preparation the player asked for.
type readyCall struct {
	url    string
	ctx    context.Context
	answer chan error
}

func (s *readyServices) SurroundReady(src Source) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ready[src.SurroundURL]
}

func (s *readyServices) PrepareSurround(ctx context.Context, src Source) error {
	c := &readyCall{url: src.SurroundURL, ctx: ctx, answer: make(chan error, 1)}
	s.mu.Lock()
	s.calls = append(s.calls, c)
	s.mu.Unlock()
	select {
	case err := <-c.answer:
		if err == nil {
			s.setReady(c.url)
		}
		return err
	case <-s.ended:
		return context.Canceled
	}
}

func (s *readyServices) setReady(url string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ready[url] = true
}

// asked is the preparations asked for, in turn.
func (s *readyServices) asked() []*readyCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.calls)
}

// call waits for the nth preparation, from 0, and fails unless it is of url.
func (s *readyServices) call(h *harness, n int, url string) *readyCall {
	h.t.Helper()
	h.wait("preparation "+url, func(*Player) bool { return len(s.asked()) > n })
	c := s.asked()[n]
	if c.url != url {
		h.t.Fatalf("preparation %d is of %s, not %s", n, c.url, url)
	}
	return c
}

// taken waits until a preparation of url returned that it is ready, and
// then a few turns of the player's goroutine, to which the answer comes
// back right after: a check after it sees what the answer did, if
// anything.
func (s *readyServices) taken(h *harness, url string) {
	h.t.Helper()
	h.wait("the answer of "+url, func(*Player) bool { return s.SurroundReady(Source{SurroundURL: url}) })
	for range 3 {
		h.on(func(*Player) {})
	}
}

// newReadyHarness is a harness whose app makes 5.1 audio ready as the test
// answers.
func newReadyHarness(t *testing.T, setup ...func(*fakeServices)) (*harness, *readyServices) {
	t.Helper()
	r := &readyServices{ready: map[string]bool{}, ended: make(chan struct{})}
	t.Cleanup(func() { close(r.ended) })
	h := newHarnessWith(t, Saved{}, func(f *fakeServices) Services {
		r.fakeServices = f
		return r
	}, setup...)
	return h, r
}

// setSurround turns 5.1 on or off in the settings.
func (h *harness) setSurround(on bool) {
	s := h.svc.Settings()
	s.Surround = on
	h.svc.with(func(f *fakeServices) { f.settings = s })
	h.p.SettingsChanged(s)
}

// TestSurroundStartsInStereo plays, with 5.1 on, a track whose 5.1 audio
// isn't ready: it starts in stereo at once, and goes on in 5.1 from where
// it is once the app made that audio ready. A track whose 5.1 audio is
// ready starts in 5.1, without a preparation (#457).
func TestSurroundStartsInStereo(t *testing.T) {
	h, r := newReadyHarness(t, func(f *fakeServices) {
		f.settings.Surround = true
		f.prepare = func(t media.Track) (Source, error) { return surroundOf(t), nil }
	})
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	if got := h.stream.lastLoad(); got != "load url:yt:a 0s true" {
		t.Fatalf("a, whose 5.1 audio isn't ready, loaded %q", got)
	}
	c := r.call(h, 0, "url:yt:a/5.1")
	// It plays on in stereo meanwhile.
	h.p.Time(4600*time.Millisecond, 200*time.Second)
	h.on(func(*Player) {})
	if n := h.stream.count("load"); n != 1 {
		t.Fatalf("while its 5.1 audio is made ready, a loaded %q", h.stream.with("load"))
	}
	c.answer <- nil
	h.wait("a in 5.1", func(*Player) bool { return h.stream.lastLoad() == "load url:yt:a/5.1 4.6s true" })
	h.wait("a playing", playing("a"))
	if n := h.stream.count("load"); n != 2 {
		t.Errorf("a loaded %q", h.stream.with("load"))
	}

	// b's 5.1 audio is ready: it starts in 5.1.
	r.setReady("url:yt:b/5.1")
	h.p.Next()
	h.wait("b playing", playing("b"))
	if got := h.stream.lastLoad(); got != "load url:yt:b/5.1 0s true" {
		t.Errorf("b, whose 5.1 audio is ready, loaded %q", got)
	}
	if n := len(r.asked()); n != 1 {
		t.Errorf("%d preparations", n)
	}
}

// TestSurroundTurnedOnPlaysStereoUntilReady turns 5.1 on while a track
// plays in stereo, its 5.1 audio not ready: the stereo audio plays on until
// the 5.1 audio is ready, which plays from where the track is then,
// playing or paused as it is (#457).
func TestSurroundTurnedOnPlaysStereoUntilReady(t *testing.T) {
	h, r := newReadyHarness(t, func(f *fakeServices) {
		f.prepare = func(t media.Track) (Source, error) { return surroundOf(t), nil }
	})
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	if got := h.stream.lastLoad(); got != "load url:yt:a 0s true" {
		t.Fatalf("with 5.1 off, loaded %q", got)
	}
	h.p.Time(12800*time.Millisecond, 200*time.Second)
	h.setSurround(true)
	c := r.call(h, 0, "url:yt:a/5.1")
	if n := h.stream.count("load"); n != 1 {
		t.Fatalf("5.1 turned on, a loaded %q before its 5.1 audio was ready", h.stream.with("load"))
	}
	h.p.Time(19300*time.Millisecond, 200*time.Second)
	c.answer <- nil
	h.wait("a in 5.1", func(*Player) bool { return h.stream.lastLoad() == "load url:yt:a/5.1 19.3s true" })
	h.wait("a playing", playing("a"))
	// Off, it goes back to stereo at once.
	h.setSurround(false)
	h.wait("a in stereo", func(*Player) bool { return strings.HasPrefix(h.stream.lastLoad(), "load url:yt:a 19.30") })

	// Paused: it goes on in 5.1, paused.
	h.p.Next()
	h.wait("b playing", playing("b"))
	h.p.Time(7*time.Second, 200*time.Second)
	h.setSurround(true)
	c = r.call(h, 1, "url:yt:b/5.1")
	h.p.Pause()
	h.wait("b paused", func(p *Player) bool { return !p.s.Playing })
	c.answer <- nil
	h.wait("b in 5.1, paused", func(*Player) bool { return h.stream.lastLoad() == "load url:yt:b/5.1 7s false" })
}

// TestSurroundReadyingStops leaves the stereo audio playing when the 5.1
// audio being made ready is no longer wanted, as 5.1 was turned off or
// another track plays, and plays from where a seek took the track the 5.1
// audio that becomes ready after it. 5.1 audio that can't be had leaves
// the track in stereo, and tells why once (#457).
func TestSurroundReadyingStops(t *testing.T) {
	h, r := newReadyHarness(t, func(f *fakeServices) {
		f.settings.Surround = true
		f.prepare = func(t media.Track) (Source, error) { return surroundOf(t), nil }
	})
	h.p.PlayTracks(tracks("a", "b", "c"), 0)
	h.wait("a playing", playing("a"))
	c := r.call(h, 0, "url:yt:a/5.1")
	loads := h.stream.count("load")

	// 5.1 off: the preparation stops, and what it found plays no more.
	h.setSurround(false)
	h.wait("a's preparation stopped", func(*Player) bool { return c.ctx.Err() != nil })
	c.answer <- nil
	r.taken(h, "url:yt:a/5.1")
	if n := h.stream.count("load"); n != loads || h.stream.sounding() != "url:yt:a" {
		t.Fatalf("5.1 turned off: loaded %q, sounding %q", h.stream.with("load"), h.stream.sounding())
	}

	// Another track: a's 5.1 audio never plays over it.
	r.mu.Lock()
	delete(r.ready, "url:yt:a/5.1")
	r.mu.Unlock()
	h.setSurround(true)
	c = r.call(h, 1, "url:yt:a/5.1")
	h.p.Next()
	h.wait("b playing", playing("b"))
	cb := r.call(h, 2, "url:yt:b/5.1")
	if c.ctx.Err() == nil {
		t.Error("a's preparation goes on as b plays")
	}
	c.answer <- nil
	r.taken(h, "url:yt:a/5.1")
	if got := h.stream.lastLoad(); got != "load url:yt:b 0s true" {
		t.Fatalf("a's 5.1 audio ready as b plays: loaded %q", got)
	}

	// A seek meanwhile: b's 5.1 audio plays from there.
	h.p.Seek(90 * time.Second)
	h.wait("b seeked", func(p *Player) bool { return h.stream.has("seek 1m30s") })
	cb.answer <- nil
	h.wait("b in 5.1", func(*Player) bool { return h.stream.lastLoad() == "load url:yt:b/5.1 1m30s true" })

	// c's 5.1 audio can't be had: it plays on in stereo, and tells why.
	h.p.Next()
	h.wait("c playing", playing("c"))
	r.call(h, 3, "url:yt:c/5.1").answer <- errors.New("YouTube has no 5.1 surround audio of this track")
	h.wait("told why", func(*Player) bool {
		for _, n := range h.notices() {
			if n.Message == "The 5.1 surround audio of Song c couldn't play (YouTube has no 5.1 surround audio of this track), so it plays in stereo" {
				return true
			}
		}
		return false
	})
	if got := h.stream.lastLoad(); got != "load url:yt:c 0s true" || h.stream.sounding() != "url:yt:c" {
		t.Errorf("c loaded %q, sounding %q", got, h.stream.sounding())
	}
	// 5.1 turned on again tries it again.
	h.setSurround(false)
	h.setSurround(true)
	r.call(h, 4, "url:yt:c/5.1")
}

// TestSurroundFoundLaterPlaysStereoUntilReady has the 5.1 audio YouTube
// told of after a track started made ready, with 5.1 on, while the track
// plays on in stereo: it plays from where the track is once ready (#457).
func TestSurroundFoundLaterPlaysStereoUntilReady(t *testing.T) {
	h, r := newReadyHarness(t, func(f *fakeServices) { f.settings.Surround = true })
	h.p.PlayTracks(tracks("a"), 0)
	h.wait("a playing", playing("a"))
	h.p.Time(2*time.Second, 200*time.Second)
	h.p.SurroundFound("url:yt:a", "url:yt:a/5.1")
	h.wait("a has 5.1 audio", func(p *Player) bool { return sourceOf(p).SurroundURL == "url:yt:a/5.1" })
	c := r.call(h, 0, "url:yt:a/5.1")
	if got := h.stream.lastLoad(); got != "load url:yt:a 0s true" {
		t.Fatalf("a loaded %q before its 5.1 audio was ready", got)
	}
	h.p.Time(9*time.Second, 200*time.Second)
	c.answer <- nil
	h.wait("a in 5.1", func(*Player) bool { return h.stream.lastLoad() == "load url:yt:a/5.1 9s true" })
}

// TestSurroundReadyAfterCrossfade crossfades into a track whose 5.1 audio
// isn't ready in stereo, and plays its 5.1 audio, made ready during the
// fade, once the fade ended: the track fading out isn't cut off (#457).
func TestSurroundReadyAfterCrossfade(t *testing.T) {
	h, r := newReadyHarness(t, func(f *fakeServices) {
		f.settings.Surround = true
		f.settings.Crossfade = 1
		f.prepare = func(t media.Track) (Source, error) { return surroundOf(t), nil }
	})
	r.setReady("url:yt:a/5.1")
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	if got := h.stream.lastLoad(); got != "load url:yt:a/5.1 0s true" {
		t.Fatalf("a loaded %q", got)
	}
	h.p.Time(199*time.Second, 200*time.Second)
	h.wait("b crossfaded", func(*Player) bool { return h.stream.has("crossfade url:yt:b 1s") })
	loads := h.stream.count("load")
	r.call(h, 0, "url:yt:b/5.1").answer <- nil
	h.wait("b's 5.1 audio ready", func(p *Player) bool { return p.readyURL == "url:yt:b/5.1" })
	h.on(func(p *Player) {
		if p.crossfading && h.stream.count("load") != loads {
			t.Errorf("b's 5.1 audio cut the crossfade off: %q", h.stream.copy())
		}
	})
	h.wait("b in 5.1", func(*Player) bool { return h.stream.lastLoad() == "load url:yt:b/5.1 0s true" })
}
