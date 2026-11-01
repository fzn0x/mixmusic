package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/library"
	"github.com/fzn0x/mixmusic/internal/match"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/spotify"
	"github.com/fzn0x/mixmusic/internal/store"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

// asked notes the requests a fake service took, as "METHOD path?query".
type asked struct {
	mu   sync.Mutex
	reqs []string
}

func (a *asked) note(r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	line := r.Method + " " + r.URL.Path
	if r.URL.Path == "/me/library" {
		line += "?" + r.URL.Query().Get("uris")
	}
	a.reqs = append(a.reqs, line)
}

// take returns the requests noted since the last take.
func (a *asked) take() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := a.reqs
	a.reqs = nil
	return out
}

// dislikeApp is an app with a library and settings in a folder of their
// own, whose YouTube Music and Spotify are yt and sp, signed in.
func dislikeApp(t *testing.T, yt, sp http.HandlerFunc) *App {
	t.Helper()
	dir := t.TempDir()
	lib, err := library.Open(filepath.Join(dir, "library.json"))
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(dir, "settings.json"), settings.Default())
	if err != nil {
		t.Fatal(err)
	}
	a := &App{library: lib, settings: st, youtube: youtube.New(), spotify: spotify.NewClient(spotify.NewAuth("client"))}
	a.matcher = match.New(a.youtube, nil)
	ytSrv := httptest.NewServer(yt)
	t.Cleanup(ytSrv.Close)
	a.youtube.BaseURL = ytSrv.URL
	a.youtube.SetCookies(testCookies)
	spSrv := httptest.NewServer(sp)
	t.Cleanup(spSrv.Close)
	a.spotify.BaseURL = spSrv.URL
	a.spotify.SetToken(spotify.Token{AccessToken: "ACCESS", Expiry: time.Now().Add(time.Hour)})
	return a
}

// TestSetDisliked keeps dislikes in the library, and sends those of
// YouTube Music tracks there, signed in. A dislike that takes the like of
// a Spotify track back takes it out of the user's Spotify library.
func TestSetDisliked(t *testing.T) {
	var yt, sp asked
	a := dislikeApp(t, func(w http.ResponseWriter, r *http.Request) {
		yt.note(r)
		w.Write([]byte(`{"responseContext":{}}`))
	}, func(w http.ResponseWriter, r *http.Request) {
		sp.note(r)
		w.WriteHeader(http.StatusOK)
	})
	c := &Collection{a}
	ctx := context.Background()
	song := media.Track{ID: "yt:vid", Source: media.SourceYouTube, Title: "Song"}
	sptrack := media.Track{ID: "sp:abc", Source: media.SourceSpotify, Title: "Track"}
	other := media.Track{ID: "sp:def", Source: media.SourceSpotify, Title: "Other"}

	if err := c.SetLiked(ctx, song, true); err != nil {
		t.Fatal(err)
	}
	if err := c.SetDisliked(ctx, song, true); err != nil {
		t.Fatal(err)
	}
	if !a.IsDisliked("yt:vid") || a.IsLiked("yt:vid") || len(c.Disliked()) != 1 {
		t.Errorf("disliked %v, liked %v", a.IsDisliked("yt:vid"), a.IsLiked("yt:vid"))
	}
	if got := yt.take(); !slices.Equal(got, []string{"POST /youtubei/v1/like/like", "POST /youtubei/v1/like/dislike"}) {
		t.Errorf("YouTube Music was asked %q", got)
	}
	if err := c.SetDisliked(ctx, song, false); err != nil {
		t.Fatal(err)
	}
	if got := yt.take(); !slices.Equal(got, []string{"POST /youtubei/v1/like/removelike"}) {
		t.Errorf("taking the dislike back asked %q", got)
	}
	if a.IsDisliked("yt:vid") || a.IsLiked("yt:vid") {
		t.Error("taking the dislike back left it disliked, or liked it")
	}

	// Spotify takes no dislikes, but loses the like a dislike took back.
	if err := c.SetLiked(ctx, sptrack, true); err != nil {
		t.Fatal(err)
	}
	if err := c.SetDisliked(ctx, sptrack, true); err != nil {
		t.Fatal(err)
	}
	if err := c.SetDisliked(ctx, other, true); err != nil {
		t.Fatal(err)
	}
	if got := sp.take(); !slices.Equal(got, []string{"PUT /me/library?spotify:track:abc", "DELETE /me/library?spotify:track:abc"}) {
		t.Errorf("Spotify was asked %q", got)
	}
	if got := yt.take(); len(got) != 0 {
		t.Errorf("disliking Spotify tracks asked YouTube Music %q", got)
	}
	if d := c.Disliked(); len(d) != 2 || d[0].ID != "sp:def" {
		t.Errorf("disliked %+v", d)
	}

	// Liking a disliked track takes the dislike back.
	if err := c.SetLiked(ctx, other, true); err != nil {
		t.Fatal(err)
	}
	if a.IsDisliked("sp:def") || !a.IsLiked("sp:def") {
		t.Error("liking a disliked track left it disliked")
	}

	// Signed out of YouTube Music, a dislike stays in MixMusic.
	a.youtube.SetCookies(nil)
	if err := c.SetDisliked(ctx, song, true); err != nil {
		t.Fatal(err)
	}
	if got := yt.take(); len(got) != 0 || !a.IsDisliked("yt:vid") {
		t.Errorf("signed out, disliking asked %q; disliked %v", got, a.IsDisliked("yt:vid"))
	}
}

