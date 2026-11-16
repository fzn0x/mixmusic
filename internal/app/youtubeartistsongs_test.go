package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
)

// TestYouTubeArtistSongs reads all the songs of a YouTube Music artist by
// their ID, from the list their recorded page links, served as the long
// playlist, giving each page's songs as they come.
func TestYouTubeArtistSongs(t *testing.T) {
	l := newLongPlaylist(t)
	c := l.catalog(t)
	artist, err := os.ReadFile("../youtube/testdata/artist.json")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		var body struct {
			BrowseID string `json:"browseId"`
		}
		json.Unmarshal(data, &body)
		if strings.HasPrefix(body.BrowseID, "UC") {
			w.Write(artist)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(data))
		l.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	c.a.youtube.BaseURL = srv.URL
	var given []int
	songs, err := c.YouTubeArtistSongs(context.Background(), "yt:UCbM50mQtG58vhLZonicYUrQ", func(batch []media.Track) {
		given = append(given, len(batch))
	})
	if err != nil || len(songs) != 6 || !slices.Equal(given, []int{2, 2, 2}) {
		t.Errorf("%d songs, given %v, %v", len(songs), given, err)
	}
	// ArtistSongs routes YouTube Music's artists here.
	if routed, err := c.ArtistSongs(context.Background(), "yt:UCbM50mQtG58vhLZonicYUrQ", func([]media.Track) {}); err != nil || len(routed) != 6 {
		t.Errorf("ArtistSongs gave %d songs, %v", len(routed), err)
	}
	if _, err := c.YouTubeArtistSongs(context.Background(), "sp:a1", nil); err == nil {
		t.Error("a Spotify artist's songs came")
	}
}
