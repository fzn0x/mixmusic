package youtube

import (
	"context"
	"errors"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// TestLive checks the parsers against YouTube Music itself, which changes
// its responses from time to time. It runs with MIXMUSIC_LIVE=1:
//
//	MIXMUSIC_LIVE=1 go test ./internal/youtube -run Live -v
func TestLive(t *testing.T) {
	if os.Getenv("MIXMUSIC_LIVE") == "" {
		t.Skip("set MIXMUSIC_LIVE=1 to test against YouTube Music")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	c := New()

	results, err := c.SearchAll(ctx, "daft punk")
	if err != nil {
		t.Fatal(err)
	}
	if len(results.Tracks) == 0 || len(results.Albums) == 0 || len(results.Artists) == 0 || len(results.Playlists) == 0 {
		t.Fatalf("search found %d tracks, %d albums, %d artists, %d playlists",
			len(results.Tracks), len(results.Albums), len(results.Artists), len(results.Playlists))
	}
	t.Logf("first track: %s by %s", results.Tracks[0].Title, results.Tracks[0].ArtistNames())

	_, albumID, _ := media.ParseID(results.Albums[0].ID)
	album, err := c.Album(ctx, albumID)
	if err != nil || len(album.Tracks) == 0 {
		t.Fatalf("album %s: %d tracks, %v", albumID, len(album.Tracks), err)
	}

	_, artistID, _ := media.ParseID(results.Artists[0].ID)
	artist, err := c.Artist(ctx, artistID)
	if err != nil || len(artist.TopTracks) == 0 {
		t.Fatalf("artist %s: %d top tracks, %v", artistID, len(artist.TopTracks), err)
	}

	_, playlistID, _ := media.ParseID(results.Playlists[0].ID)
	playlist, err := c.Playlist(ctx, playlistID)
	if err != nil || len(playlist.Tracks) == 0 {
		t.Fatalf("playlist %s: %d tracks, %v", playlistID, len(playlist.Tracks), err)
	}

	_, videoID, _ := media.ParseID(results.Tracks[0].ID)
	radio, err := c.Radio(ctx, videoID)
	if err != nil || len(radio) < 10 {
		t.Fatalf("radio of %s: %d tracks, %v", videoID, len(radio), err)
	}

	shelves, err := c.Home(ctx)
	if err != nil || len(shelves) == 0 {
		t.Fatalf("home: %d shelves, %v", len(shelves), err)
	}

	suggestions, err := c.Suggestions(ctx, "daft p")
	if err != nil || len(suggestions) == 0 {
		t.Fatalf("suggestions: %v, %v", suggestions, err)
	}
}

// TestLiveLabels reads the label of AI of videos YouTube labels, an upload
// and an art track, and of one it doesn't. It runs with MIXMUSIC_LIVE=1. A
// video YouTube no longer has is passed over.
func TestLiveLabels(t *testing.T) {
	if os.Getenv("MIXMUSIC_LIVE") == "" {
		t.Skip("set MIXMUSIC_LIVE=1 to test against YouTube")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	c := New()
	for id, want := range map[string]bool{"NWuajLluF_E": true, "_d9BrC8DEcs": true, "dQw4w9WgXcQ": false} {
		ai, err := c.MadeWithAI(ctx, id)
		switch {
		case errors.Is(err, ErrNotFound):
			t.Logf("YouTube has no video %s any more", id)
		case err != nil:
			t.Errorf("label of %s: %v", id, err)
		case ai != want:
			t.Errorf("label of %s = %v, want %v", id, ai, want)
		}
	}
}

// TestLiveSignedIn reads the library of a real account. It runs with the
// Cookie header of a signed-in music.youtube.com tab, from the browser's
// developer tools:
//
//	MIXMUSIC_LIVE=1 MIXMUSIC_YT_COOKIES='SAPISID=…; …' go test ./internal/youtube -run LiveSignedIn -v
func TestLiveSignedIn(t *testing.T) {
	header := os.Getenv("MIXMUSIC_YT_COOKIES")
	if os.Getenv("MIXMUSIC_LIVE") == "" || header == "" {
		t.Skip("set MIXMUSIC_LIVE=1 and MIXMUSIC_YT_COOKIES to test a signed-in account")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	c := New()
	cookies, err := http.ParseCookie(header)
	if err != nil {
		t.Fatal(err)
	}
	c.SetCookies(cookies)
	a, err := c.Account(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("signed in as %s %s", a.Name, a.Handle)
	liked, err := c.LikedSongs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	playlists, err := c.LibraryPlaylists(ctx)
	if err != nil {
		t.Fatal(err)
	}
	albums, err := c.LibraryAlbums(ctx)
	if err != nil {
		t.Fatal(err)
	}
	artists, err := c.LibraryArtists(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%d liked songs, %d playlists, %d albums, %d artists", len(liked.Tracks), len(playlists), len(albums), len(artists))
	home, err := c.Home(ctx)
	if err != nil || len(home) == 0 {
		t.Fatalf("home: %d shelves, %v", len(home), err)
	}
}
