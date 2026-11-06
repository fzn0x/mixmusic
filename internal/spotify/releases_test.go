package spotify

import (
	"context"
	"slices"
	"testing"
)

// TestArtistReleases asks for an artist's newest albums, then singles, ten
// of each, and keeps each release's date as far as its precision goes.
func TestArtistReleases(t *testing.T) {
	c, api := newFake(t, map[string]string{
		"GET /artists/a1/albums?include_groups=album&limit=10": `{"items": [
			{"id": "al2", "name": "Second Light", "album_type": "album", "release_date": "2026-10-30", "release_date_precision": "day", "total_tracks": 11,
				"artists": [{"id": "a1", "name": "Harbour Lights"}], "images": [{"url": "https://i/al2", "width": 640, "height": 640}]},
			null,
			{"id": "al1", "name": "First Light", "album_type": "album", "release_date": "2019-01-01", "release_date_precision": "year",
				"artists": [{"id": "a1", "name": "Harbour Lights"}], "images": []}]}`,
		"GET /artists/a1/albums?include_groups=single&limit=10": `{"items": [
			{"id": "s1", "name": "Pier", "album_type": "single", "release_date": "2026-09", "release_date_precision": "month",
				"artists": [{"id": "a1", "name": "Harbour Lights"}], "images": []}]}`,
	})
	got, err := c.ArtistReleases(context.Background(), "a1")
	if err != nil {
		t.Fatal(err)
	}
	var ids, days []string
	for _, r := range got {
		ids, days = append(ids, r.Album.ID), append(days, r.Released)
	}
	if !slices.Equal(ids, []string{"sp:al2", "sp:al1", "sp:s1"}) || !slices.Equal(days, []string{"2026-10-30", "2019", "2026-09"}) {
		t.Errorf("releases %q, released %q", ids, days)
	}
	if al := got[0].Album; al.Title != "Second Light" || al.Kind != "album" || al.Year != "2026" || len(al.Artwork) != 1 ||
		len(al.Artists) != 1 || al.Artists[0].ID != "sp:a1" || got[2].Album.Kind != "single" {
		t.Errorf("the releases read %+v", got)
	}
	if len(api.requests) != 2 {
		t.Errorf("asked %q", api.requests)
	}
}

// TestArtistReleasesFail fails when a page of them does.
func TestArtistReleasesFail(t *testing.T) {
	c, _ := newFake(t, map[string]string{
		"GET /artists/a1/albums?include_groups=album&limit=10": `{"items": []}`,
	})
	if got, err := c.ArtistReleases(context.Background(), "a1"); err == nil {
		t.Errorf("no singles answered, and it returned %+v", got)
	}
}