// TestWithoutDisliked leaves out of a radio the tracks the user disliked,
// and the other uploads of their songs.
func TestWithoutDisliked(t *testing.T) {
	song := func(id, title, artist string) media.Track {
		return media.Track{ID: id, Title: title, Artists: []media.Ref{{Name: artist}}}
	}
	radio := []media.Track{
		song("yt:1", "Blue", "Ann"),
		song("yt:2", "Blue (Official Video)", "Ann"),
		song("yt:3", "Red", "Bo"),
		song("yt:4", "Blue", "Cy"),
		song("yt:5", "Green", "Di"),
	}
	if got := withoutDisliked(radio, nil); len(got) != len(radio) {
		t.Errorf("without dislikes, kept %d of %d", len(got), len(radio))
	}
	got := withoutDisliked(radio, []media.Track{song("sp:x", "Blue", "ANN"), song("yt:5", "Green", "Di")})
	var ids []string
	for _, tr := range got {
		ids = append(ids, tr.ID)
	}
	if !slices.Equal(ids, []string{"yt:3", "yt:4"}) {
		t.Errorf("kept %v", ids)
	}
}

// TestRadioWithoutDisliked has a radio, as YouTube Music answered one,
// leave out the songs the user dislikes while the settings skip them.
func TestRadioWithoutDisliked(t *testing.T) {
	answer, err := os.ReadFile(filepath.Join("..", "youtube", "testdata", "next.json"))
	if err != nil {
		t.Fatal(err)
	}
	a := dislikeApp(t, func(w http.ResponseWriter, r *http.Request) { w.Write(answer) },
		func(w http.ResponseWriter, r *http.Request) {})
	c := &Catalog{a}
	ctx := context.Background()
	seed := media.Track{ID: "yt:seed", Source: media.SourceYouTube, Title: "Seed"}
	all, err := c.Radio(ctx, seed)
	if err != nil || len(all) < 3 {
		t.Fatalf("the radio: %d tracks, %v", len(all), err)
	}
	upload := all[1]
	upload.ID, upload.Title = "yt:another", upload.Title+" (Official Video)"
	for _, tr := range []media.Track{all[0], upload} {
		if _, err := a.library.SetDisliked(tr, true); err != nil {
			t.Fatal(err)
		}
	}
	got, err := c.Radio(ctx, seed)
	if err != nil {
		t.Fatal(err)
	}
	for _, tr := range got {
		if tr.ID == all[0].ID || player.SongKey(tr) == player.SongKey(all[1]) {
			t.Errorf("the radio kept %s, %q", tr.ID, tr.Title)
		}
	}
	if !slices.ContainsFunc(got, func(tr media.Track) bool { return tr.ID == all[2].ID }) {
		t.Errorf("the radio lost %s", all[2].ID)
	}
	st := a.settings.Get()
	st.SkipDisliked = false
	if err := a.settings.Set(st); err != nil {
		t.Fatal(err)
	}
	if got, err := c.Radio(ctx, seed); err != nil || len(got) != len(all) {
		t.Errorf("not skipping, the radio has %d tracks of %d, %v", len(got), len(all), err)
	}
}
