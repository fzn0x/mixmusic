package spotify

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
)

// fakeAPI serves canned responses by path and query, and records the
// requests it gets, which may come at once.
type fakeAPI struct {
	responses map[string]string
	mu        sync.Mutex
	requests  []string
}

func (f *fakeAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := r.Method + " " + r.URL.Path
	f.mu.Lock()
	f.requests = append(f.requests, key+"?"+r.URL.RawQuery)
	f.mu.Unlock()
	body, ok := f.responses[key+"?"+r.URL.RawQuery]
	if !ok {
		body, ok = f.responses[key]
	}
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error": {"status": 404, "message": "Not found"}}`))
		return
	}
	if body == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Write([]byte(strings.ReplaceAll(body, "{{base}}", "http://"+r.Host)))
}

func newFake(t *testing.T, responses map[string]string) (*Client, *fakeAPI) {
	t.Helper()
	api := &fakeAPI{responses: responses}
	srv := httptest.NewServer(api)
	t.Cleanup(srv.Close)
	return testClient(t, srv), api
}

const trackJSON = `{"id": "t1", "name": "One More Time", "type": "track", "duration_ms": 320357, "explicit": false,
	"artists": [{"id": "a1", "name": "Daft Punk"}],
	"album": {"id": "al1", "name": "Discovery", "images": [{"url": "https://i/large", "width": 640, "height": 640}, {"url": "https://i/small", "width": 64, "height": 64}]},
	"external_urls": {"spotify": "https://open.spotify.com/track/t1"}}`

func TestSearch(t *testing.T) {
	c, api := newFake(t, map[string]string{
		"GET /search": `{"tracks": {"items": [` + trackJSON + `]},
			"albums": {"items": [{"id": "al1", "name": "Discovery", "album_type": "album", "release_date": "2001-03-12", "total_tracks": 14, "artists": [{"id": "a1", "name": "Daft Punk"}], "images": []}]},
			"artists": {"items": [{"id": "a1", "name": "Daft Punk", "genres": ["french house"], "images": []}]},
			"playlists": {"items": [null, {"id": "p1", "name": "Mix", "owner": {"id": "u2", "display_name": "Someone"}, "items": {"total": 3}},
				{"id": "p2", "name": "Theirs", "owner": {"id": "u3", "display_name": "Them"}, "items": {"total": 9}},
				{"id": "p3", "name": "Shared", "owner": {"id": "u3", "display_name": "Them"}, "collaborative": true, "items": {"total": 4}}]}}`,
	})
	// Spotify shows apps the tracks of the playlists the user owns or
	// collaborates on only: the others are left out.
	res, err := c.Search(context.Background(), "daft punk", "u2")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(api.requests[len(api.requests)-1], "limit=10") {
		t.Errorf("search asked %s, beyond the limit of development mode", api.requests)
	}
	if len(res.Tracks) != 1 || len(res.Albums) != 1 || len(res.Artists) != 1 || len(res.Playlists) != 2 {
		t.Fatalf("results = %+v", res)
	}
	if res.Playlists[0].ID != "sp:p1" || !res.Playlists[0].Mine || res.Playlists[1].ID != "sp:p3" || !res.Playlists[1].Collaborative {
		t.Errorf("playlists = %+v", res.Playlists)
	}
	// Signed-in user unknown, none can be opened.
	none, err := c.Search(context.Background(), "daft punk", "")
	if err != nil || len(none.Playlists) != 0 {
		t.Errorf("playlists of an unknown user = %+v, %v", none.Playlists, err)
	}
	tr := res.Tracks[0]
	if tr.ID != "sp:t1" || tr.Source != media.SourceSpotify || tr.Album.ID != "sp:al1" || tr.Artists[0].ID != "sp:a1" {
		t.Errorf("track = %+v", tr)
	}
	if tr.Artwork[0].Width != 64 {
		t.Errorf("artwork is not smallest first: %+v", tr.Artwork)
	}
	if res.Albums[0].Year != "2001" || res.Albums[0].Kind != media.AlbumKindAlbum {
		t.Errorf("album = %+v", res.Albums[0])
	}
	if res.Playlists[0].TrackCount != 3 || res.Playlists[0].Owner != "Someone" {
		t.Errorf("playlist = %+v", res.Playlists[0])
	}
}

