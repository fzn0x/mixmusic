package player

import (
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// follow has the fake engine play the track its other deck holds on the
// sample after the one that played, as the engine does once that one
// ended, and tell the player so. It returns the URL of the track.
func (s *fakeStream) follow() string {
	s.mu.Lock()
	url := s.ahead
	s.url, s.ahead = url, ""
	s.mu.Unlock()
	s.record("followed %s", url)
	s.p.Followed(url)
	return url
}

// armed waits for the engine's other deck to hold url, told last that it
// follows the track that plays as arm says: "true", "false", "true glide"
// and so on.
func (h *harness) armed(url, arm string) {
	h.t.Helper()
	h.wait("url "+url+" armed "+arm, func(*Player) bool {
		return h.stream.has("preload "+url) && h.stream.last("arm "+url+" ") == "arm "+url+" "+arm
	})
}

// followHarness plays queue from its first track, with Crossfade off, as
// far into it as the next track is preloaded.
func followHarness(t *testing.T, queue []media.Track, setup ...func(*fakeServices)) *harness {
	t.Helper()
	h := newHarness(t, Saved{}, setup...)
	h.p.PlayTracks(queue, 0)
	h.wait("the first track playing", playing(strings.TrimPrefix(queue[0].ID, "yt:")))
	h.p.Time(prefetchAfter+time.Second, 200*time.Second)
	return h
}

// TestFollowJoinsTheNextTrack arms the next track to follow the one that
// plays on its next sample, as it is preloaded, and takes in the engine
// following it as one track change: the next track is current at once,
// playing, with nothing loaded, paused or stopped, and its own next one is
// armed in turn.
func TestFollowJoinsTheNextTrack(t *testing.T) {
	h := followHarness(t, tracks("a", "b", "c"))
	h.armed("url:yt:b", "true")
	before := len(h.stream.copy())
	var changes atomic.Int32
	h.p.Subscribe(func(e Event) {
		if e == TrackChange {
			changes.Add(1)
		}
	})
	var reports int
	h.svc.with(func(f *fakeServices) { reports = len(f.reports) })
	h.p.Time(199*time.Second, 200*time.Second)
	h.stream.follow()
	h.wait("b playing", playing("b"))
	h.on(func(p *Player) {
		if p.s.Index != 1 || p.s.Position != 0 || p.s.Loading || p.s.Speed != 1 {
			t.Errorf("b is at %d, at %v, loading %v, at %v×", p.s.Index, p.s.Position, p.s.Loading, p.s.Speed)
		}
	})
	if after := h.stream.copy()[before:]; !slices.Equal(after, []string{"followed url:yt:b"}) {
		t.Errorf("the engine was asked again: %q", after)
	}
	if n := changes.Load(); n != 1 {
		t.Errorf("%d track changes", n)
	}
	h.svc.with(func(f *fakeServices) {
		for _, now := range f.reports[reports:] {
			if !now.Playing || now.Loading {
				t.Errorf("the app was told playing %v, loading %v, of %v", now.Playing, now.Loading, now.Track.ID)
			}
		}
	})
	h.p.Time(prefetchAfter+time.Second, 200*time.Second)
	h.armed("url:yt:c", "true")
}

// TestFollowDisarms tells the engine the next track follows no longer as
// the state changes so that it isn't the one to play next, or mustn't
// start so, and arms it again as it changes back.
func TestFollowDisarms(t *testing.T) {
	for _, c := range []struct {
		what         string
		change, back func(h *harness)
	}{
		{"repeat one", func(h *harness) { h.on(func(p *Player) { p.s.Repeat = RepeatOne }) },
			func(h *harness) { h.on(func(p *Player) { p.s.Repeat = RepeatOff }) }},
		{"a track put next", func(h *harness) { h.p.PlayNext([]media.Track{track("d")}) },
			func(h *harness) { h.p.RemoveAt(1) }},
		{"the next track moved", func(h *harness) { h.p.Move(1, 2) }, func(h *harness) { h.p.Move(2, 1) }},
		{"shuffle", func(h *harness) {
			h.on(func(p *Player) {
				// Shuffled until b isn't next.
				for next, _ := p.s.UpNext(); !p.s.Shuffle || next.Track.ID == "yt:b"; next, _ = p.s.UpNext() {
					p.setShuffle(false)
					p.setShuffle(true)
				}
			})
		}, func(h *harness) { h.p.SetShuffle(false) }},
		{"the next track disliked", func(h *harness) { h.p.Disliked("yt:b", true) },
			func(h *harness) { h.p.Disliked("yt:b", false) }},
		{"a crossfade under way", func(h *harness) { h.on(func(p *Player) { p.crossfading = true }) },
			func(h *harness) { h.on(func(p *Player) { p.crossfading = false }) }},
	} {
		t.Run(c.what, func(t *testing.T) {
			h := followHarness(t, tracks("a", "b", "c"), func(f *fakeServices) { f.settings.SkipDisliked = true })
			h.armed("url:yt:b", "true")
			c.change(h)
			h.armed("url:yt:b", "false")
			if c.back != nil {
				c.back(h)
				h.armed("url:yt:b", "true")
			}
		})
	}
}

// TestFollowedFallsBack takes in the engine following the next track it
// was told to follow no longer, as the state changed a moment before: the
// player moves on as at the end of the track, to the entry the queue goes
// on to.
func TestFollowedFallsBack(t *testing.T) {
	h := followHarness(t, tracks("a", "b", "c"))
	h.armed("url:yt:b", "true")
	h.on(func(p *Player) { p.s.Repeat = RepeatOne })
	h.armed("url:yt:b", "false")
	loads := h.stream.count("load")
	h.stream.follow()
	h.wait("a again", func(p *Player) bool {
		return h.stream.count("load") == loads+1 && h.stream.lastLoad() == "load url:yt:a 0s true" && playing("a")(p)
	})
}

// TestFollowNotOnACastDevice moves the streams to a cast device, which
// plays one at a time: the engine lets go of the track it played, which
// disarms the next one, and nothing is armed on the device.
func TestFollowNotOnACastDevice(t *testing.T) {
	h := followHarness(t, tracks("a", "b"))
	h.armed("url:yt:b", "true")
	cast := newCastStream(h)
	h.p.SetOutput(Output{Stream: cast})
	h.wait("a on the device", func(p *Player) bool { return p.s.Remote && cast.lastLoad() != "" })
	if calls := h.stream.copy(); calls[len(calls)-1] != "unload" {
		t.Errorf("the engine kept the track: %q", calls)
	}
	cast.p.Time(prefetchAfter+2*time.Second, 200*time.Second)
	h.wait("b preloaded for the device", func(*Player) bool { return cast.has("preload url:yt:b") })
	if cast.count("arm") != 0 || h.stream.count("arm url:yt:b true") != 1 {
		t.Errorf("armed: the device did %q, the engine %q", cast.copy(), h.stream.copy())
	}
}

// TestFollowedAfterALoad takes in a Followed that comes after the player
// loaded a track, as Next pressed as the track ended: the track loaded
// plays on, and the queue doesn't move on again.
func TestFollowedAfterALoad(t *testing.T) {
	h := followHarness(t, tracks("a", "b", "c"))
	h.armed("url:yt:b", "true")
	h.p.Next()
	h.wait("b playing", playing("b"))
	loads := h.stream.count("load")
	h.p.Followed("url:yt:b")
	h.on(func(*Player) {})
	if n := h.stream.count("load"); n != loads || !playing("b")(h.p) {
		h.on(func(p *Player) { t.Errorf("at %s, the stream did %q", current(p.s), h.stream.copy()) })
	}
}

// TestFollowAtItsSpeed arms the next track at its kind's speed, and again
// at a speed changed after, which it plays at once it followed.
func TestFollowAtItsSpeed(t *testing.T) {
	h := followHarness(t, tracks("a", "b"), atSpeedOf(1.25))
	h.armed("url:yt:b", "true speed 1.25")
	h.setSpeed(1.5)
	h.armed("url:yt:b", "true speed 1.5")
	h.stream.follow()
	h.wait("b playing", playing("b"))
	if s := h.p.State(); s.Speed != 1.5 {
		t.Errorf("b plays at %v×", s.Speed)
	}
}

// TestFollowGlidesInAnAlbum has the next song of the same album follow at
// the loudness of the one before, and a song of another album at its own.
func TestFollowGlidesInAnAlbum(t *testing.T) {
	h := followHarness(t, []media.Track{albumTrack("a", "yt:MPREb_1", 1, 0), albumTrack("b", "yt:MPREb_1", 2, 0)})
	h.armed("url:yt:b", "true glide")
	h = followHarness(t, []media.Track{albumTrack("a", "yt:MPREb_1", 1, 0), albumTrack("b", "yt:MPREb_2", 2, 0)})
	h.armed("url:yt:b", "true")
}

// TestFollowInAGaplessAlbum plays the next song of an album on the sample
// after the one before with Crossfade on, as Keep albums gapless has it
// play straight on: no crossfade, no load.
func TestFollowInAGaplessAlbum(t *testing.T) {
	h := gaplessHarness(t, []media.Track{albumTrack("a", "yt:MPREb_1", 1, 0), albumTrack("b", "yt:MPREb_1", 2, 0)})
	h.armed("url:yt:b", "true glide")
	if fades(h) {
		t.Fatal("the album's songs crossfade")
	}
	loads := h.stream.count("load")
	h.stream.follow()
	h.wait("b playing", playing("b"))
	if h.stream.count("load") != loads || h.stream.has("crossfade url:yt:b 5s") {
		t.Errorf("the stream did %q", h.stream.copy())
	}
}

// TestFollowNotIntoACrossfade disarms the next track as the crossfade into
// it starts, before the engine crossfades: a crossfade plays as before.
func TestFollowNotIntoACrossfade(t *testing.T) {
	h := gaplessHarness(t, tracks("a", "b"))
	h.armed("url:yt:b", "true")
	if !fades(h) {
		t.Fatal("no crossfade")
	}
	h.wait("b crossfading in", playing("b"))
	calls := h.stream.copy()
	disarmed, faded := slices.Index(calls, "arm url:yt:b false"), slices.Index(calls, "crossfade url:yt:b 5s")
	if disarmed < 0 || faded < disarmed {
		t.Errorf("the stream did %q", calls)
	}
}

// TestFollowArmedAgainIn51 arms the next track again once the switch to
// 5.1 surround let go of it: it is preloaded again, in 5.1.
func TestFollowArmedAgainIn51(t *testing.T) {
	h := followHarness(t, tracks("a", "b"), func(f *fakeServices) {
		f.prepare = func(t media.Track) (Source, error) { return surroundOf(t), nil }
	})
	h.armed("url:yt:b", "true")
	h.setSurround(true)
	h.wait("a in 5.1", func(*Player) bool { return strings.HasPrefix(h.stream.lastLoad(), "load url:yt:a/5.1 ") })
	h.p.Time(prefetchAfter+2*time.Second, 200*time.Second)
	h.armed("url:yt:b/5.1", "true")
}

// joiningStream is a stream that tells whether the next track may start
// on the sample after the one that plays, as a room's host's does.
type joiningStream struct {
	*fakeStream
	joins atomic.Bool
}

func (s *joiningStream) Joins() bool { return s.joins.Load() }

// TestFollowAsTheStreamLets arms the next track only while the stream lets
// it start so, as it tells at each of the player's tasks.
func TestFollowAsTheStreamLets(t *testing.T) {
	h := &harness{t: t, svc: newServices(), stream: &fakeStream{}, device: &fakeDevice{}}
	s := &joiningStream{fakeStream: h.stream}
	h.p = New(h.svc, s, h.device, Saved{})
	h.stream.p = h.p
	t.Cleanup(h.p.Close)
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	h.p.Time(prefetchAfter+time.Second, 200*time.Second)
	h.wait("b preloaded", func(*Player) bool { return h.stream.has("preload url:yt:b") })
	if h.stream.has("arm url:yt:b true") {
		t.Fatal("armed while the stream doesn't let it")
	}
	s.joins.Store(true)
	h.p.Time(prefetchAfter+2*time.Second, 200*time.Second)
	h.armed("url:yt:b", "true")
	s.joins.Store(false)
	h.p.Time(prefetchAfter+3*time.Second, 200*time.Second)
	h.armed("url:yt:b", "false")
}
