package spotify

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// playlistRoutes is a fake Spotify that answers the new and the old route
// of saving a playlist with the statuses it is given, and records the
// requests.
type playlistRoutes struct {
	library, followers int
	mu                 sync.Mutex
	requests           []string
}

func (p *playlistRoutes) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	p.requests = append(p.requests, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery)
	p.mu.Unlock()
	status := p.followers
	if r.URL.Path == "/me/library" {
		status = p.library
	}
	if status >= 400 {
		w.WriteHeader(status)
		fmt.Fprintf(w, `{"error": {"status": %d, "message": "Not found"}}`, status)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func TestFollowPlaylistRoutes(t *testing.T) {
	cases := []struct {
		name               string
		library, followers int
		on                 bool
		want               []string
		fails              bool
	}{
		{"save by the new route", 204, 500, true, []string{"PUT /me/library?uris=spotify%3Aplaylist%3Ap1"}, false},
		{"remove by the new route", 204, 500, false, []string{"DELETE /me/library?uris=spotify%3Aplaylist%3Ap1"}, false},
		{"403 falls back", 403, 204, true, []string{"PUT /me/library?uris=spotify%3Aplaylist%3Ap1", "PUT /playlists/p1/followers?"}, false},
		{"404 falls back", 404, 204, false, []string{"DELETE /me/library?uris=spotify%3Aplaylist%3Ap1", "DELETE /playlists/p1/followers?"}, false},
		{"both fail", 404, 403, true, []string{"PUT /me/library?uris=spotify%3Aplaylist%3Ap1", "PUT /playlists/p1/followers?"}, true},
		{"another error doesn't fall back", 500, 204, true, []string{"PUT /me/library?uris=spotify%3Aplaylist%3Ap1"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			api := &playlistRoutes{library: tc.library, followers: tc.followers}
			srv := httptest.NewServer(api)
			t.Cleanup(srv.Close)
			c := testClient(t, srv)
			err := c.FollowPlaylist(context.Background(), "p1", tc.on)
			if (err != nil) != tc.fails {
				t.Fatalf("error = %v, want failure %v", err, tc.fails)
			}
			if strings.Join(api.requests, "\n") != strings.Join(tc.want, "\n") {
				t.Errorf("requests = %q, want %q", api.requests, tc.want)
			}
		})
	}
}

func TestFollowPlaylistBothFailSaysSo(t *testing.T) {
	api := &playlistRoutes{library: 404, followers: 403}
	srv := httptest.NewServer(api)
	t.Cleanup(srv.Close)
	err := testClient(t, srv).FollowPlaylist(context.Background(), "p1", true)
	if err == nil {
		t.Fatal("no error")
	}
	for _, want := range []string{"not saved", "/me/library answered", "/playlists/{id}/followers answered"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Errorf("the Spotify error is not reachable from %v", err)
	}
}
