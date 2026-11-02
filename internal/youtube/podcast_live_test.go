package youtube

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// TestLivePodcasts checks the parsers of podcasts against YouTube Music
// itself, with MIXMUSIC_LIVE=1: a search, a show's page and its next page.
func TestLivePodcasts(t *testing.T) {
	if os.Getenv("MIXMUSIC_LIVE") == "" {
		t.Skip("set MIXMUSIC_LIVE=1 to test against YouTube Music")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	c := New()
	res, err := c.SearchPodcasts(ctx, "huberman lab")
	if err != nil || len(res.Podcasts) == 0 || len(res.Episodes) == 0 {
		t.Fatalf("search found %d shows, %d episodes, %v", len(res.Podcasts), len(res.Episodes), err)
	}
	for _, e := range res.Episodes {
		if e.Episode.Released == "" {
			t.Errorf("episode %s tells no day", e.ID)
		}
	}
	_, id, _ := media.ParseID(res.Podcasts[0].ID)
	p, err := c.Podcast(ctx, id)
	if err != nil || len(p.Episodes) == 0 || p.More == "" {
		t.Fatalf("show %s: %d episodes, more %q, %v", id, len(p.Episodes), p.More, err)
	}
	for _, e := range p.Episodes {
		if e.DurationMs == 0 || e.Episode.Released == "" {
			t.Errorf("episode %s: %d ms, released %q", e.ID, e.DurationMs, e.Episode.Released)
		}
	}
	page, err := c.PodcastEpisodes(ctx, media.Ref{ID: p.ID, Name: p.Title}, p.More)
	if err != nil || len(page.Episodes) < 20 {
		t.Fatalf("the next page: %d episodes, %v", len(page.Episodes), err)
	}
	t.Logf("%s: %d episodes, then %d, the first %q of %s", p.Title, len(p.Episodes), len(page.Episodes), p.Episodes[0].Title, p.Episodes[0].Episode.Released)
}
