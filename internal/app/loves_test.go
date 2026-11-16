package app

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"github.com/fzn0x/mixmusic/internal/lastfm"
	"github.com/fzn0x/mixmusic/internal/library"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/scrobble"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// lovesApp is an App with a library, connected to Last.fm and to the
// ListenBrainz f, whose Send likes as loves are on as the settings say.
// It returns what Last.fm heard.
func lovesApp(t *testing.T, f *fakeBrainz, lastfmLoves, brainzLoves bool) (*App, func() []string) {
	t.Helper()
	a, acc, _ := brainzApp(t)
	a.settings = testSettings(t, func(s *settings.Settings) {
		s.LastfmLoves, s.ListenBrainzLoves = lastfmLoves, brainzLoves
	})
	lib, err := library.Open(filepath.Join(t.TempDir(), "library.json"))
	if err != nil {
		t.Fatal(err)
	}
	a.library = lib
	a.listenbrainz.validate = nil
	if _, err := acc.ListenBrainzLogin(context.Background(), "good", f.srv.URL); err != nil {
		t.Fatal(err)
	}
	a.listenbrainz.conn.client.Gap = 0

	var mu sync.Mutex
	var heard []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		mu.Lock()
		heard = append(heard, r.PostForm.Get("method")+" "+r.PostForm.Get("sk")+" "+r.PostForm.Get("artist")+" "+r.PostForm.Get("track"))
		mu.Unlock()
		io.WriteString(w, `{}`)
	}))
	t.Cleanup(srv.Close)
	c := lastfmCredentials{APIKey: "key", Secret: "secret", Session: lastfm.Session{Name: "ann", Key: "sk"}}
	a.lastfmStore = lastfmStore{testSecret(t, newKeeper(), nil)}
	if err := a.lastfmStore.Save(c); err != nil {
		t.Fatal(err)
	}
	a.startScrobbling(c)
	a.lastfm.client.BaseURL = srv.URL
	return a, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(heard)
	}
}

func (f *fakeBrainz) loved() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.loves)
}

// TestLikesAsLoves sends a like to Last.fm as a love and to ListenBrainz
// as feedback on the recording a play of it got, or the one the lookup
// finds, and an unlike as the love taken back.
func TestLikesAsLoves(t *testing.T) {
	f := newFakeBrainz(t)
	a, lastfmHeard := lovesApp(t, f, true, true)
	ctx := context.Background()
	col := &Collection{a}
	played := media.Track{ID: "yt:p", Source: media.SourceYouTube, Title: "Played", Artists: []media.Ref{{Name: "Ann"}}, DurationMs: 180000}
	a.listenbrainz.conn.recordings.keep(scrobble.Track{ID: "yt:p"}, "msid-p")
	known := media.Track{ID: "yt:k", Source: media.SourceYouTube, Title: "Known", Artists: []media.Ref{{Name: "Ann"}}, DurationMs: 180000}
	unknown := media.Track{ID: "yt:u", Source: media.SourceYouTube, Title: "Unknown", Artists: []media.Ref{{Name: "Ann"}}, DurationMs: 180000}
	episode := media.Track{ID: "yt:e", Source: media.SourceYouTube, Title: "Episode", Artists: []media.Ref{{Name: "Show"}}, Episode: &media.Episode{}}
	for _, step := range []struct {
		track media.Track
		liked bool
	}{{played, true}, {known, true}, {unknown, true}, {episode, true}, {played, false}} {
		if err := col.SetLiked(ctx, step.track, step.liked); err != nil {
			t.Fatal(err)
		}
	}
	if want := []string{"msid-p 1", "mbid-known 1", "msid-p 0"}; !slices.Equal(f.loved(), want) {
		t.Errorf("ListenBrainz heard %q, want %q", f.loved(), want)
	}
	want := []string{"track.love sk Ann Played", "track.love sk Ann Known", "track.love sk Ann Unknown", "track.unlove sk Ann Played"}
	if got := lastfmHeard(); !slices.Equal(got, want) {
		t.Errorf("Last.fm heard %q, want %q", got, want)
	}
}

// TestLovesOff sends no love while the switches are off, or while a
// service refuses the connection.
func TestLovesOff(t *testing.T) {
	f := newFakeBrainz(t)
	a, lastfmHeard := lovesApp(t, f, false, false)
	song := media.Track{ID: "yt:k", Source: media.SourceYouTube, Title: "Known", Artists: []media.Ref{{Name: "Ann"}}, DurationMs: 180000}
	a.sendLoves(context.Background(), song, true)
	if len(f.loved()) != 0 || len(lastfmHeard()) != 0 {
		t.Errorf("heard %q and %q", f.loved(), lastfmHeard())
	}
	a, lastfmHeard = lovesApp(t, f, true, true)
	a.mu.Lock()
	a.lastfm.expired, a.listenbrainz.conn.expired = true, true
	a.mu.Unlock()
	a.sendLoves(context.Background(), song, true)
	if len(f.loved()) != 0 || len(lastfmHeard()) != 0 {
		t.Errorf("expired: heard %q and %q", f.loved(), lastfmHeard())
	}
}
