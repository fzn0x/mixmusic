package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/lastfm"
	"github.com/fzn0x/mixmusic/internal/librefm"
	"github.com/fzn0x/mixmusic/internal/librefm/librefmtest"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/scrobble"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/spotify"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

// gnufmApp is an App whose Libre.fm connection is kept in a store of the
// test's, whose browser approves each token on the server it opens as the
// user whose name *user holds, and which asks for the session at once.
// It returns the pages the browser opened.
func gnufmApp(t *testing.T, srv *librefmtest.Server, user *string, fn ...func(*settings.Settings)) (*App, *Accounts, func() []string) {
	t.Helper()
	auth := spotify.NewAuth("")
	a := &App{opts: Options{DataDir: t.TempDir(), Version: "1.2.3"}, settings: testSettings(t, fn...), auth: auth, spotify: spotify.NewClient(auth), youtube: youtube.New()}
	a.librefm.store = librefmStore{&secretStore{key: librefmSecret, plain: filepath.Join(t.TempDir(), "librefm.json"), keep: newKeeper()}}
	a.librefm.resolve = func(context.Context, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("93.184.215.14")}, nil
	}
	a.librefm.poll = time.Millisecond
	var mu sync.Mutex
	var opened []string
	a.librefm.open = func(page string) error {
		mu.Lock()
		opened = append(opened, page)
		mu.Unlock()
		u, err := url.Parse(page)
		if err != nil {
			return err
		}
		if srv != nil {
			srv.Approve(u.Query().Get("token"), *user)
		}
		return nil
	}
	t.Cleanup(a.stopScrobbling)
	return a, &Accounts{a: a}, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(opened)
	}
}

// TestLibrefmConnect connects to a GNU FM server with MixMusic's own key:
// the user approves on the server's page, and the server stays with the
// session.
func TestLibrefmConnect(t *testing.T) {
	srv := librefmtest.New(t)
	user := "ann"
	a, acc, opened := gnufmApp(t, srv, &user)
	ctx := context.Background()
	if _, err := acc.LibrefmLogin(ctx, srv.URL+"/2.0/", false, "", ""); err != nil {
		t.Fatal(err)
	}
	if got := a.librefmAccount(); !got.Connected || got.Name != "ann" || got.Server != srv.URL || !got.Plain {
		t.Errorf("account = %+v", got)
	}
	c, err := a.librefm.store.Load()
	if err != nil || c.Server != srv.URL || c.APIKey != "" || c.Secret != "" || c.Session.Name != "ann" || c.Session.Key == "" {
		t.Errorf("kept %+v, %v", c, err)
	}
	if pages := opened(); len(pages) != 1 || !strings.HasPrefix(pages[0], srv.URL+"/api/auth/?api_key="+librefm.APIKey+"&token=") {
		t.Errorf("opened %q", pages)
	}
	for _, key := range srv.Keys() {
		if key != librefm.APIKey {
			t.Errorf("called with the key %q", key)
		}
	}
	if got := a.knownAccountsStatus().Librefm; got.Name != "ann" {
		t.Errorf("status = %+v", got)
	}
}

// TestLibrefmOwnAccount signs calls with the user's API account, for a
// server that wants one, and keeps it with the session.
func TestLibrefmOwnAccount(t *testing.T) {
	srv := librefmtest.New(t)
	user := "ann"
	a, acc, _ := gnufmApp(t, srv, &user)
	ctx := context.Background()
	if _, err := acc.LibrefmLogin(ctx, srv.URL, true, " my-key ", ""); err == nil || err.Error() != "paste the API key and the shared secret of your API account on the server" {
		t.Errorf("no secret: %v", err)
	}
	if a.librefmAccount().Connected || len(srv.Keys()) != 0 {
		t.Fatal("connected without a secret")
	}
	if _, err := acc.LibrefmLogin(ctx, srv.URL, true, " my-key ", " my-secret\n"); err != nil {
		t.Fatal(err)
	}
	if c, _ := a.librefm.store.Load(); c.APIKey != "my-key" || c.Secret != "my-secret" {
		t.Errorf("kept %+v", c)
	}
	for _, key := range srv.Keys() {
		if key != "my-key" {
			t.Errorf("called with the key %q", key)
		}
	}
}

// TestLibrefmServers takes Libre.fm for no address, a server's https
// address anywhere, and its plain http address on the local network
// alone.
func TestLibrefmServers(t *testing.T) {
	a, _, _ := gnufmApp(t, nil, nil)
	ctx := context.Background()
	for _, c := range []struct {
		server, want string
		err          error
	}{
		{"", "", nil},
		{" libre.fm/ ", "", nil},
		{"gnufm.example.com", "https://gnufm.example.com", nil},
		{"http://192.168.1.20:8080/2.0/", "http://192.168.1.20:8080", nil},
		{"http://nas/fm", "http://nas/fm", nil},
		{"http://gnufm.example.com", "", errPlainGNUFM},
		{"ftp://gnufm.example.com", "", librefm.ErrNotServer},
	} {
		got, err := a.librefmCheck(ctx, c.server, false, "", "")
		if !errors.Is(err, c.err) || got.Server != c.want {
			t.Errorf("%q: %+v, %v", c.server, got, err)
		}
	}
}

