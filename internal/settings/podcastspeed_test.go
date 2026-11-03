package settings

import (
	"encoding/json"
	"testing"
)

// TestPodcastSpeed plays episodes at their own speed in a new install and
// in settings from before the podcasts' speed, keeps it apart from the
// songs', and cleans it as the songs' is cleaned.
func TestPodcastSpeed(t *testing.T) {
	if d := Default(); d.PodcastSpeed != 1 {
		t.Errorf("a new install plays episodes at %v", d.PodcastSpeed)
	}
	for _, c := range []struct {
		file          string
		song, episode float64
	}{
		{`{}`, 1, 1},
		{`{"songSpeed":1.5}`, 1.5, 1},
		{`{"podcastSpeed":1.75}`, 1, 1.75},
		{`{"songSpeed":0.75,"podcastSpeed":2}`, 0.75, 2},
		{`{"podcastSpeed":1.234}`, 1, 1.25},
		{`{"podcastSpeed":0}`, 1, 1},
		{`{"podcastSpeed":-1}`, 1, 1},
		{`{"podcastSpeed":0.1}`, 1, 0.5},
		{`{"podcastSpeed":9}`, 1, 3},
	} {
		s := Default()
		if err := json.Unmarshal([]byte(c.file), &s); err != nil {
			t.Fatal(err)
		}
		s = s.Clean()
		if s.SongSpeed != c.song || s.PodcastSpeed != c.episode {
			t.Errorf("%s: songs play at %v, episodes at %v; want %v and %v", c.file, s.SongSpeed, s.PodcastSpeed, c.song, c.episode)
		}
	}
	// It writes as podcastSpeed, and reads back as it was.
	s := Default()
	s.PodcastSpeed = 1.1
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(data, &keys); err != nil {
		t.Fatal(err)
	}
	if got := string(keys["podcastSpeed"]); got != "1.1" {
		t.Errorf("podcastSpeed writes as %s", got)
	}
	back := Default()
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if back = back.Clean(); back.PodcastSpeed != 1.1 || back.SongSpeed != 1 {
		t.Errorf("read back, episodes play at %v and songs at %v", back.PodcastSpeed, back.SongSpeed)
	}
}
