package youtube

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// TestParseEpisodeDetails reads the details of an episode, made up: its
// show, views, day and length, and its description line by line, with its
// links, unwrapped from YouTube's redirect, its times, and links to other
// videos. A link that isn't to a web page, and a hashtag's, are text.
func TestParseEpisodeDetails(t *testing.T) {
	d, err := parseEpisodeDetails(fixture(t, "episode-details"), "Ep1Video000")
	if err != nil {
		t.Fatal(err)
	}
	if d.Show != (media.Ref{ID: "yt:MPSPPLshow", Name: "Night Science"}) || d.Plays != 12000 || d.Released != "2025-10-26" || d.DurationMs != 3720000 {
		t.Errorf("details = %+v", d)
	}
	want := [][]media.TextRun{
		{{Text: "Why owls stay up, and what the barn hears at night."}},
		{{Text: "Notes: "}, {Text: "https://example.com/night-sci...", Link: "https://example.com/night-science/owls"}},
		{{Text: "Our site: "}, {Text: "example.org/barn", Link: "https://example.org/barn"}},
		{{Text: "Not a page: "}, {Text: "run this"}},
		nil,
		{{Text: "0:00", Seek: true}, {Text: " - Dusk"}},
		{{Text: "12:34", Seek: true, AtMs: 754000}, {Text: " - The barn"}},
		{{Text: "2:05:00", Seek: true, AtMs: 7500000}, {Text: " - Long after the end"}},
		{{Text: "Part one: "}, {Text: "Owls, part one", Link: "https://music.youtube.com/watch?v=otherVideo1"}, {Text: " "}, {Text: "#owls"}},
	}
	if !reflect.DeepEqual(d.Description, want) {
		t.Errorf("description =\n%+v\nwant\n%+v", d.Description, want)
	}
	if _, err := parseEpisodeDetails(node{map[string]any{}}, "Ep1Video000"); !errors.Is(err, ErrNotFound) {
		t.Errorf("a page without details: %v", err)
	}
}

// TestEpisodeDetails asks for the page of the episode's Details tab.
func TestEpisodeDetails(t *testing.T) {
	var asked string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct{ BrowseID string }
		json.NewDecoder(r.Body).Decode(&body)
		asked = r.URL.Path + " " + body.BrowseID
		json.NewEncoder(w).Encode(fixture(t, "episode-details").v)
	}))
	defer srv.Close()
	c := New()
	c.BaseURL = srv.URL
	d, err := c.EpisodeDetails(context.Background(), "Ep1Video000")
	if err != nil || d.Show.Name != "Night Science" || len(d.Description) == 0 {
		t.Fatalf("EpisodeDetails = %+v, %v", d, err)
	}
	if asked != "/youtubei/v1/browse MPPEEp1Video000" {
		t.Errorf("asked %q", asked)
	}
}

// TestLiveEpisodeDetails reads the details of an episode a search finds
// on YouTube Music itself, with MIXMUSIC_LIVE=1.
func TestLiveEpisodeDetails(t *testing.T) {
	if os.Getenv("MIXMUSIC_LIVE") == "" {
		t.Skip("set MIXMUSIC_LIVE=1 to test against YouTube Music")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	c := New()
	res, err := c.SearchPodcasts(ctx, "lex fridman podcast")
	if err != nil || len(res.Episodes) == 0 {
		t.Fatalf("search found %d episodes, %v", len(res.Episodes), err)
	}
	_, id, _ := media.ParseID(res.Episodes[0].ID)
	d, err := c.EpisodeDetails(ctx, id)
	if err != nil || d.Show.ID == "" || d.Released == "" || d.DurationMs == 0 || len(d.Description) == 0 {
		t.Fatalf("details of %s = %+v, %v", id, d, err)
	}
	links, times := 0, 0
	for _, line := range d.Description {
		for _, r := range line {
			if r.Link != "" {
				links++
			}
			if r.Seek {
				times++
			}
		}
	}
	t.Logf("%s: %s, %d plays, %s, %d ms, %d lines, %d links, %d times, %d chapters",
		id, d.Show.Name, d.Plays, d.Released, d.DurationMs, len(d.Description), links, times, len(media.ChaptersOf(d.Description)))
}
