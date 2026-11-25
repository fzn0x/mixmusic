package player

import (
	"errors"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
)

// episode is a track of a show's episode.
func episode(id string) media.Track {
	t := track(id)
	t.Episode = &media.Episode{Show: media.Ref{ID: "ytpod:show", Name: "A Show"}}
	return t
}

// TestNextEpisodeFollowsAnEpisode plays a queue that ends on an episode,
// with autoplay on: the show's next episode is added and plays (#589).
func TestNextEpisodeFollowsAnEpisode(t *testing.T) {
	next := episode("ep2")
	h := newResumeHarness(t, nil, func(f *fakeServices) {
		f.settings.Autoplay = true
		f.nextEpisode = next
	})
	first := episode("ep1")
	h.p.PlayTracks([]media.Track{first}, 0)
	h.wait("the episode plays", func(p *Player) bool {
		cur, ok := p.s.Current()
		return ok && cur.Track.ID == first.ID && p.s.Playing && !p.s.Buffering
	})
	h.p.Ended()
	h.wait("the next episode plays", func(p *Player) bool {
		cur, ok := p.s.Current()
		return ok && cur.Track.ID == next.ID && p.s.Playing
	})
	if n := len(h.p.State().Queue); n != 2 {
		t.Errorf("the queue holds %d tracks, want 2", n)
	}
}

// TestNextEpisodeStopsAtTheNewest shows a show with no newer episode
// stopping the queue, as today, with nothing said.
func TestNextEpisodeStopsAtTheNewest(t *testing.T) {
	h := newResumeHarness(t, nil, func(f *fakeServices) {
		f.settings.Autoplay = true
		f.nextEpisodeNone = true
	})
	last := episode("ep9")
	h.p.PlayTracks([]media.Track{last}, 0)
	h.wait("the episode plays", func(p *Player) bool {
		cur, ok := p.s.Current()
		return ok && cur.Track.ID == last.ID && p.s.Playing
	})
	h.p.Ended()
	h.wait("stopped at the end", func(p *Player) bool { return p.atEnd && !p.s.Playing })
	if n := len(h.p.State().Queue); n != 1 {
		t.Errorf("the queue holds %d tracks, want 1", n)
	}
}

// TestNextEpisodeUnreadStopsWithANote shows a show whose episodes can't be
// read stopping the queue, saying why.
func TestNextEpisodeUnreadStopsWithANote(t *testing.T) {
	h := newResumeHarness(t, nil, func(f *fakeServices) {
		f.settings.Autoplay = true
		f.nextEpisodeErr = errors.New("no network")
	})
	last := episode("ep9")
	h.p.PlayTracks([]media.Track{last}, 0)
	h.wait("the episode plays", func(p *Player) bool {
		cur, ok := p.s.Current()
		return ok && cur.Track.ID == last.ID && p.s.Playing
	})
	h.p.Ended()
	h.wait("stopped at the end", func(p *Player) bool { return p.atEnd && !p.s.Playing })
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.notes) == 0 {
		t.Fatal("nothing said why it stopped")
	}
}
