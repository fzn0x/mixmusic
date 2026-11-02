package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/spotify"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

// TestHomeOfEachService asks each service for its shelves of the home page
// on its own: one that is down says why, rather than leaving its shelves
// out unsaid, and a Spotify that answers some of its asks gives the
// shelves of those.
func TestHomeOfEachService(t *testing.T) {
	yt := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer yt.Close()
	var albums atomic.Bool // whether Spotify answers for the saved albums
	sp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/me":
			w.Write([]byte(`{"id": "me", "display_name": "Me"}`))
		case r.URL.Path == "/me/albums" && albums.Load():
			w.Write([]byte(`{"items": [{"album": {"id": "al1", "name": "Mezzanine"}}]}`))
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer sp.Close()
	client := spotify.NewClient(spotify.NewAuth("client"))
	client.BaseURL = sp.URL
	client.SetToken(spotify.Token{AccessToken: "ACCESS", Expiry: time.Now().Add(time.Hour)})
	a := &App{auth: spotify.NewAuth("client"), spotify: client, youtube: youtube.New()}
	a.youtube.BaseURL = yt.URL
	c := &Catalog{a: a}
	ctx := context.Background()

	if shelves, err := c.YouTubeHome(ctx); err == nil {
		t.Errorf("YouTube Music down gave %d shelves and no error", len(shelves))
	}
	if shelves, err := c.SpotifyHome(ctx); err == nil {
		t.Errorf("Spotify down gave %d shelves and no error", len(shelves))
	}
	albums.Store(true)
	shelves, err := c.SpotifyHome(ctx)
	if err != nil || len(shelves) != 1 || shelves[0].Title != "Saved albums" {
		t.Errorf("Spotify with its saved albums alone: %+v, %v", shelves, err)
	}

	// Signed out of Spotify, it has no shelves, and nothing failed.
	signedOut := &Catalog{a: &App{spotify: spotify.NewClient(spotify.NewAuth("client")), youtube: a.youtube}}
	if shelves, err := signedOut.SpotifyHome(ctx); err != nil || len(shelves) != 0 {
		t.Errorf("signed out of Spotify: %+v, %v", shelves, err)
	}
}
