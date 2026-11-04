package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

// detailsPage is YouTube Music's page of an episode's details, made up:
// two chapters, and a link.
const detailsPage = `{"contents": {"sectionListRenderer": {"contents": [{"musicDescriptionShelfRenderer": {
	"strapline": {"runs": [{"text": "3K views"}, {"text": " • "}, {"text": "Oct 26, 2025"}, {"text": " • "}, {"text": "34 min"}]},
	"subheader": {"runs": [{"text": "Night Science", "navigationEndpoint": {"browseEndpoint": {"browseId": "MPSPPLshow",
		"browseEndpointContextSupportedConfigs": {"browseEndpointContextMusicConfig": {"pageType": "MUSIC_PAGE_TYPE_PODCAST_SHOW_DETAIL_PAGE"}}}}}]},
	"description": {"runs": [{"text": "Why we dream.\n"},
		{"text": "0:00", "navigationEndpoint": {"watchEndpoint": {"videoId": "ep2", "startTimeSeconds": 0}}}, {"text": " Dusk\n"},
		{"text": "9:30", "navigationEndpoint": {"watchEndpoint": {"videoId": "ep2", "startTimeSeconds": 570}}}, {"text": " Dreams\n"},
		{"text": "example.com", "navigationEndpoint": {"urlEndpoint": {"url": "https://example.com/dreams"}}}]}}}]}}}`

var anEpisode = media.Track{ID: "yt:ep2", Source: media.SourceYouTube, Title: "Why we dream", Artists: []media.Ref{{Name: "Night Science"}},
	DurationMs: 2040000, Video: true, Episode: &media.Episode{Show: media.Ref{ID: "yt:MPSPPLshow", Name: "Night Science"}}}

// detailsCatalog is a catalog whose YouTube Music answers with answer,
// and counts what it is asked.
func detailsCatalog(t *testing.T, answer http.HandlerFunc) (*Catalog, *atomic.Int32) {
	t.Helper()
	var asks atomic.Int32
	yt := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asks.Add(1)
		answer(w, r)
	}))
	t.Cleanup(yt.Close)
	a := &App{youtube: youtube.New(), episodeDetails: newMemo[media.EpisodeDetails](4)}
	a.youtube.BaseURL = yt.URL
	return &Catalog{a: a}, &asks
}

// TestEpisodeDetailsKept reads an episode's details with their chapters,
// once: the second time, they are those kept.
func TestEpisodeDetailsKept(t *testing.T) {
	c, asks := detailsCatalog(t, status(http.StatusOK, detailsPage))
	for range 2 {
		d, err := c.EpisodeDetails(context.Background(), anEpisode)
		if err != nil {
			t.Fatal(err)
		}
		if d.Show.Name != "Night Science" || d.Plays != 3000 || d.DurationMs != 2040000 || len(d.Description) != 4 ||
			len(d.Chapters) != 2 || d.Chapters[1] != (media.Chapter{AtMs: 570000, Title: "Dreams", Line: 2}) {
			t.Errorf("details = %+v", d)
		}
	}
	if n := asks.Load(); n != 1 {
		t.Errorf("asked %d times", n)
	}
}

// TestEpisodeDetailsFail tells an episode without details, which asking
// again won't find, from a failure, which may pass, and is kept by
// neither; a song has no details.
func TestEpisodeDetailsFail(t *testing.T) {
	c, asks := detailsCatalog(t, status(http.StatusOK, `{"contents": {}}`))
	if _, err := c.EpisodeDetails(context.Background(), anEpisode); !errors.Is(err, ErrNoEpisodeDetails) || err.Error() != "No details found for this episode." {
		t.Errorf("an episode without details: %v", err)
	}
	c, _ = detailsCatalog(t, status(http.StatusNotFound, `{"error": {"code": 404, "message": "Requested entity was not found."}}`))
	if _, err := c.EpisodeDetails(context.Background(), anEpisode); !errors.Is(err, ErrNoEpisodeDetails) {
		t.Errorf("an episode YouTube Music doesn't know: %v", err)
	}
	c, asks = detailsCatalog(t, status(http.StatusServiceUnavailable, ""))
	for range 2 {
		if _, err := c.EpisodeDetails(context.Background(), anEpisode); err == nil || errors.Is(err, ErrNoEpisodeDetails) {
			t.Errorf("a failure: %v", err)
		}
	}
	if n := asks.Load(); n != 2 {
		t.Errorf("a failure was kept: asked %d times", n)
	}
	song := anEpisode
	song.Episode = nil
	if _, err := c.EpisodeDetails(context.Background(), song); !errors.Is(err, ErrNoEpisodeDetails) || asks.Load() != 2 {
		t.Errorf("a song: %v", err)
	}
}
