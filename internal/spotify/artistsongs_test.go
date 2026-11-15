package spotify

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// albumWith is an album of the artist with the songs given as {id,
// title, seconds}.
func albumWith(id string, tracks ...[3]string) string {
	var items []string
	for _, t := range tracks {
		items = append(items, fmt.Sprintf(`{"id": %q, "name": %q, "duration_ms": %s000, "artists": [{"id": "a1", "name": "Daft Punk"}]}`, t[0], t[1], t[2]))
	}
	return fmt.Sprintf(`{"id": %q, "name": %q, "album_type": "album", "release_date": "2020", "artists": [{"id": "a1", "name": "Daft Punk"}], "images": [],
		"tracks": {"items": [%s], "total": %d}}`, id, id, strings.Join(items, ","), len(tracks))
}

func ids(tracks []media.Track) []string {
	var out []string
	for _, t := range tracks {
		out = append(out, t.ID)
	}
	return out
}

func quickPageWaits(t *testing.T) {
	waits := media.PageWaits
	media.PageWaits = []time.Duration{time.Millisecond, time.Millisecond}
	t.Cleanup(func() { media.PageWaits = waits })
}

// TestArtistSongsReadsEveryRelease reads all pages of an artist's
// releases, past the five the page shows, then the tracks of each, and
// gives a song on a single and on its album once.
func TestArtistSongsReadsEveryRelease(t *testing.T) {
	const albums = "GET /artists/a1/albums?include_groups=album%2Csingle&limit=10&offset="
	responses := map[string]string{
		albums + "0":  albumsJSON(0, 10, 63),
		albums + "10": albumsJSON(10, 10, 63),
		albums + "20": albumsJSON(20, 10, 63),
		albums + "30": albumsJSON(30, 10, 63),
		albums + "40": albumsJSON(40, 10, 63),
		albums + "50": albumsJSON(50, 10, 63),
		albums + "60": albumsJSON(60, 3, 63),
	}
	for i := range 63 {
		// Every release has a song of its own, and Record 0 and Record 1
		// both have "Shared", which is one song.
		own := [3]string{fmt.Sprintf("t%d", i), fmt.Sprintf("Song %d", i), "200"}
		shared := [3]string{fmt.Sprintf("s%d", i), "Shared", "180"}
		if i > 1 {
			responses[fmt.Sprintf("GET /albums/al%d", i)] = albumWith(fmt.Sprintf("al%d", i), own)
		} else {
			responses[fmt.Sprintf("GET /albums/al%d", i)] = albumWith(fmt.Sprintf("al%d", i), shared, own)
		}
	}
	c, api := newFake(t, responses)
	var batches int
	songs, err := c.ArtistSongs(context.Background(), "a1", func(b []media.Track) { batches++ })
	if err != nil {
		t.Fatal(err)
	}
	// 63 songs of their own and the shared one once.
	if len(songs) != 64 || batches != 63 {
		t.Fatalf("%d songs in %d batches (requests %s)", len(songs), batches, api.requests)
	}
	if got := ids(songs)[:3]; !slices.Equal(got, []string{"sp:s0", "sp:t0", "sp:t1"}) {
		t.Errorf("the songs begin %v, out of the releases' order, or the shared one twice", got)
	}
}

// TestArtistSongsPassesOverAFailingRelease keeps the songs of the
// releases that came, and says how many came, and a failing page is tried
// again.
func TestArtistSongsPassesOverAFailingRelease(t *testing.T) {
	quickPageWaits(t)
	var tries atomic.Int32
	const albums = "/artists/a1/albums"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case albums:
			if r.URL.Query().Get("offset") == "0" && tries.Add(1) == 1 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.Write([]byte(albumsJSON(0, 3, 3)))
		case "/albums/al0":
			w.Write([]byte(albumWith("al0", [3]string{"t0", "Zero", "100"})))
		case "/albums/al2":
			w.Write([]byte(albumWith("al2", [3]string{"t2", "Two", "100"})))
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer srv.Close()
	c := testClient(t, srv)
	var got []string
	songs, err := c.ArtistSongs(context.Background(), "a1", func(b []media.Track) { got = append(got, ids(b)...) })
	short, ok := media.Short(err)
	if !ok || short.Got != 2 || short.Total != 3 {
		t.Fatalf("error %v, want a ShortList of 2 of 3", err)
	}
	if !slices.Equal(ids(songs), []string{"sp:t0", "sp:t2"}) || !slices.Equal(got, ids(songs)) {
		t.Errorf("songs %v, given %v", ids(songs), got)
	}
	if tries.Load() < 2 {
		t.Errorf("the first page of releases was asked %d times, not again after failing", tries.Load())
	}
}

// TestArtistSongsAllFailing gives the error alone when no release came,
// and stops when the context ends.
func TestArtistSongsAllFailing(t *testing.T) {
	quickPageWaits(t)
	c, _ := newFake(t, map[string]string{
		"GET /artists/a1/albums": albumsJSON(0, 2, 2),
	})
	songs, err := c.ArtistSongs(context.Background(), "a1", nil)
	if _, short := media.Short(err); err == nil || short || len(songs) != 0 {
		t.Errorf("no release came: %v, %d songs", err, len(songs))
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.ArtistSongs(ctx, "a1", nil); err == nil {
		t.Errorf("a read dropped gave no error")
	}
}
