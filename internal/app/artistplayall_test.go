package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/player/playertest"
	"github.com/fzn0x/mixmusic/internal/spotify"
)

// TestSongsReaderByService reads a Spotify artist with releases from
// them, a YouTube Music artist from the list their page links, and no
// other: their top songs are all there are.
func TestSongsReaderByService(t *testing.T) {
	c := &Catalog{}
	for _, tc := range []struct {
		name string
		a    media.Artist
		want bool
	}{
		{"spotify with releases", media.Artist{ID: "sp:a", Source: media.SourceSpotify, Albums: []media.Album{{ID: "sp:al"}}}, true},
		{"spotify with singles", media.Artist{ID: "sp:a", Source: media.SourceSpotify, Singles: []media.Album{{ID: "sp:al"}}}, true},
		{"spotify without releases", media.Artist{ID: "sp:a", Source: media.SourceSpotify}, false},
		{"youtube with a list", media.Artist{ID: "yt:UC", Source: media.SourceYouTube, SongsID: "yt:VLPL1"}, true},
		{"youtube without a list", media.Artist{ID: "yt:UC", Source: media.SourceYouTube}, false},
		{"this computer's music", media.Artist{ID: "file:a", Source: media.SourceFile}, false},
	} {
		if got := c.songsReader(tc.a) != nil; got != tc.want {
			t.Errorf("%s: reads %v, want %v", tc.name, got, tc.want)
		}
	}
}

// spotifyArtist serves one artist with two releases.
func spotifyArtist(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/artists/a1/albums":
			w.Write([]byte(`{"items": [{"id": "al1", "name": "One", "album_type": "album", "release_date": "2020", "artists": [], "images": []},
				{"id": "al2", "name": "Two", "album_type": "single", "release_date": "2021", "artists": [], "images": []}], "total": 2}`))
		case "/albums/al1":
			w.Write([]byte(`{"id": "al1", "name": "One", "album_type": "album", "release_date": "2020", "artists": [], "images": [],
				"tracks": {"items": [{"id": "t1", "name": "First", "duration_ms": 1000, "artists": [{"id": "a1", "name": "A"}]}], "total": 1}}`))
		case "/albums/al2":
			w.Write([]byte(`{"id": "al2", "name": "Two", "album_type": "single", "release_date": "2021", "artists": [], "images": [],
				"tracks": {"items": [{"id": "t2", "name": "Second", "duration_ms": 2000, "artists": [{"id": "a1", "name": "A"}]}], "total": 1}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestPlayArtistSpotifySongs plays and shuffles all of a Spotify artist's
// songs through the one path that serves every service.
func TestPlayArtistSpotifySongs(t *testing.T) {
	sp := spotifyArtist(t)
	a := newQuietApp(t, nil)
	a.spotify.BaseURL = sp.URL
	a.spotify.SetToken(spotify.Token{AccessToken: "ACCESS", Expiry: time.Now().Add(time.Hour)})
	c := &Catalog{a: a}
	artist := media.Artist{ID: "sp:a1", Source: media.SourceSpotify, Name: "A", Albums: []media.Album{{ID: "sp:al1"}},
		TopTracks: []media.Track{{ID: "sp:top", Source: media.SourceSpotify, Title: "Top"}}}
	for _, shuffled := range []bool{false, true} {
		p := player.New(songsServices{}, playertest.Stream{}, playertest.Device{}, player.Saved{})
		t.Cleanup(p.Close)
		if err := c.PlayArtist(context.Background(), p, artist, shuffled); err != nil {
			t.Fatal(err)
		}
		waitPlayer(t, p, "both releases queued", func(s player.State) bool {
			ids := slices.Sorted(slices.Values(queueIDs(s)))
			return s.Shuffle == shuffled && slices.Equal(ids, []string{"sp:t1", "sp:t2"})
		})
	}
	songs, err := c.AllSongs(context.Background(), artist)
	if err != nil || !slices.Equal(trackIDs(songs), []string{"sp:t1", "sp:t2"}) {
		t.Errorf("all songs %v, %v", trackIDs(songs), err)
	}
}

// TestPlayArtistID reads the artist as their page does, then plays all
// their songs, shuffled or not; an artist that doesn't open says so.
func TestPlayArtistID(t *testing.T) {
	as := newArtistSongs(t, nil)
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
		r.Body = io.NopCloser(strings.NewReader(string(data)))
		as.l.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	as.c.a.youtube.BaseURL = srv.URL
	const id = "yt:UCbM50mQtG58vhLZonicYUrQ"
	if err := as.c.PlayArtistID(context.Background(), as.p, id, true); err != nil {
		t.Fatal(err)
	}
	want := slices.Sorted(slices.Values(trackIDs(as.whole)))
	waitPlayer(t, as.p, "all the songs, shuffled", func(s player.State) bool {
		return s.Shuffle && slices.Equal(slices.Sorted(slices.Values(queueIDs(s))), want)
	})
	err = as.c.PlayArtistID(context.Background(), as.p, "nope", false)
	if err == nil || !strings.Contains(err.Error(), "couldn't be opened") {
		t.Errorf("an artist that isn't there: %v", err)
	}
}

// TestAllSongsOfArtists reads a YouTube Music artist's list whole, and
// gives the top songs of an artist with no way to read more.
func TestAllSongsOfArtists(t *testing.T) {
	as := newArtistSongs(t, nil)
	songs, err := as.c.AllSongs(context.Background(), as.artist)
	if err != nil || !slices.Equal(trackIDs(songs), trackIDs(as.whole)) {
		t.Errorf("all songs %v, %v", trackIDs(songs), err)
	}
	bare := as.artist
	bare.SongsID = ""
	songs, err = as.c.AllSongs(context.Background(), bare)
	if err != nil || !slices.Equal(trackIDs(songs), []string{"yt:top1"}) {
		t.Errorf("top songs %v, %v", trackIDs(songs), err)
	}
	as.l.fail[""] = 4
	songs, err = as.c.AllSongs(context.Background(), as.artist)
	if err == nil || !slices.Equal(trackIDs(songs), []string{"yt:top1"}) {
		t.Errorf("a list that fails: %v, %v", trackIDs(songs), err)
	}
}
