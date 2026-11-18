package librefm_test

import (
	"context"
	"errors"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/lastfm"
	"github.com/fzn0x/mixmusic/internal/librefm"
	"github.com/fzn0x/mixmusic/internal/librefm/librefmtest"
	"github.com/fzn0x/mixmusic/internal/scrobble"
)

func TestParseServer(t *testing.T) {
	for _, c := range []struct {
		addr, want string
		plain      bool
		err        error
	}{
		{"", librefm.DefaultServer, false, nil},
		{"  libre.fm ", librefm.DefaultServer, false, nil},
		{"https://www.libre.fm/", librefm.DefaultServer, false, nil},
		{"https://libre.fm/2.0/", librefm.DefaultServer, false, nil},
		{"https://libre.fm/api/auth/", librefm.DefaultServer, false, nil},
		{"HTTPS://GNUFM.Example.com/fm/", "https://gnufm.example.com/fm", false, nil},
		{"http://192.168.1.20:8080/2.0", "http://192.168.1.20:8080", true, nil},
		{"ftp://gnufm.example.com", "", false, librefm.ErrNotServer},
		{"https://ann:secret@gnufm.example.com", "", false, librefm.ErrNotServer},
		{"https://gnufm.example.com/?x=1", "", false, librefm.ErrNotServer},
		{"https://", "", false, librefm.ErrNotServer},
	} {
		got, plain, err := librefm.ParseServer(c.addr)
		if got != c.want || plain != c.plain || !errors.Is(err, c.err) {
			t.Errorf("ParseServer(%q) = %q, %v, %v; want %q, %v, %v", c.addr, got, plain, err, c.want, c.plain, c.err)
		}
	}
}

// TestNew calls the server's API at /2.0/ and has the user approve at
// /api/auth/, with MixMusic's key or the user's own.
func TestNew(t *testing.T) {
	c := librefm.New(librefm.DefaultServer, "", "", "MixMusic/1.2.3")
	if c.BaseURL != "https://libre.fm/2.0/" || c.APIKey != librefm.APIKey || c.Secret != librefm.Secret || c.UserAgent != "MixMusic/1.2.3" {
		t.Errorf("Libre.fm's client = %+v", c)
	}
	if got := c.AuthURL("tok"); got != "https://libre.fm/api/auth/?api_key="+librefm.APIKey+"&token=tok" {
		t.Errorf("AuthURL = %s", got)
	}
	if len(librefm.APIKey) != 32 || len(librefm.Secret) != 32 {
		t.Error("GNU FM takes a key of 32 characters")
	}
	c = librefm.New("https://gnufm.example.com/fm", "mine", "my secret", "")
	if c.BaseURL != "https://gnufm.example.com/fm/2.0/" || c.APIKey != "mine" || c.Secret != "my secret" {
		t.Errorf("another server's client = %+v", c)
	}
}

// TestGNUFM connects to a GNU FM server, as desktop authentication does,
// and sends it what plays, the scrobbles and the loves.
func TestGNUFM(t *testing.T) {
	srv := librefmtest.New(t)
	c := librefm.New(srv.URL, "", "", "MixMusic/1.2.3")
	ctx := context.Background()
	token, err := c.Token(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Session(ctx, token); !errors.Is(err, lastfm.ErrNotAuthorized) {
		t.Fatalf("before approval: %v", err)
	}
	page, err := url.Parse(c.AuthURL(token))
	if err != nil || !strings.HasPrefix(page.String(), srv.URL+"/api/auth/?") {
		t.Fatalf("AuthURL = %v, %v", page, err)
	}
	if !srv.Approve(page.Query().Get("token"), "ann") {
		t.Fatal("the page's token isn't the server's")
	}
	session, err := c.Session(ctx, token)
	if err != nil || session.Name != "ann" || len(session.Key) != 32 {
		t.Fatalf("session = %+v, %v", session, err)
	}

	song := lastfm.Track{Artist: "Ann", Title: "Song", Album: "Album", Duration: 3 * time.Minute}
	if err := c.NowPlaying(ctx, session.Key, song); err != nil {
		t.Fatal(err)
	}
	plays := []lastfm.Play{{Track: song, Start: time.Unix(1_790_000_000, 0)}, {Track: lastfm.Track{Artist: "Bo", Title: "Two"}, Start: time.Unix(1_790_000_300, 0)}}
	if err := c.Scrobble(ctx, session.Key, plays); err != nil {
		t.Fatal(err)
	}
	if err := c.Love(ctx, session.Key, song, true); err != nil {
		t.Fatal(err)
	}
	if err := c.Love(ctx, session.Key, song, false); err != nil {
		t.Fatal(err)
	}
	want := []string{"track.updatenowplaying ann Ann Song", "track.scrobble ann Ann Song", "track.scrobble ann Bo Two",
		"track.love ann Ann Song", "track.unlove ann Ann Song"}
	if got := srv.Heard(); !slices.Equal(got, want) {
		t.Errorf("heard %q, want %q", got, want)
	}
	for _, key := range srv.Keys() {
		if key != librefm.APIKey {
			t.Errorf("called with the key %q", key)
		}
	}

	// A call the server fails on its side keeps the plays; a session it
	// doesn't know refuses the key.
	srv.Fail(16)
	if err := c.Scrobble(ctx, session.Key, plays); !lastfm.IsTemporary(err) {
		t.Errorf("error 16: %v", err)
	}
	srv.Fail(0)
	srv.Revoke("ann")
	if err := c.Scrobble(ctx, session.Key, plays); !errors.Is(err, scrobble.ErrKeyRefused) {
		t.Errorf("a revoked session: %v", err)
	}
}