// TestLibrefmNotGNUFM says that the server couldn't be asked when it
// isn't one of GNU FM, and keeps nothing.
func TestLibrefmNotGNUFM(t *testing.T) {
	site := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(site.Close)
	user := "ann"
	a, acc, opened := gnufmApp(t, nil, &user)
	_, err := acc.LibrefmLogin(context.Background(), site.URL, false, "", "")
	host := strings.TrimPrefix(site.URL, "http://")
	if err == nil || err.Error() != "MixMusic couldn't ask "+host+" for a sign-in: check its address and your connection (librefm: HTTP 404)" {
		t.Errorf("err = %v", err)
	}
	if a.librefmAccount().Connected || len(opened()) != 0 {
		t.Error("connected, or opened the browser")
	}
	if c, err := a.librefm.store.Load(); err == nil && c.Session.Key != "" {
		t.Errorf("kept %+v", c)
	}
}

// TestLibrefmSendsAndKeeps sends the scrobbles that wait, keeps them while
// the server fails on its side or refuses the session, and sends them
// once the user connects again.
func TestLibrefmSendsAndKeeps(t *testing.T) {
	srv := librefmtest.New(t)
	user := "ann"
	a, acc, _ := gnufmApp(t, srv, &user)
	ctx := context.Background()
	if _, err := acc.LibrefmLogin(ctx, srv.URL, false, "", ""); err != nil {
		t.Fatal(err)
	}
	l := a.librefm.conn
	play := func(title string, at int64) scrobble.Play {
		return scrobble.Play{Track: scrobble.Track{Artist: "A", Title: title, Duration: time.Minute}, Start: time.Unix(at, 0)}
	}
	scrobbled := func() []string {
		var got []string
		for _, h := range srv.Heard() {
			if t, ok := strings.CutPrefix(h, "track.scrobble ann A "); ok {
				got = append(got, t)
			}
		}
		return got
	}
	l.scrobbler.SetQueue([]scrobble.Play{play("One", 1_790_000_000)})
	a.sendScrobbles(&l.scrobbling)
	eventually(t, "the scrobble going", func() bool { return len(scrobbled()) == 1 })

	// The server fails on its side: the scrobbles wait, and go after.
	srv.Fail(16)
	l.scrobbler.SetQueue([]scrobble.Play{play("Two", 1_790_000_100)})
	if err := l.scrobbler.Send(ctx); !lastfm.IsTemporary(err) {
		t.Errorf("a failure on the server's side: %v", err)
	}
	if got := a.librefmAccount(); got.Waiting != 1 || got.Expired {
		t.Errorf("account = %+v", got)
	}
	srv.Fail(0)
	l.scrobbler.Retry()
	eventually(t, "the waiting scrobble going", func() bool { return len(scrobbled()) == 2 })

	// The user revoked the session: the next scrobbles wait for a new one.
	srv.Revoke("ann")
	l.scrobbler.SetQueue([]scrobble.Play{play("Three", 1_790_000_200), play("Four", 1_790_000_300)})
	eventually(t, "the session refused", func() bool { return a.librefmAccount().Expired })
	if got := a.librefmAccount(); got.Waiting != 2 {
		t.Errorf("account = %+v", got)
	}
	if err := acc.LibrefmReconnect(ctx); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the waiting scrobbles going", func() bool { return len(scrobbled()) == 4 })
	if got := scrobbled(); !slices.Equal(got, []string{"One", "Two", "Three", "Four"}) {
		t.Errorf("scrobbled %q", got)
	}
	if got := a.librefmAccount(); got.Expired || got.Waiting != 0 || got.Name != "ann" || got.Server != srv.URL {
		t.Errorf("account = %+v", got)
	}
	if c, _ := a.librefm.store.Load(); c.Server != srv.URL || c.Session.Key != a.librefm.conn.creds.Session.Key {
		t.Errorf("kept %+v", c)
	}
}

