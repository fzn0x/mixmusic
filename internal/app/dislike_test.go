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

// requestLog notes the requests a fake service took, as "METHOD path?query".
type requestLog struct {
	mu   sync.Mutex
	reqs []string
}

func (a *requestLog) note(r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	line := r.Method + " " + r.URL.Path
	if r.URL.Path == "/me/library" {
		line += "?" + r.URL.Query().Get("uris")
	}
	a.reqs = append(a.reqs, line)
}

// take returns the requests noted since the last take.
func (a *requestLog) take() []string {
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
	var yt, sp requestLog
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

// TestRadioOfAVideoWithoutDisliked has the radio of a music video, which
// is its song's, leave out the songs the user dislikes too.
func TestRadioOfAVideoWithoutDisliked(t *testing.T) {
	c := newFakeMusic(t).catalog(t)
	all, err := c.Radio(t.Context(), instantCrushVideo)
	if err != nil || len(all) < 2 {
		t.Fatalf("the radio: %v, %v", trackIDs(all), err)
	}
	if _, err := c.a.library.SetDisliked(all[0], true); err != nil {
		t.Fatal(err)
	}
	got, err := c.Radio(t.Context(), instantCrushVideo)
	if want := trackIDs(all[1:]); err != nil || !slices.Equal(trackIDs(got), want) {
		t.Errorf("found %v, not %v, %v", trackIDs(got), want, err)
	}
}

// TestReloadSettingsFollowsSkipDisliked has an edit of settings.json that
// turns skipping off let the queue's disliked songs play again, and one
// that turns it on pass over them again, as the switch does.
func TestReloadSettingsFollowsSkipDisliked(t *testing.T) {
	a := fileApp(t, `{}`)
	var err error
	if a.queue, err = store.Open(filepath.Join(a.opts.DataDir, "queue.json"), player.Saved{}); err != nil {
		t.Fatal(err)
	}
	disliked := media.Track{ID: "yt:b", Source: media.SourceYouTube, Title: "B"}
	if _, err := a.library.SetDisliked(disliked, true); err != nil {
		t.Fatal(err)
	}
	stream := &quietStream{}
	p := player.New(offlineServices{playerServices{a}}, stream, connectDevice{a}, player.Saved{})
	stream.p.Store(p)
	a.goPlayer.Store(p)
	t.Cleanup(p.Close)
	p.Enqueue([]media.Track{{ID: "yt:a", Source: media.SourceYouTube, Title: "A"}, disliked})
	skip := func() string {
		q := p.State().Queue
		if len(q) != 2 {
			return "no queue"
		}
		return q[1].Skip
	}
	until(t, "B skipped", func() bool { return skip() == player.SkipDisliked })
	writeSettings(t, a.settings.Path(), `{"skipDisliked": false}`)
	a.ReloadSettings()
	until(t, "B let off", func() bool { return skip() == "" })
	writeSettings(t, a.settings.Path(), `{"skipDisliked": true}`)
	a.ReloadSettings()
	until(t, "B skipped again", func() bool { return skip() == player.SkipDisliked })
}

// TestGlobalDislike dislikes what plays from outside the window, as a
// global shortcut does: the library keeps it, and the player moves on from
// it at once, while the settings skip disliked songs.
func TestGlobalDislike(t *testing.T) {
	a := newQuietApp(t, nil)
	p := a.GoPlayer()
	plays := func(id string) func(player.State) bool {
		return func(s player.State) bool {
			cur, ok := s.Current()
			return ok && cur.Track.ID == id && s.Playing
		}
	}
	p.PlayTracks([]media.Track{
		{ID: "yt:a", Source: media.SourceYouTube, Title: "A"},
		{ID: "yt:b", Source: media.SourceYouTube, Title: "B"},
	}, 0)
	waitPlayer(t, p, "a playing", plays("yt:a"))
	a.globalCommand("player.dislike")
	waitPlayer(t, p, "b playing", plays("yt:b"))
	until(t, "a disliked", func() bool { return a.library.IsDisliked("yt:a") })
	if q := p.State().Queue; q[0].Skip != player.SkipDisliked {
		t.Errorf("the queue doesn't pass over a: %+v", q[0])
	}
}

// TestGlobalDislikeTakenBack takes the dislike of what plays back at a
// second press, which plays on while the settings don't skip disliked
// songs.
func TestGlobalDislikeTakenBack(t *testing.T) {
	a := newQuietApp(t, nil)
	st := a.settings.Get()
	st.SkipDisliked = false
	if err := a.settings.Set(st); err != nil {
		t.Fatal(err)
	}
	p := a.GoPlayer()
	p.SettingsChanged(st)
	p.PlayTracks([]media.Track{{ID: "yt:a", Source: media.SourceYouTube, Title: "A"}}, 0)
	waitPlayer(t, p, "a playing", func(s player.State) bool { return s.Playing })
	a.command(PlayerCommand{Command: CommandDislike})
	until(t, "a disliked", func() bool { return a.library.IsDisliked("yt:a") })
	a.command(PlayerCommand{Command: CommandDislike})
	until(t, "the dislike taken back", func() bool { return !a.library.IsDisliked("yt:a") })
	if s := p.State(); !s.Playing || s.Index != 0 || len(a.library.Disliked()) != 0 {
		t.Errorf("playing %v at %d, disliked %v", s.Playing, s.Index, a.library.Disliked())
	}
}

// TestDislikedNote tells what a dislike from outside the window did.
func TestDislikedNote(t *testing.T) {
	track := media.Track{Title: "One More Time", Artists: []media.Ref{{Name: "Daft Punk"}}}
	if title, body := dislikedNote(track, true); title != "Disliked" || body != "One More Time — Daft Punk" {
		t.Errorf("disliked: %q, %q", title, body)
	}
	if title, body := dislikedNote(media.Track{Title: "Intro"}, false); title != "Removed the dislike" || body != "Intro" {
		t.Errorf("no longer disliked: %q, %q", title, body)
	}
}
