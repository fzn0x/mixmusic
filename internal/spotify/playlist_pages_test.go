package spotify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// TestPlaylistFirstPage reads the first page of a playlist Spotify gives
// without its tracks, and tells where the next is, which only the API
// is asked for.
func TestPlaylistFirstPage(t *testing.T) {
	c, api := newFake(t, map[string]string{
		"GET /playlists/p1": `{"id": "p1", "name": "Mine", "owner": {"id": "me", "display_name": "Me"}}`,
		"GET /playlists/p1/items?limit=50": `{"total": 3, "items": [{"item": ` + trackJSON + `}],
			"next": "{{base}}/playlists/p1/items?offset=1&limit=50"}`,
		"GET /playlists/p1/items?offset=1&limit=50": `{"total": 3, "items": [{"item": ` + strings.Replace(trackJSON, `"t1"`, `"t2"`, 1) + `},
			{"item": null}, {"item": ` + strings.Replace(trackJSON, `"t1"`, `"t3"`, 1) + `}]}`,
	})
	ctx := context.Background()
	p, err := c.Playlist(ctx, "p1", "me")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Tracks) != 1 || p.TrackCount != 3 || !strings.HasSuffix(p.More, "/playlists/p1/items?offset=1&limit=50") {
		t.Fatalf("the first page: %d tracks of %d, more %q", len(p.Tracks), p.TrackCount, p.More)
	}
	tracks, next, err := c.PlaylistPage(ctx, p.More)
	if err != nil || len(tracks) != 2 || tracks[1].ID != "sp:t3" || next != "" {
		t.Fatalf("the next page: %+v, next %q, %v", tracks, next, err)
	}

	asked := len(api.requests)
	if _, _, err := c.PlaylistPage(ctx, "https://example.com/v1/playlists/p1/items?offset=1"); err == nil || len(api.requests) != asked {
		t.Errorf("a page off the API was asked for: %v", err)
	}
}

// TestAlbumPagesTriedAgain asks again for a page of an album's tracks that
// fails, and fails the album once it failed each time, rather than show
// the album cut short.
func TestAlbumPagesTriedAgain(t *testing.T) {
	waits := media.PageWaits
	media.PageWaits = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}
	t.Cleanup(func() { media.PageWaits = waits })
	var fails atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/albums/al1":
			w.Write([]byte(strings.ReplaceAll(`{"id": "al1", "name": "Discovery", "album_type": "album", "images": [], "artists": [{"id": "a1", "name": "Daft Punk"}],
				"tracks": {"items": [{"id": "t1", "name": "One", "duration_ms": 1000, "artists": [{"id": "a1", "name": "Daft Punk"}]}],
				"next": "{{base}}/albums/al1/tracks?offset=1&limit=50"}}`, "{{base}}", "http://"+r.Host)))
		case "/albums/al1/tracks":
			if fails.Add(-1) >= 0 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.Write([]byte(`{"items": [{"id": "t2", "name": "Two", "duration_ms": 2000, "artists": [{"id": "a1", "name": "Daft Punk"}]}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	c := testClient(t, srv)

	fails.Store(3)
	al, err := c.Album(context.Background(), "al1")
	if err != nil || len(al.Tracks) != 2 {
		t.Fatalf("a page that failed three times: %d tracks, %v", len(al.Tracks), err)
	}
	fails.Store(4)
	if al, err := c.Album(context.Background(), "al1"); err == nil {
		t.Errorf("a page that failed four times gave %d tracks", len(al.Tracks))
	}
}
