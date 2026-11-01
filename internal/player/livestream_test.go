package player

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// liveTrack is a live video, as YouTube Music lists it: of no length.
func liveTrack(id string) media.Track {
	return media.Track{ID: "yt:" + id, Source: media.SourceYouTube, Title: "Radio " + id, Video: true}
}

// liveServices are the fake services, whose tracks of IDs that start with
// "live" play as live streams.
type liveServices struct{ *fakeServices }

func (s liveServices) Prepare(ctx context.Context, t media.Track) (Source, error) {
	src, err := s.fakeServices.Prepare(ctx, t)
	src.Live = strings.HasPrefix(t.ID, "yt:live")
	return src, err
}

func (s liveServices) PrepareStream(ctx context.Context, t media.Track) (Source, error) {
	src, err := s.fakeServices.PrepareStream(ctx, t)
	src.Live = strings.HasPrefix(t.ID, "yt:live")
	return src, err
}

func newLiveHarness(t *testing.T, setup ...func(*fakeServices)) *harness {
	t.Helper()
	return newHarnessWith(t, Saved{}, func(f *fakeServices) Services { return liveServices{f} }, setup...)
}

// TestLiveTrack plays a live stream without a length and without seeking,
// counts it once it played a while, and tells the app it is live.
func TestLiveTrack(t *testing.T) {
	h := newLiveHarness(t)
	live := liveTrack("liveRadio01")
	live.DurationMs = 5000 // what a list may say of it
	h.p.PlayTracks([]media.Track{track("a", "A"), live, track("b", "B")}, 1)
	h.wait("the live stream playing", playing("liveRadio01"))
	if s := h.p.State(); !s.Live || s.Duration != 0 {
		t.Errorf("live %v, a duration of %v", s.Live, s.Duration)
	}
	h.p.Time(10*time.Second, 0)
	h.p.Seek(time.Minute)
	h.on(func(*Player) {})
	if s := h.p.State(); s.Position != 10*time.Second || h.stream.count("seek") != 0 {
		t.Errorf("at %v after a seek, the stream did %q", s.Position, h.stream.copy())
	}

	played := func() []string {
		var out []string
		h.svc.with(func(f *fakeServices) { out = slices.Clone(f.played) })
		return out
	}
	h.p.Time(20*time.Second, 0)
	h.on(func(*Player) {})
	if got := played(); len(got) != 0 {
		t.Errorf("counted after 20 seconds: %v", got)
	}
	h.p.Time(31*time.Second, 0)
	h.wait("the live stream counted", func(*Player) bool { return slices.Equal(played(), []string{"yt:liveRadio01"}) })
	// The reports are throttled: the last comes a moment later.
	h.wait("the live stream reported", func(*Player) bool {
		var last Now
		h.svc.with(func(f *fakeServices) { last = f.reports[len(f.reports)-1] })
		return last.Position == 31*time.Second
	})
	// Once its stream was found.
	h.svc.with(func(f *fakeServices) {
		for _, r := range f.reports {
			if r.Playing && (!r.Live || r.Duration != 0) {
				t.Errorf("reported %+v", r)
			}
		}
	})

	// Previous plays the track before: a live stream has no start.
	h.p.Previous()
	h.wait("a playing", playing("a"))
	if s := h.p.State(); s.Live || s.Duration != 200*time.Second {
		t.Errorf("a: live %v, a duration of %v", s.Live, s.Duration)
	}
}

// TestLiveTrackEnds moves on to the next track once a broadcast ended,
// and never fades out of a live stream, which has no end to fade at.
func TestLiveTrackEnds(t *testing.T) {
	h := newLiveHarness(t, func(f *fakeServices) { f.settings.Crossfade = 5 })
	h.p.PlayTracks([]media.Track{liveTrack("liveRadio01"), track("b", "B")}, 0)
	h.wait("the live stream playing", playing("liveRadio01"))
	h.p.Time(3*time.Hour, 0)
	h.on(func(*Player) {})
	if h.stream.count("crossfade") != 0 {
		t.Errorf("faded out of a live stream: %q", h.stream.copy())
	}
	h.p.Ended()
	h.wait("b playing", playing("b"))
}

// TestLiveTrackEndsTheQueue plays a live stream that ended the queue
// again when asked to play: it is found again, rather than played from a
// start it doesn't have.
func TestLiveTrackEndsTheQueue(t *testing.T) {
	h := newLiveHarness(t)
	h.p.PlayTracks([]media.Track{liveTrack("liveRadio01")}, 0)
	h.wait("the live stream playing", playing("liveRadio01"))
	h.p.Time(time.Minute, 0)
	h.p.Ended()
	h.wait("the end of the queue", func(p *Player) bool { return p.atEnd })
	h.stream.Pause() // as the engine does at the end
	h.wait("stopped", func(p *Player) bool { return !p.s.Playing })
	h.p.Play()
	h.wait("the live stream playing again", func(p *Player) bool { return h.stream.count("load") == 2 && playing("liveRadio01")(p) })
	if h.stream.count("seek") != 0 {
		t.Errorf("seeked a live stream: %q", h.stream.copy())
	}
}

// TestNextLiveTrackNotPreloaded finds the stream of a live track that
// comes next, but holds none of its audio ahead, which would be behind
// its live edge once it plays.
func TestNextLiveTrackNotPreloaded(t *testing.T) {
	h := newLiveHarness(t)
	h.p.PlayTracks([]media.Track{track("a", "A"), liveTrack("liveRadio01")}, 0)
	h.wait("a playing", playing("a"))
	h.p.Time(11*time.Second, 200*time.Second)
	h.wait("the live stream prepared", func(*Player) bool {
		var prepared []string
		h.svc.with(func(f *fakeServices) { prepared = slices.Clone(f.prepared) })
		n := 0
		for _, id := range prepared {
			if id == "yt:liveRadio01" {
				n++
			}
		}
		return n == 2
	})
	h.on(func(*Player) {})
	if n := h.stream.count("preload"); n != 0 {
		t.Errorf("%d preloads: %q", n, h.stream.copy())
	}
	h.p.Next()
	h.wait("the live stream playing", playing("liveRadio01"))
	if got := h.stream.lastLoad(); got != "load url:yt:liveRadio01 0s true" {
		t.Errorf("loaded %q", got)
	}
}

// TestLiveTrackTriedAgain plays a live stream that broke off again, as a
// stream that played, live all the while.
func TestLiveTrackTriedAgain(t *testing.T) {
	h := newLiveHarness(t)
	h.p.PlayTracks([]media.Track{liveTrack("liveRadio01")}, 0)
	h.wait("the stream playing", playing("liveRadio01"))
	h.p.Time(time.Minute, 0)
	h.p.Error("the network failed while loading the audio")
	h.wait("tried again", func(*Player) bool { return strings.HasPrefix(h.stream.lastLoad(), "load url:yt:liveRadio01?retry=") })
	h.wait("live again", func(p *Player) bool { return p.s.Live && p.s.Playing })
	if s := h.p.State(); !strings.HasSuffix(h.stream.lastLoad(), " 1m0s true") || s.Duration != 0 {
		t.Errorf("loaded %q, a duration of %v", h.stream.lastLoad(), s.Duration)
	}
}