func TestAlbumReadsAllPages(t *testing.T) {
	c, _ := newFake(t, map[string]string{
		"GET /albums/al1": `{"id": "al1", "name": "Discovery", "album_type": "album", "images": [], "artists": [{"id": "a1", "name": "Daft Punk"}],
			"tracks": {"items": [{"id": "t1", "name": "One", "duration_ms": 1000, "artists": [{"id": "a1", "name": "Daft Punk"}]}],
				"next": "{{base}}/albums/al1/tracks?offset=1&limit=50"}}`,
		"GET /albums/al1/tracks": `{"items": [{"id": "t2", "name": "Two", "duration_ms": 2000, "artists": [{"id": "a1", "name": "Daft Punk"}]}]}`,
	})
	al, err := c.Album(context.Background(), "al1")
	if err != nil {
		t.Fatal(err)
	}
	if len(al.Tracks) != 2 || al.Tracks[1].Title != "Two" {
		t.Fatalf("tracks = %+v", al.Tracks)
	}
	if al.Tracks[1].Album == nil || al.Tracks[1].Album.ID != "sp:al1" {
		t.Errorf("track of the second page lost its album: %+v", al.Tracks[1])
	}
}

func TestPlaylistOwnedReadsItems(t *testing.T) {
	c, _ := newFake(t, map[string]string{
		"GET /playlists/p1": `{"id": "p1", "name": "Mine", "owner": {"id": "me", "display_name": "Me"},
			"items": {"total": 2, "items": [{"item": ` + trackJSON + `}], "next": "{{base}}/playlists/p1/items?offset=1"}}`,
		"GET /playlists/p1/items": `{"total": 2, "items": [{"track": ` + strings.Replace(trackJSON, `"t1"`, `"t2"`, 1) + `}]}`,
	})
	p, err := c.Playlist(context.Background(), "p1", "me")
	if err != nil {
		t.Fatal(err)
	}
	if err := p.ReadOn(context.Background(), c.PlaylistPage, nil); err != nil {
		t.Fatal(err)
	}
	if !p.Editable || len(p.Tracks) != 2 || p.Tracks[1].ID != "sp:t2" {
		t.Errorf("playlist = %+v", p)
	}
}

func TestPlaylistOfOthersIsUnavailable(t *testing.T) {
	c, _ := newFake(t, map[string]string{
		"GET /playlists/p2": `{"id": "p2", "name": "Theirs", "owner": {"id": "them", "display_name": "Them"}}`,
	})
	p, err := c.Playlist(context.Background(), "p2", "me")
	if !errors.Is(err, ErrPlaylistUnavailable) || p.Title != "Theirs" {
		t.Errorf("playlist = %+v, %v", p, err)
	}
}

// TestPlaylistNotShown tells Spotify's own playlists, which Spotify answers
// apps 404 for, from a failure, with their address on Spotify (#530).
func TestPlaylistNotShown(t *testing.T) {
	c, _ := newFake(t, map[string]string{})
	p, err := c.Playlist(context.Background(), "37i9dQZF1DXcBWIGoYBM5M", "me")
	if !errors.Is(err, ErrPlaylistNotShown) || p.ID != "sp:37i9dQZF1DXcBWIGoYBM5M" || p.Source != media.SourceSpotify ||
		p.URL != "https://open.spotify.com/playlist/37i9dQZF1DXcBWIGoYBM5M" {
		t.Errorf("playlist = %+v, %v", p, err)
	}
}
