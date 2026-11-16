package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// slowYouTube is YouTube Music answering a list of the library in two
// pages: first, the cards of the first page, at once; then, once the test
// sends on release, second, the cards of the page after it. It tells on
// asked that the second page was asked for, which then waits for release.
func slowYouTube(t *testing.T, first, second []any) (srv *httptest.Server, asked <-chan struct{}, release chan<- struct{}) {
	t.Helper()
	askedCh, releaseCh := make(chan struct{}, 4), make(chan struct{})
	t.Cleanup(func() { close(releaseCh) })
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Continuation string `json:"continuation"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		switch {
		case strings.HasSuffix(r.URL.Path, "/account/account_menu"):
			json.NewEncoder(w).Encode(accountMenuOf("Me"))
		case !strings.HasSuffix(r.URL.Path, "/browse"):
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{}`))
		case body.Continuation == "":
			json.NewEncoder(w).Encode(gridPage("T1", first...))
		default:
			askedCh <- struct{}{}
			<-releaseCh
			json.NewEncoder(w).Encode(signedInAnswer(map[string]any{"continuationContents": map[string]any{
				"gridContinuation": map[string]any{"items": second}}}))
		}
	}))
	t.Cleanup(srv.Close)
	return srv, askedCh, releaseCh
}

// TestYouTubePlaylistsComeAsTheyAreRead hands a reader that asked the
// playlists read so far, the user's own marked, before the page after the
// first is read, and the whole list, collaborations marked, after it.
func TestYouTubePlaylistsComeAsTheyAreRead(t *testing.T) {
	late := youtubeCard("MUSIC_PAGE_TYPE_PLAYLIST", "VLPLlate", "Late night")
	dawn := youtubeCard("MUSIC_PAGE_TYPE_PLAYLIST", "VLPLdawn", "Dawn")
	srv, asked, release := slowYouTube(t, []any{late}, []any{dawn})
	now := releaseDay
	a := newReleaseApp(t, t.TempDir(), &fakeReleases{asks: map[string]int{}}, &now)
	a.youtube.BaseURL = srv.URL
	a.youtube.SetCookies(testCookies)
	c := &Collection{a: a}

	read := make(chan []media.Playlist, 4)
	ctx := media.WithPartial(context.Background(), func(ps []media.Playlist) { read <- ps })
	type answer struct {
		ps  []media.Playlist
		err error
	}
	done := make(chan answer, 1)
	go func() {
		ps, err := c.YouTubePlaylists(ctx)
		done <- answer{ps, err}
	}()
	select {
	case ps := <-read:
		if len(ps) != 1 || ps[0].ID != "yt:PLlate" || !ps[0].Editable {
			t.Fatalf("the first page came as %+v", ps)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("the first page didn't come")
	}
	<-asked
	select {
	case <-done:
		t.Fatal("the list came whole before its second page")
	default:
	}
	release <- struct{}{}
	select {
	case ps := <-read:
		if len(ps) != 2 || ps[1].ID != "yt:PLdawn" {
			t.Errorf("the second page came as %+v", ps)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("the second page didn't come")
	}
	if got := <-done; got.err != nil || len(got.ps) != 2 {
		t.Errorf("the whole list: %+v, %v", got.ps, got.err)
	}
}

// TestReleasesWaitForTheWholeYouTubeList has new releases read the
// artists followed on YouTube Music whole: it doesn't go on while a page
// of the list is still to come, and takes every artist.
func TestReleasesWaitForTheWholeYouTubeList(t *testing.T) {
	harbour := youtubeCard("MUSIC_PAGE_TYPE_ARTIST", "UCharbour", "Harbour")
	tide := youtubeCard("MUSIC_PAGE_TYPE_ARTIST", "UCtide", "Tide")
	srv, asked, release := slowYouTube(t, []any{harbour}, []any{tide})
	now := releaseDay
	a := newReleaseApp(t, t.TempDir(), &fakeReleases{asks: map[string]int{}}, &now)
	a.youtube.BaseURL = srv.URL
	a.youtube.SetCookies(testCookies)

	type answer struct {
		followed []media.Artist
		lost     map[media.Source]bool
	}
	done := make(chan answer, 1)
	go func() {
		followed, lost := a.followedArtists(context.Background())
		done <- answer{followed, lost}
	}()
	select {
	case <-asked:
	case <-time.After(20 * time.Second):
		t.Fatal("the second page wasn't asked for")
	}
	select {
	case got := <-done:
		t.Fatalf("new releases took %+v with a page still to come", got.followed)
	default:
	}
	release <- struct{}{}
	select {
	case got := <-done:
		ids := []string{}
		for _, ar := range got.followed {
			ids = append(ids, ar.ID)
		}
		slices.Sort(ids)
		if !slices.Equal(ids, []string{"yt:UCharbour", "yt:UCtide"}) || len(got.lost) != 0 {
			t.Errorf("followed %q, lost %v", ids, got.lost)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("the whole list didn't come")
	}
}
