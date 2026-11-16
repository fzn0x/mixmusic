package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/spotify"
)

// TestCatalogArtistSongs reads the songs of a Spotify artist through the
// catalog, from a fake Spotify, and says an artist of another service has
// no way yet.
func TestCatalogArtistSongs(t *testing.T) {
	sp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/artists/a1/albums":
			w.Write([]byte(`{"items": [{"id": "al1", "name": "One", "album_type": "album", "release_date": "2020", "artists": [], "images": []}], "total": 1}`))
		case "/albums/al1":
			w.Write([]byte(`{"id": "al1", "name": "One", "album_type": "album", "release_date": "2020", "artists": [], "images": [],
				"tracks": {"items": [{"id": "t1", "name": "First", "duration_ms": 1000, "artists": [{"id": "a1", "name": "A"}]},
				{"id": "t2", "name": "Second", "duration_ms": 2000, "artists": [{"id": "a1", "name": "A"}]}], "total": 2}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer sp.Close()
	now := releaseDay
	a := newReleaseApp(t, t.TempDir(), &fakeReleases{asks: map[string]int{}}, &now)
	a.spotify.BaseURL = sp.URL
	a.spotify.SetToken(spotify.Token{AccessToken: "ACCESS", Expiry: time.Now().Add(time.Hour)})
	c := &Catalog{a: a}
	var given []string
	songs, err := c.ArtistSongs(context.Background(), "sp:a1", func(b []media.Track) {
		for _, t := range b {
			given = append(given, t.ID)
		}
	})
	if err != nil || !slices.Equal(given, []string{"sp:t1", "sp:t2"}) || len(songs) != 2 {
		t.Errorf("songs %v, given %v, error %v", songs, given, err)
	}
	if _, err := c.ArtistSongs(context.Background(), "file:a1", nil); !errors.Is(err, ErrNoArtistSongs) {
		t.Errorf("an artist of music on this computer: %v", err)
	}
}
