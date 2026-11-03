package library

import (
	"path/filepath"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
)

// TestFollowPodcasts keeps the podcasts followed, without their episodes,
// the last followed first, for the next start.
func TestFollowPodcasts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "library.json")
	l, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Podcasts()) != 0 || l.Podcasts() == nil {
		t.Errorf("a new library follows %v", l.Podcasts())
	}
	a := media.Podcast{ID: "yt:MPSPa", Title: "A", Episodes: []media.Track{{ID: "yt:e"}}, More: "next"}
	b := media.Podcast{ID: "sp:b", Title: "B"}
	for _, p := range []media.Podcast{a, b, a} {
		if err := l.SetFollowed(p, true); err != nil {
			t.Fatal(err)
		}
	}
	got := l.Podcasts()
	if len(got) != 2 || got[0].ID != a.ID || got[1].ID != b.ID || len(got[0].Episodes) != 0 || got[0].More != "" {
		t.Errorf("Podcasts() = %+v", got)
	}
	if err := l.SetFollowed(b, false); err != nil {
		t.Fatal(err)
	}
	again, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if !again.Follows(a.ID) || again.Follows(b.ID) || len(again.Podcasts()) != 1 {
		t.Errorf("after a restart: %+v", again.Podcasts())
	}
}

// TestStatsLeaveOutEpisodes counts music in the stats: an episode of hours
// stays in the history, but not in the plays, the minutes or the tops.
func TestStatsLeaveOutEpisodes(t *testing.T) {
	l := open(t)
	song := tr("yt:a", media.SourceYouTube, "A")
	ep := episode("yt:e")
	l.RecordPlay(song)
	l.RecordPlay(ep)
	s := l.Stats(0)
	if s.Plays != 1 || s.Minutes != 3 || len(s.TopTracks) != 1 || len(s.TopArtists) != 1 {
		t.Errorf("stats = %+v", s)
	}
	if h := l.History(0); len(h) != 2 || h[0].Track.ID != ep.ID {
		t.Errorf("history = %+v", h)
	}
}
