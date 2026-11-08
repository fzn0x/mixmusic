package youtube

import (
	"slices"
	"testing"
)

// TestShelfKeys names each shelf of the home feed by the page its header
// opens, which its title, in the user's language, isn't: a page of YouTube
// Music's own with the parameters that pick its part, an artist's channel
// without the parameters every link has, and nothing for a shelf that
// opens no page. The fixture is hand-written, as its _fixture key says.
func TestShelfKeys(t *testing.T) {
	shelves := parseShelves(fixture(t, "home-shelf-keys"))
	var titles, keys []string
	for _, sh := range shelves {
		titles = append(titles, sh.Title)
		keys = append(keys, sh.Key)
	}
	wantTitles := []string{"Listen again", "Long listening", "Quick picks", "M83", "New releases"}
	wantKeys := []string{
		"FEmusic_listen_again",
		"FEmusic_moods_and_genres_category/ggMPOg1uX1JOQWZFeDByc2Jm",
		"",
		"UCdmrbmWDzVHOtrWyFSLDzLg",
		"FEmusic_new_releases_albums",
	}
	if !slices.Equal(titles, wantTitles) {
		t.Fatalf("shelves %q, want %q", titles, wantTitles)
	}
	if !slices.Equal(keys, wantKeys) {
		t.Errorf("keys %q, want %q", keys, wantKeys)
	}
}

// TestRecordedShelfKeys reads the recorded home feed's shelves, whose
// headers open no page: they keep no key, and Home names them by their
// titles.
func TestRecordedShelfKeys(t *testing.T) {
	for _, sh := range parseShelves(fixture(t, "home")) {
		if sh.Key != "" {
			t.Errorf("%q has the key %q", sh.Title, sh.Key)
		}
	}
}
