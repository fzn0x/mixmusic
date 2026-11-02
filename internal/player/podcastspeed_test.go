package player

import (
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
)

// atSpeedsOf has the songs of the fake services play at song, and the
// episodes at episode.
func atSpeedsOf(song, episode float64) func(*fakeServices) {
	return func(f *fakeServices) { f.settings.SongSpeed, f.settings.PodcastSpeed = song, episode }
}

// TestEpisodeAtPodcastSpeed plays a podcast's episode at the podcasts'
// speed, and a song at the songs', as each becomes current.
func TestEpisodeAtPodcastSpeed(t *testing.T) {
	h := newHarness(t, Saved{}, atSpeedsOf(1.5, 2))
	h.p.PlayTracks([]media.Track{track("a", "A"), episodeTrack("e1"), track("b", "B")}, 0)
	h.wait("a playing", playing("a"))
	if s, got := h.p.State(), h.stream.lastLoad(); s.Speed != 1.5 || got != "load url:yt:a 0s true speed 1.5" {
		t.Errorf("the song plays at %v, loaded %q", s.Speed, got)
	}
	h.p.Next()
	h.wait("e1 playing", playing("e1"))
	if s, got := h.p.State(), h.stream.lastLoad(); s.Speed != 2 || got != "load url:yt:e1 0s true speed 2" {
		t.Errorf("the episode plays at %v, loaded %q", s.Speed, got)
	}
	h.wait("the episode reported at 2×", func(*Player) bool { return h.lastReport().Speed == 2 })
	h.p.Next()
	h.wait("b playing", playing("b"))
	if s, got := h.p.State(), h.stream.lastLoad(); s.Speed != 1.5 || got != "load url:yt:b 0s true speed 1.5" {
		t.Errorf("the song after the episode plays at %v, loaded %q", s.Speed, got)
	}
}

// TestPodcastSpeedChangeApplies plays the episode that plays at a new
// speed of the podcasts at once, and leaves it be as the songs' speed
// changes; the song that plays takes only the songs' speed.
func TestPodcastSpeedChangeApplies(t *testing.T) {
	h := newHarness(t, Saved{})
	h.p.PlayTracks([]media.Track{episodeTrack("e1"), track("b", "B")}, 0)
	h.wait("e1 playing", playing("e1"))
	if got := h.stream.lastLoad(); got != "load url:yt:e1 0s true" {
		t.Errorf("loaded %q", got)
	}
	set := func(song, episode float64) {
		h.svc.with(atSpeedsOf(song, episode))
		h.p.SettingsChanged(h.svc.Settings())
	}
	set(1, 1.25)
	h.wait("e1 at 1.25×", func(p *Player) bool { return p.s.Speed == 1.25 && h.stream.has("set speed 1.25") })
	set(1.75, 1.25)
	h.on(func(*Player) {})
	if s := h.p.State(); s.Speed != 1.25 || h.stream.count("set speed") != 1 {
		t.Errorf("the songs' speed moved the episode to %v: %q", s.Speed, h.stream.copy())
	}
	h.p.Next()
	h.wait("b playing", playing("b"))
	if s, got := h.p.State(), h.stream.lastLoad(); s.Speed != 1.75 || got != "load url:yt:b 0s true speed 1.75" {
		t.Errorf("the song plays at %v, loaded %q", s.Speed, got)
	}
	set(1.75, 0.75)
	h.on(func(*Player) {})
	if s := h.p.State(); s.Speed != 1.75 || h.stream.count("set speed") != 1 {
		t.Errorf("the podcasts' speed moved the song to %v: %q", s.Speed, h.stream.copy())
	}
}
