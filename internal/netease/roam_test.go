package netease

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestRoam plays the songs of three of a tag's playlists, each once, at
// most 100 of them.
func TestRoam(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/weapi/playlist/list":
			fmt.Fprint(w, `{"code":200,"playlists":[{"id":1},{"id":2},{"id":3},{"id":4}]}`)
		case "/weapi/v6/playlist/detail":
			// Every playlist holds the same 80 songs: 80 in all.
			ids := make([]string, 80)
			for i := range ids {
				ids[i] = fmt.Sprintf(`{"id":%d}`, i+1)
			}
			fmt.Fprintf(w, `{"code":200,"playlist":{"trackIds":[%s]}}`, strings.Join(ids, ","))
		case "/weapi/v3/song/detail":
			songs := make([]string, 80)
			for i := range songs {
				songs[i] = fmt.Sprintf(`{"id":%d,"name":"s"}`, i+1)
			}
			fmt.Fprintf(w, `{"code":200,"songs":[%s]}`, strings.Join(songs, ","))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := New()
	c.BaseURL = srv.URL
	tracks, err := c.Roam(context.Background(), "90后")
	if err != nil {
		t.Fatal(err)
	}
	if len(tracks) != 80 {
		t.Errorf("Roam gave %d tracks, want the 80 songs once each", len(tracks))
	}
}