// TestLibrefmAnotherUser holds the connection of another user while
// scrobbles wait, until the user says to send them there or drop them.
func TestLibrefmAnotherUser(t *testing.T) {
	for _, send := range []bool{true, false} {
		srv := librefmtest.New(t)
		user := "ann"
		a, acc, _ := gnufmApp(t, srv, &user)
		ctx := context.Background()
		if _, err := acc.LibrefmLogin(ctx, srv.URL, false, "", ""); err != nil {
			t.Fatal(err)
		}
		l := a.librefm.conn
		l.scrobbler.SetQueue(make([]scrobble.Play, 2))
		a.mu.Lock()
		l.expired = true
		a.mu.Unlock()
		user = "bo"
		if err := acc.LibrefmReconnect(ctx); err != nil {
			t.Fatal(err)
		}
		if got := a.librefmAccount(); got.Other != "bo" || got.Name != "ann" || got.Waiting != 2 {
			t.Fatalf("account = %+v", got)
		}
		if c, _ := a.librefm.store.Load(); c.Session.Name != "ann" {
			t.Errorf("kept %+v before the choice", c)
		}
		if err := acc.LibrefmResolve(ctx, send); err != nil {
			t.Fatal(err)
		}
		want := 0
		if send {
			want = 2
		}
		if got := a.librefmAccount(); got.Name != "bo" || got.Other != "" || got.Expired || got.Waiting != want {
			t.Errorf("send %v: account %+v", send, got)
		}
		if c, _ := a.librefm.store.Load(); c.Session.Name != "bo" || c.Server != srv.URL {
			t.Errorf("send %v: kept %+v", send, c)
		}
	}
}

func TestLibrefmLogout(t *testing.T) {
	srv := librefmtest.New(t)
	user := "ann"
	a, acc, _ := gnufmApp(t, srv, &user)
	ctx := context.Background()
	if _, err := acc.LibrefmLogin(ctx, srv.URL, false, "", ""); err != nil {
		t.Fatal(err)
	}
	a.librefm.conn.scrobbler.Discard() // which saves the queue's file
	queue := filepath.Join(a.opts.DataDir, librefmQueueFile)
	if _, err := os.Stat(queue); err != nil {
		t.Fatal(err)
	}
	if err := acc.LibrefmLogout(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(queue); !os.IsNotExist(err) {
		t.Errorf("the queue: %v", err)
	}
	if a.librefmAccount().Connected {
		t.Error("still connected")
	}
	if c, err := a.librefm.store.Load(); err == nil && c.Session.Key != "" {
		t.Errorf("kept %+v", c)
	}
	if err := acc.LibrefmReconnect(ctx); err == nil || err.Error() != "connect to Libre.fm first" {
		t.Errorf("connecting again: %v", err)
	}
}

// TestLibrefmFeed tells the server what plays while Scrobble to Libre.fm
// is on, and nothing while it's off.
func TestLibrefmFeed(t *testing.T) {
	song := &media.Track{ID: "yt:s", Source: media.SourceYouTube, Title: "Song", Artists: []media.Ref{{Name: "Ann"}}, DurationMs: 180000}
	for _, on := range []bool{true, false} {
		srv := librefmtest.New(t)
		user := "ann"
		a, acc, _ := gnufmApp(t, srv, &user, func(s *settings.Settings) { s.LibrefmScrobbling = on })
		if _, err := acc.LibrefmLogin(context.Background(), srv.URL, false, "", ""); err != nil {
			t.Fatal(err)
		}
		a.scrobble(NowPlaying{Track: song, Playing: true})
		if on {
			eventually(t, "the song announced", func() bool { return slices.Contains(srv.Heard(), "track.updatenowplaying ann Ann Song") })
			continue
		}
		time.Sleep(50 * time.Millisecond)
		if got := srv.Heard(); len(got) != 0 {
			t.Errorf("off: heard %q", got)
		}
	}
}

// TestLibrefmLoves sends a like as a love and an unlike as an unlove while
// Send likes as loves is on, and nothing while it's off or the server
// refuses the session.
func TestLibrefmLoves(t *testing.T) {
	song := media.Track{ID: "yt:s", Source: media.SourceYouTube, Title: "Song", Artists: []media.Ref{{Name: "Ann"}}, DurationMs: 180000}
	episode := media.Track{ID: "yt:e", Source: media.SourceYouTube, Title: "Episode", Artists: []media.Ref{{Name: "Show"}}, Episode: &media.Episode{}}
	for _, on := range []bool{true, false} {
		srv := librefmtest.New(t)
		user := "ann"
		a, acc, _ := gnufmApp(t, srv, &user, func(s *settings.Settings) { s.LibrefmLoves = on })
		ctx := context.Background()
		if _, err := acc.LibrefmLogin(ctx, srv.URL, false, "", ""); err != nil {
			t.Fatal(err)
		}
		a.sendLoves(ctx, song, true)
		a.sendLoves(ctx, episode, true)
		a.sendLoves(ctx, song, false)
		var want []string
		if on {
			want = []string{"track.love ann Ann Song", "track.unlove ann Ann Song"}
		}
		if got := srv.Heard(); !slices.Equal(got, want) {
			t.Errorf("on %v: heard %q, want %q", on, got, want)
		}
		if !on {
			continue
		}
		a.mu.Lock()
		a.librefm.conn.expired = true
		a.mu.Unlock()
		a.sendLoves(ctx, song, true)
		if got := srv.Heard(); len(got) != 2 {
			t.Errorf("refused: heard %q", got)
		}
	}
}
