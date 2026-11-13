package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/spotify"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

// TestSearchListsSpotifyPlaylistsItOpens leaves out of a search the Spotify
// playlists of others, whose tracks Spotify shows apps none of (#530), and
// keeps the user's own.
func TestSearchListsSpotifyPlaylistsItOpens(t *testing.T) {
	yt := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer yt.Close()
	sp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/me":
			w.Write([]byte(`{"id": "me", "display_name": "Me"}`))
		case "/search":
			w.Write([]byte(`{"playlists": {"items": [
				{"id": "p1", "name": "Thai Viral Song", "owner": {"id": "them", "display_name": "Them"}},
				{"id": "p2", "name": "My Thai mix", "owner": {"id": "me", "display_name": "Me"}}]}}`))
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

	res, err := (&Catalog{a: a}).search(context.Background(), "thailand")
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Spotify.Playlists; len(got) != 1 || got[0].Title != "My Thai mix" {
		t.Errorf("Spotify's playlists = %+v, want only the user's own", got)
	}
}
