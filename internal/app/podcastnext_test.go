package app

import (
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
)

func pageEpisode(id string, played bool) media.Track {
	return media.Track{ID: "ytpod:" + id, Title: id, Episode: &media.Episode{Played: played}}
}

// TestNextNewerIsTheNearestUnheard shows the episode after one taken from
// a page that lists them newest first: the nearest newer one the user
// hasn't heard (#589).
func TestNextNewerIsTheNearestUnheard(t *testing.T) {
	// Newest first.
	page := []media.Track{
		pageEpisode("ep5", false),
		pageEpisode("ep4", true),
		pageEpisode("ep3", false),
		pageEpisode("ep2", false),
		pageEpisode("ep1", false),
	}
	heard := func(t media.Track) bool { return t.Episode.Played }
	got, ok := nextNewer(page, "ytpod:ep2", heard)
	if !ok || got.Title != "ep3" {
		t.Fatalf("the next episode is %q, %v, want ep3", got.Title, ok)
	}
	// A heard episode newer still is passed over.
	pg := append([]media.Track(nil), page...)
	pg[2].Episode.Played = true
	got, ok = nextNewer(pg, "ytpod:ep2", heard)
	if !ok || got.Title != "ep5" {
		t.Fatalf("the next episode is %q, %v, want ep5", got.Title, ok)
	}
}

// TestNextNewerAtTheNewest shows the newest episode having nothing after
// it, and a page that doesn't hold the episode having no answer either.
func TestNextNewerAtTheNewest(t *testing.T) {
	page := []media.Track{pageEpisode("ep2", false), pageEpisode("ep1", false)}
	heard := func(t media.Track) bool { return t.Episode.Played }
	if got, ok := nextNewer(page, "ytpod:ep2", heard); ok {
		t.Errorf("the newest episode answered with %q", got.Title)
	}
	if got, ok := nextNewer(page, "ytpod:ep9", heard); ok {
		t.Errorf("an episode the page doesn't hold answered with %q", got.Title)
	}
}
