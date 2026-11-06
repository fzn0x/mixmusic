package spotify

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// albumsJSON is a page of n of an artist's albums and singles from first,
// each other one a single, of total.
func albumsJSON(first, n, total int) string {
	var items []string
	for i := first; i < first+n; i++ {
		kind := "album"
		if i%2 == 1 {
			kind = "single"
		}
		items = append(items, fmt.Sprintf(`{"id": "al%d", "name": "Record %d", "album_type": %q, "release_date": "2020", "artists": [{"id": "a1", "name": "Daft Punk"}], "images": []}`, i, i, kind))
	}
	return fmt.Sprintf(`{"items": [%s], "total": %d}`, strings.Join(items, ", "), total)
}

func TestArtistReadsAlbumsTenAtATime(t *testing.T) {
	const albums = "GET /artists/a1/albums?include_groups=album%2Csingle&limit=10&offset="
	c, api := newFake(t, map[string]string{
		"GET /artists/a1": `{"id": "a1", "name": "Daft Punk", "images": []}`,
		albums + "0":      albumsJSON(0, 10, 23),
		albums + "10":     albumsJSON(10, 10, 23),
		albums + "20":     albumsJSON(20, 3, 23),
		"GET /search":     `{"tracks": {"items": []}}`,
	})
	a, err := c.Artist(context.Background(), "a1")
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Albums) != 12 || len(a.Singles) != 11 {
		t.Fatalf("%d albums and %d singles, want 12 and 11 (requests %s)", len(a.Albums), len(a.Singles), api.requests)
	}
	for i, al := range a.Albums {
		if want := fmt.Sprintf("sp:al%d", 2*i); al.ID != want {
			t.Errorf("album %d is %s, want %s: the pages are out of order", i, al.ID, want)
		}
	}
	for _, r := range api.requests {
		if strings.Contains(r, "/albums") && !strings.Contains(r, "limit=10&") {
			t.Errorf("asked %s, beyond the 10 a page of an artist's albums holds", r)
		}
	}
}

func TestArtistReadsAtMostFivePagesOfAlbums(t *testing.T) {
	responses := map[string]string{
		"GET /artists/a1": `{"id": "a1", "name": "Daft Punk", "images": []}`,
		"GET /search":     `{"tracks": {"items": []}}`,
	}
	for offset := 0; offset < 100; offset += 10 {
		responses[fmt.Sprintf("GET /artists/a1/albums?include_groups=album%%2Csingle&limit=10&offset=%d", offset)] = albumsJSON(offset, 10, 100)
	}
	c, api := newFake(t, responses)
	a, err := c.Artist(context.Background(), "a1")
	if err != nil {
		t.Fatal(err)
	}
	if n := len(a.Albums) + len(a.Singles); n != 50 {
		t.Errorf("%d albums and singles, want the first 50 (requests %s)", n, api.requests)
	}
}
