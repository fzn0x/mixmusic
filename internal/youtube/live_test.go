package youtube

import (
	"context"
	"net/http"
	"os"
	"strings"
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
	// The save button is how a page tells a saved playlist, whose icons
	// YouTube Music renames now and then.
	page, err := c.browse(ctx, "VL"+playlistID)
	if err != nil || !saveToggle(responsiveHeader(page)).exists() {
		t.Fatalf("playlist %s: no save button in its header, %v", playlistID, err)
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

// TestLiveDeletePlaylist makes a playlist on a real account, finds YouTube
// offering to delete it, on its page and in the library, and deletes it.
// It changes the account, so it runs only with MIXMUSIC_YT_WRITE=1 too,
// best with an account kept for tests:
//
//	MIXMUSIC_LIVE=1 MIXMUSIC_YT_WRITE=1 MIXMUSIC_YT_COOKIES='SAPISID=…; …' go test ./internal/youtube -run LiveDeletePlaylist -v
//
// It logs what the tests here can't know from answers read signed out:
// where the delete endpoint sits, what YouTube answers a delete, and a
// delete of a playlist that's gone.
func TestLiveDeletePlaylist(t *testing.T) {
	header := os.Getenv("MIXMUSIC_YT_COOKIES")
	if os.Getenv("MIXMUSIC_LIVE") == "" || os.Getenv("MIXMUSIC_YT_WRITE") == "" || header == "" {
		t.Skip("set MIXMUSIC_LIVE=1, MIXMUSIC_YT_WRITE=1 and MIXMUSIC_YT_COOKIES to delete a playlist of a signed-in account")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	c := New()
	cookies, err := http.ParseCookie(header)
	if err != nil {
		t.Fatal(err)
	}
	c.SetCookies(cookies)
	id, err := c.CreatePlaylist(ctx, "MixMusic live test "+time.Now().Format("2006-01-02 15:04:05"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(id, "PL") {
		t.Fatalf("the new playlist's ID is %q", id)
	}
	deleted := false
	t.Cleanup(func() {
		if !deleted {
			if err := c.DeletePlaylist(context.Background(), id); err != nil {
				t.Errorf("deleting %s at the end: %v", id, err)
			}
		}
	})
	// eventually tries for a few seconds, as YouTube's lists catch up, and
	// tells whether try came true; last tells what the last try saw.
	var last any
	eventually := func(what string, try func() bool) {
		t.Helper()
		for start := time.Now(); !try(); time.Sleep(time.Second) {
			if time.Since(start) > 20*time.Second {
				t.Fatalf("%s: not after 20 s; the last answer: %v", what, last)
			}
		}
	}

	eventually("its page offers to delete it", func() bool {
		n, err := c.browse(ctx, "VL"+id)
		if err != nil {
			last = err
			return false
		}
		head := n.get("contents", "twoColumnBrowseResultsRenderer", "tabs", 0, "tabRenderer", "content", "sectionListRenderer", "contents", 0)
		last = head.v
		p, _, err := parsePlaylist(n, id)
		if err != nil || !p.Deletable {
			return false
		}
		endpoint := head.find("deletePlaylistEndpoint")
		t.Logf("the page's delete endpoint: %v; editable %v, saved %v", endpoint.v, p.Editable, p.Saved)
		if endpoint.get("deletePlaylistContext").exists() {
			t.Errorf("the endpoint has a deletePlaylistContext, which DeletePlaylist doesn't send: %v", endpoint.v)
		}
		return true
	})
	eventually("the library offers to delete it", func() bool {
		n, err := c.browse(ctx, "FEmusic_liked_playlists")
		if err != nil {
			last = err
			return false
		}
		for _, s := range parseShelves(n) {
			for _, it := range s.Items {
				if p := it.Playlist; p != nil && p.ID == media.ID(media.SourceYouTube, id) {
					last = p
					return p.Deletable && p.Editable && !p.Saved
				}
			}
		}
		last = "no tile of it"
		return false
	})
	t.Logf("in the library: %+v", last)

	answer, err := c.deletePlaylist(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	deleted = true
	t.Logf("deleted: %v", answer.v)
	if got := answer.find("handlePlaylistDeletionCommand").get("playlistId").str(); got != id {
		t.Errorf("the answer's handlePlaylistDeletionCommand names %q", got)
	}
	eventually("its page goes", func() bool {
		_, err := c.Playlist(ctx, id)
		return err != nil
	})
	// Deleting it again is what a delete of a playlist that's gone gets:
	// errDeleteRefused takes its status.
	_, again := c.call(ctx, "playlist/delete", map[string]any{"playlistId": id}, nil)
	t.Logf("deleting it again: %v", again)
	if _, err := c.Account(ctx); err != nil || c.Rejected() {
		t.Errorf("the session after a refused delete: %v, rejected %v", err, c.Rejected())
	}
}
