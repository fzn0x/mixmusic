package player

import (
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// resumingServices are the fake services, which know where the user left
// episodes.
type resumingServices struct {
	*fakeServices
	mu sync.Mutex
	at map[string]time.Duration
}

func (r *resumingServices) ResumeAt(t media.Track) time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.at[t.ID]
}

// episodeTrack is a podcast's episode an hour long.
func episodeTrack(id string) media.Track {
	t := track(id, "Show")
	t.DurationMs = 3600000
	t.Episode = &media.Episode{Show: media.Ref{Name: "Show"}}
	return t
}

func newResumeHarness(t *testing.T, at map[string]time.Duration, setup ...func(*fakeServices)) *harness {
	t.Helper()
	return newHarnessWith(t, Saved{}, func(f *fakeServices) Services { return &resumingServices{fakeServices: f, at: at} }, setup...)
}

// TestResumeEpisode plays an episode from where the user left it, as it
// becomes current: chosen, or next in the queue. A song starts from its
// start, whatever the app says of it.
func TestResumeEpisode(t *testing.T) {
	h := newResumeHarness(t, map[string]time.Duration{"yt:e1": 59 * time.Second, "yt:e2": 2 * time.Minute, "yt:s": time.Minute})
	h.p.PlayTracks([]media.Track{episodeTrack("e1"), episodeTrack("e2"), track("s")}, 0)
	h.wait("e1 playing", playing("e1"))
	if got := h.stream.lastLoad(); got != "load url:yt:e1 59s true" {
		t.Errorf("loaded %q", got)
	}
	h.p.Next()
	h.wait("e2 playing", playing("e2"))
	if got := h.stream.lastLoad(); got != "load url:yt:e2 2m0s true" {
		t.Errorf("loaded %q", got)
	}
	h.p.Next()
	h.wait("s playing", playing("s"))
	if got := h.stream.lastLoad(); got != "load url:yt:s 0s true" {
		t.Errorf("loaded %q", got)
	}
}

// TestRestartEpisode plays an episode from its start when the user asks
// for it, as the episode is current: Previous, or seeking to its start.
func TestRestartEpisode(t *testing.T) {
	h := newResumeHarness(t, map[string]time.Duration{"yt:e1": time.Minute})
	h.p.PlayTracks([]media.Track{episodeTrack("e1")}, 0)
	h.wait("e1 playing", playing("e1"))
	h.p.Time(time.Minute+10*time.Second, time.Hour)
	h.p.Previous()
	h.wait("back at its start", func(*Player) bool { return h.stream.has("seek 0s") })
	if n := h.stream.count("load"); n != 1 {
		t.Errorf("loaded again: %q", h.stream.copy())
	}
}

// TestEpisodesNeitherCrossfadeNorPreload plays the next episode once one
// ends, without fading the two together, and without holding the next
// one's audio ahead.
func TestEpisodesNeitherCrossfadeNorPreload(t *testing.T) {
	h := newResumeHarness(t, nil, func(f *fakeServices) { f.settings.Crossfade = 5 })
	h.p.PlayTracks([]media.Track{episodeTrack("e1"), episodeTrack("e2"), track("s")}, 0)
	h.wait("e1 playing", playing("e1"))
	h.p.Time(time.Hour-3*time.Second, time.Hour)
	h.wait("e2 prepared", func(*Player) bool {
		var prepared []string
		h.svc.with(func(f *fakeServices) { prepared = slices.Clone(f.prepared) })
		return slices.Contains(prepared, "yt:e2")
	})
	h.on(func(*Player) {})
	time.Sleep(20 * time.Millisecond)
	h.on(func(*Player) {})
	if n := h.stream.count("crossfade") + h.stream.count("preload"); n != 0 {
		t.Errorf("into an episode: %q", h.stream.copy())
	}
	h.p.Ended()
	h.wait("e2 playing", playing("e2"))
	h.p.Time(time.Hour-3*time.Second, time.Hour)
	h.on(func(*Player) {})
	time.Sleep(20 * time.Millisecond)
	h.on(func(*Player) {})
	if n := h.stream.count("crossfade"); n != 0 {
		t.Errorf("out of an episode: %q", h.stream.copy())
	}
}

// TestNoRadioAfterEpisode stops at the end of a queue that ends on an
// episode, with autoplay on: its radio would be of songs.
func TestNoRadioAfterEpisode(t *testing.T) {
	h := newResumeHarness(t, nil, func(f *fakeServices) {
		f.settings.Autoplay = true
		f.radio = []media.Track{track("r1"), track("r2")}
	})
	h.p.PlayTracks([]media.Track{episodeTrack("e1")}, 0)
	h.wait("e1 playing", playing("e1"))
	h.p.Ended()
	h.wait("stopped at the end", func(p *Player) bool { return p.atEnd && !p.s.Playing })
	if n := len(h.p.State().Queue); n != 1 {
		t.Errorf("the radio added %d tracks", n-1)
	}
}

// TestSpotifyEpisodeResumesOnDevice has a Spotify device play an episode
// from where the user left it, and never hands it an episode ahead, which
// it would play from its start: the player starts that one itself.
func TestSpotifyEpisodeResumesOnDevice(t *testing.T) {
	connectTick, connectAskEvery, connectSettle = time.Hour, time.Hour, 0
	t.Cleanup(func() { connectTick, connectAskEvery, connectSettle = time.Second, 3*time.Second, 5*time.Second })
	h := newResumeHarness(t, map[string]time.Duration{"spotify:e1": 10 * time.Minute, "spotify:e2": time.Minute},
		func(f *fakeServices) { f.prepare = here })
	own := &aheadDevice{}
	h.p.SpotifyHere(own)
	e1, e2 := spotifyTrack("e1"), spotifyTrack("e2")
	e1.Episode, e2.Episode = &media.Episode{}, &media.Episode{}
	h.p.PlayTracks([]media.Track{e1, e2}, 0)
	h.wait("e1 playing", func(p *Player) bool { return current(p.s) == "spotify:e1" && p.connect.active() && p.s.Playing })
	if !own.has("play spotify:e1 10m0s") {
		t.Errorf("the device was told %v", own.calls)
	}
}
