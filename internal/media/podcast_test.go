package media

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestEpisodeJSON keeps the tracks that are no episodes as they were
// written, and an episode's show and day through JSON, as queue.json and
// library.json keep them.
func TestEpisodeJSON(t *testing.T) {
	song, err := json.Marshal(Track{ID: "yt:a", Source: SourceYouTube, Title: "A"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(song), "episode") {
		t.Errorf("a song encodes %s", song)
	}
	ep := Track{ID: "sp:e", Source: SourceSpotify, Title: "E", Episode: &Episode{
		Show: Ref{ID: "sp:s", Name: "Show"}, Released: "2026-10-30", ResumeMs: 5000}}
	data, err := json.Marshal(ep)
	if err != nil {
		t.Fatal(err)
	}
	var back Track
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if !back.IsEpisode() || *back.Episode != *ep.Episode {
		t.Errorf("the episode came back as %+v from %s", back.Episode, data)
	}
	if (Track{}).IsEpisode() {
		t.Error("a song is an episode")
	}
}

// TestPodcastSummary leaves out what only a podcast's page has.
func TestPodcastSummary(t *testing.T) {
	p := Podcast{ID: "yt:MPSPx", Title: "Show", Episodes: []Track{{ID: "yt:e"}}, More: "next", Saved: true}
	s := p.Summary()
	if len(s.Episodes) != 0 || s.Episodes == nil || s.More != "" || s.Saved || s.ID != p.ID || s.Title != p.Title {
		t.Errorf("Summary() = %+v", s)
	}
	if len(p.Episodes) != 1 {
		t.Error("Summary changed the podcast")
	}
	data, _ := json.Marshal(EmptyPodcastResults())
	if string(data) != `{"podcasts":[],"episodes":[]}` {
		t.Errorf("empty results encode as %s", data)
	}
}
