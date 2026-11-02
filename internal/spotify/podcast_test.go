package spotify

import (
	"context"
	"strings"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
)

// The answers below are in the shape the Web API's reference gives them,
// trimmed to what MixMusic reads.

const showJSON = `{"id": "s1", "name": "The Daily Thing", "type": "show", "total_episodes": 120,
	"description": "News, daily.", "images": [{"url": "https://i/show-640", "width": 640, "height": 640}, {"url": "https://i/show-64", "width": 64, "height": 64}],
	"external_urls": {"spotify": "https://open.spotify.com/show/s1"}`

const episodeJSON = `{"id": "e1", "name": "Monday", "type": "episode", "duration_ms": 1830000, "explicit": false,
	"release_date": "2026-11-02", "release_date_precision": "day",
	"images": [{"url": "https://i/ep-640", "width": 640, "height": 640}],
	"external_urls": {"spotify": "https://open.spotify.com/episode/e1"},
	"resume_point": {"fully_played": false, "resume_position_ms": 600000}}`

// TestSearchPodcasts asks for shows and episodes at once, 10 of each at
// most, and converts them.
func TestSearchPodcasts(t *testing.T) {
	c, api := newFake(t, map[string]string{
		"GET /search": `{"shows": {"items": [` + showJSON + `}, null]}, "episodes": {"items": [` + episodeJSON + `, null]}}`,
	})
	res, err := c.SearchPodcasts(context.Background(), "daily")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(api.requests[0], "type=show%2Cepisode") || !strings.Contains(api.requests[0], "limit=10") {
		t.Errorf("request = %s", api.requests[0])
	}
	if len(res.Podcasts) != 1 || len(res.Episodes) != 1 {
		t.Fatalf("results = %+v", res)
	}
	p := res.Podcasts[0]
	if p.ID != "sp:s1" || p.Title != "The Daily Thing" || p.EpisodeCount != 120 || len(p.Authors) != 0 || p.Authors == nil ||
		p.Artwork[0].Width != 64 || p.URL != "https://open.spotify.com/show/s1" {
		t.Errorf("podcast = %+v", p)
	}
	e := res.Episodes[0]
	if e.ID != "sp:e1" || !e.IsEpisode() || e.DurationMs != 1830000 || e.Episode.Released != "2026-11-02" ||
		e.Episode.ResumeMs != 600000 || e.Episode.Played || len(e.Artists) != 0 {
		t.Errorf("episode = %+v %+v", e, e.Episode)
	}
}

// TestShow reads a show with the first page of its episodes, which name
// it, and reads the next pages by offset.
func TestShow(t *testing.T) {
	c, api := newFake(t, map[string]string{
		"GET /shows/s1": showJSON + `, "publisher": "Someone", "episodes": {"items": [` + episodeJSON + `], "next": "{{base}}/shows/s1/episodes?offset=1&limit=50", "total": 2}}`,
		"GET /shows/s1/episodes": `{"items": [{"id": "e2", "name": "Sunday", "type": "episode", "duration_ms": 60000, "release_date": "2026-11", "release_date_precision": "month",
			"resume_point": {"fully_played": true, "resume_position_ms": 0}}], "next": null, "total": 2}`,
	})
	p, err := c.Show(context.Background(), "s1")
	if err != nil {
		t.Fatal(err)
	}
	if p.Title != "The Daily Thing" || len(p.Authors) != 1 || p.Authors[0].Name != "Someone" || p.More != "1" || len(p.Episodes) != 1 {
		t.Fatalf("show = %+v", p)
	}
	if e := p.Episodes[0]; e.Episode.Show != (media.Ref{ID: "sp:s1", Name: "The Daily Thing"}) || e.ArtistNames() != "The Daily Thing" {
		t.Errorf("episode = %+v %+v", e, e.Episode)
	}
	page, err := c.ShowEpisodes(context.Background(), media.Ref{ID: p.ID, Name: p.Title}, p.More)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Episodes) != 1 || page.More != "" || !page.Episodes[0].Episode.Played || page.Episodes[0].Episode.Released != "2026-11" ||
		page.Episodes[0].Episode.Show.ID != "sp:s1" {
		t.Errorf("page = %+v", page)
	}
	if got := api.requests[1]; got != "GET /shows/s1/episodes?limit=50&offset=1" {
		t.Errorf("request = %s", got)
	}
}

// TestSavedShows lists the shows of the user's library.
func TestSavedShows(t *testing.T) {
	c, _ := newFake(t, map[string]string{
		"GET /me/shows": `{"items": [{"added_at": "2026-10-01T00:00:00Z", "show": ` + showJSON + `}}, {"show": null}], "next": null}`,
	})
	shows, err := c.SavedShows(context.Background())
	if err != nil || len(shows) != 1 || shows[0].ID != "sp:s1" || len(shows[0].Episodes) != 0 {
		t.Fatalf("SavedShows = %+v, %v", shows, err)
	}
}

// TestStateOfEpisode follows a device that plays an episode, which Spotify
// tells only to an app that asks for episodes.
func TestStateOfEpisode(t *testing.T) {
	c, api := newFake(t, map[string]string{
		"GET /me/player": `{"device": {"id": "d1", "name": "Desk", "type": "Computer", "is_active": true}, "is_playing": true, "progress_ms": 1000,
			"currently_playing_type": "episode", "item": ` + strings.TrimSuffix(episodeJSON, "}") + `, "show": {"id": "s1", "name": "The Daily Thing"}}}`,
	})
	st, err := c.State(context.Background())
	if err != nil || st == nil || st.Track == nil {
		t.Fatalf("State = %+v, %v", st, err)
	}
	if st.Track.ID != "sp:e1" || !st.Track.IsEpisode() || st.Track.Episode.Show.Name != "The Daily Thing" {
		t.Errorf("track = %+v", st.Track)
	}
	if !strings.Contains(api.requests[0], "additional_types=track%2Cepisode") {
		t.Errorf("request = %s", api.requests[0])
	}
}

func TestTrackURI(t *testing.T) {
	if got := TrackURI(media.Track{ID: "sp:t1"}); got != "spotify:track:t1" {
		t.Errorf("a track: %s", got)
	}
	if got := TrackURI(media.Track{ID: "sp:e1", Episode: &media.Episode{}}); got != "spotify:episode:e1" {
		t.Errorf("an episode: %s", got)
	}
}
