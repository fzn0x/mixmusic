package youtube

import (
	"slices"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
)

func TestParseAlbum(t *testing.T) {
	a, err := parseAlbum(fixture(t, "album"), "MPREb_I0lA998lMLT")
	if err != nil {
		t.Fatal(err)
	}
	if a.Title != "Q-ENA II" || a.ID != "yt:MPREb_I0lA998lMLT" {
		t.Errorf("album = %q %s", a.Title, a.ID)
	}
	if a.Kind != media.AlbumKindAlbum {
		t.Errorf("kind = %q", a.Kind)
	}
	if len(a.Artists) != 1 || a.Artists[0].Name != "DJT" {
		t.Errorf("artists = %+v", a.Artists)
	}
	if a.TrackCount != 8 {
		t.Errorf("track count = %d", a.TrackCount)
	}
	if len(a.Tracks) == 0 {
		t.Fatal("no tracks")
	}
	first := a.Tracks[0]
	if first.Title != "GG" || first.DurationMs != 148000 {
		t.Errorf("first track = %q %d", first.Title, first.DurationMs)
	}
	if first.Album == nil || first.Album.ID != a.ID || len(first.Artists) == 0 || len(first.Artwork) == 0 {
		t.Errorf("track not completed from the album: %+v", first)
	}
}

// TestAlbumTrackNumbers numbers an album's tracks by their rows, from 1:
// a row MixMusic leaves out keeps its number.
func TestAlbumTrackNumbers(t *testing.T) {
	n := fixture(t, "album")
	a, err := parseAlbum(n, "MPREb_I0lA998lMLT")
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Tracks) != 6 {
		t.Fatalf("%d tracks", len(a.Tracks))
	}
	for i, tr := range a.Tracks {
		if tr.TrackNumber != i+1 || tr.DiscNumber != 0 {
			t.Errorf("track %d is number %d of disc %d", i, tr.TrackNumber, tr.DiscNumber)
		}
	}
	_, shelf := secondaryShelf(n)
	delete(shelf.get("contents", 1, "musicResponsiveListItemRenderer").v.(map[string]any), "flexColumns")
	a, err = parseAlbum(n, "MPREb_I0lA998lMLT")
	if err != nil {
		t.Fatal(err)
	}
	var got []int
	for _, tr := range a.Tracks {
		got = append(got, tr.TrackNumber)
	}
	if want := []int{1, 3, 4, 5, 6}; !slices.Equal(got, want) {
		t.Errorf("numbers %v without the second row, not %v", got, want)
	}
}
