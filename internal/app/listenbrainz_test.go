package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/listenbrainz"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/scrobble"
	"github.com/fzn0x/mixmusic/internal/spotify"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

const lbToken = "0f1e2d3c-4b5a-6978-8796-a5b4c3d2e1f0"

// brainzApp is an App whose ListenBrainz connection is kept in a store of
// the test's, and whose tokens validate asks of: by default, every token
// is ann's.
func brainzApp(t *testing.T) (*App, *Accounts, *[]string) {
	t.Helper()
	auth := spotify.NewAuth("")
	a := &App{opts: Options{DataDir: t.TempDir(), Version: "1.2.3"}, settings: testSettings(t), auth: auth, spotify: spotify.NewClient(auth), youtube: youtube.New()}
	keep := newKeeper()
	a.listenbrainz.store = listenbrainzStore{&secretStore{key: listenbrainzSecret, plain: filepath.Join(t.TempDir(), "listenbrainz.json"), keep: keep}}
	var asked []string
	a.listenbrainz.validate = func(_ context.Context, server, token string) (string, error) {
		asked = append(asked, server+" "+token)
		if token == "refused" || token == "0f1e2d3c-4b5a-6978-8796-000000000000" {
			return "", listenbrainz.ErrTokenRefused
		}
		return "ann", nil
	}
	a.listenbrainz.resolve = func(context.Context, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("93.184.215.14")}, nil
	}
	t.Cleanup(a.stopScrobbling)
	return a, &Accounts{a: a}, &asked
}

func TestListenBrainzConnect(t *testing.T) {
	a, acc, asked := brainzApp(t)
	ctx := context.Background()
	if _, err := acc.ListenBrainzLogin(ctx, " "+lbToken+"\n", ""); err != nil {
		t.Fatal(err)
	}
	if got := a.listenbrainzAccount(); !got.Connected || got.Name != "ann" || got.Server != "" || got.Plain {
		t.Errorf("account = %+v", got)
	}
	if c, err := a.listenbrainz.store.Load(); err != nil || c != (listenbrainzCredentials{Token: lbToken, Name: "ann"}) {
		t.Errorf("kept %+v, %v", c, err)
	}
	if want := listenbrainz.DefaultServer + " " + lbToken; len(*asked) != 1 || (*asked)[0] != want {
		t.Errorf("asked %q", *asked)
	}
	if got := a.knownAccountsStatus().ListenBrainz; got.Name != "ann" {
		t.Errorf("status = %+v", got)
	}
}

// TestListenBrainzRefusesTokens keeps nothing of a token the server
// refuses, or that isn't one of ListenBrainz's own, which it doesn't ask
// about.
func TestListenBrainzRefusesTokens(t *testing.T) {
	for _, token := range []string{"refused", "0f1e2d3c-4b5a-6978-8796-000000000000", "my password", ""} {
		a, acc, asked := brainzApp(t)
		if _, err := acc.ListenBrainzLogin(context.Background(), token, ""); !errors.Is(err, errListenBrainzToken) || err.Error() != "ListenBrainz doesn't accept this token" {
			t.Errorf("%q: err = %v", token, err)
		}
		if c, err := a.listenbrainz.store.Load(); err == nil && c.Token != "" {
			t.Errorf("%q: kept %+v", token, c)
		}
		if a.listenbrainzAccount().Connected {
			t.Errorf("%q: connected", token)
		}
		if strings.Contains(token, "-") != (len(*asked) == 1) {
			t.Errorf("%q: asked %q", token, *asked)
		}
	}
}

// TestListenBrainzServers takes a server of the user's: over https
// anywhere, over plain http on the local network alone, with any token.
func TestListenBrainzServers(t *testing.T) {
	for _, c := range []struct {
		server, want string
		plain        bool
		err          error
	}{
		{"https://music.example.com/apis/listenbrainz", "https://music.example.com/apis/listenbrainz", false, nil},
		{"http://192.168.1.20:42010/apis/listenbrainz/", "http://192.168.1.20:42010/apis/listenbrainz", true, nil},
		{"http://nas:42010/apis/listenbrainz", "http://nas:42010/apis/listenbrainz", true, nil},
		{"http://music.example.com/apis/listenbrainz", "", false, errPlainRemote},
		{"ftp://music.example.com", "", false, listenbrainz.ErrNotServer},
	} {
		a, acc, _ := brainzApp(t)
		_, err := acc.ListenBrainzLogin(context.Background(), "maloja-key", c.server)
		if !errors.Is(err, c.err) {
			t.Errorf("%s: err = %v", c.server, err)
			continue
		}
		got := a.listenbrainzAccount()
		if c.err != nil {
			if got.Connected {
				t.Errorf("%s: connected", c.server)
			}
			continue
		}
		if got.Server != c.want || got.Plain != c.plain {
			t.Errorf("%s: account %+v", c.server, got)
		}
	}
}

// fakeBrainz is a ListenBrainz server that takes the token "good" alone,
// and keeps the listens it took.
type fakeBrainz struct {
	mu      sync.Mutex
	srv     *httptest.Server
	token   string
	listens []string
	loves   []string
	types   []string
}

func newFakeBrainz(t *testing.T) *fakeBrainz {
	t.Helper()
	f := &fakeBrainz{token: "good"}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if r.Header.Get("Authorization") != "Token "+f.token {
			if r.URL.Path == "/1/validate-token" {
				io.WriteString(w, `{"code": 200, "message": "Token invalid.", "valid": false}`)
				return
			}
			w.WriteHeader(http.StatusUnauthorized)
			io.WriteString(w, `{"code": 401, "error": "Invalid authorization token."}`)
			return
		}
		var body struct {
			ListenType string `json:"listen_type"`
			Payload    []struct {
				Track struct {
					Title string `json:"track_name"`
				} `json:"track_metadata"`
			} `json:"payload"`
			MSID  string `json:"recording_msid"`
			MBID  string `json:"recording_mbid"`
			Score int    `json:"score"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		switch r.URL.Path {
		case "/1/validate-token":
			io.WriteString(w, `{"code": 200, "message": "Token valid.", "user_name": "ann", "valid": true}`)
		case "/1/submit-listens":
			f.types = append(f.types, body.ListenType)
			if body.ListenType == "playing_now" {
				io.WriteString(w, `{"recording_msid": "msid-`+body.Payload[0].Track.Title+`", "status": "ok"}`)
				return
			}
			for _, p := range body.Payload {
				f.listens = append(f.listens, p.Track.Title)
			}
			io.WriteString(w, `{"status": "ok"}`)
		case "/1/feedback/recording-feedback":
			f.loves = append(f.loves, body.MSID+body.MBID+" "+string(rune('0'+body.Score)))
			io.WriteString(w, `{"status": "ok"}`)
		case "/1/metadata/lookup/":
			if r.URL.Query().Get("recording_name") == "Known" {
				io.WriteString(w, `{"recording_mbid": "mbid-known"}`)
				return
			}
			io.WriteString(w, `{}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeBrainz) took() ([]string, []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.listens...), append([]string(nil), f.types...)
}

// TestListenBrainzSendsAndKeeps sends the listens that wait to a server of
// the user's, keeps them while it refuses the token, and sends them with
// a new one.
func TestListenBrainzSendsAndKeeps(t *testing.T) {
	f := newFakeBrainz(t)
	a, acc, _ := brainzApp(t)
	a.listenbrainz.validate = nil // the fake server answers
	ctx := context.Background()
	if _, err := acc.ListenBrainzLogin(ctx, "good", f.srv.URL); err != nil {
		t.Fatal(err)
	}
	l := a.listenbrainz.conn
	l.scrobbler.SetQueue([]scrobble.Play{{Track: scrobble.Track{Artist: "A", Title: "One"}, Start: time.Unix(1_790_000_000, 0)}})
	a.sendScrobbles(&l.scrobbling)
	eventually(t, "the listen going", func() bool { got, _ := f.took(); return len(got) == 1 })
	if _, types := f.took(); len(types) != 1 || types[0] != "single" {
		t.Errorf("submitted %q", types)
	}

	// The user made a new token on the server's site: the next listens wait.
	f.mu.Lock()
	f.token = "newer"
	f.mu.Unlock()
	l.scrobbler.SetQueue([]scrobble.Play{{Track: scrobble.Track{Artist: "A", Title: "Two"}, Start: time.Unix(1_790_000_100, 0)},
		{Track: scrobble.Track{Artist: "A", Title: "Three"}, Start: time.Unix(1_790_000_200, 0)}})
	eventually(t, "the token refused", func() bool { return a.listenbrainzAccount().Expired })
	if got := a.listenbrainzAccount(); got.Waiting != 2 {
		t.Errorf("account = %+v", got)
	}
	if err := acc.ListenBrainzReconnect(ctx, "wrong"); !errors.Is(err, errListenBrainzToken) {
		t.Errorf("a wrong token: %v", err)
	}
	if err := acc.ListenBrainzReconnect(ctx, "newer"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the waiting listens going", func() bool { got, _ := f.took(); return len(got) == 3 })
	if got, types := f.took(); got[1] != "Two" || got[2] != "Three" || types[1] != "import" {
		t.Errorf("took %q as %q", got, types)
	}
	if got := a.listenbrainzAccount(); got.Expired || got.Waiting != 0 || got.Name != "ann" {
		t.Errorf("account = %+v", got)
	}
	if c, _ := a.listenbrainz.store.Load(); c.Token != "newer" || c.Server != f.srv.URL {
		t.Errorf("kept %+v", c)
	}
}

// TestListenBrainzAnotherUser holds a token of another user while listens
// wait, until the user says to send them there or drop them.
func TestListenBrainzAnotherUser(t *testing.T) {
	for _, send := range []bool{true, false} {
		a, acc, _ := brainzApp(t)
		ctx := context.Background()
		if _, err := acc.ListenBrainzLogin(ctx, lbToken, ""); err != nil {
			t.Fatal(err)
		}
		l := a.listenbrainz.conn
		l.scrobbler.SetQueue(make([]scrobble.Play, 2))
		a.mu.Lock()
		l.expired = true
		a.mu.Unlock()
		a.listenbrainz.validate = func(context.Context, string, string) (string, error) { return "bo", nil }
		if err := acc.ListenBrainzReconnect(ctx, "1f1e2d3c-4b5a-6978-8796-a5b4c3d2e1f0"); err != nil {
			t.Fatal(err)
		}
		if got := a.listenbrainzAccount(); got.Other != "bo" || got.Name != "ann" || got.Waiting != 2 {
			t.Fatalf("account = %+v", got)
		}
		if c, _ := a.listenbrainz.store.Load(); c.Token != lbToken {
			t.Errorf("kept %+v before the choice", c)
		}
		if err := acc.ListenBrainzResolve(ctx, send); err != nil {
			t.Fatal(err)
		}
		want := 0
		if send {
			want = 2
		}
		if got := a.listenbrainzAccount(); got.Name != "bo" || got.Other != "" || got.Expired || got.Waiting != want {
			t.Errorf("send %v: account %+v", send, got)
		}
	}
}

func TestListenBrainzLogout(t *testing.T) {
	a, acc, _ := brainzApp(t)
	ctx := context.Background()
	if _, err := acc.ListenBrainzLogin(ctx, lbToken, ""); err != nil {
		t.Fatal(err)
	}
	l := a.listenbrainz.conn
	l.recordings.keep(scrobble.Track{ID: "yt:a"}, "msid")
	l.scrobbler.Discard() // which saves the queue's file
	if _, err := os.Stat(filepath.Join(a.opts.DataDir, listenbrainzRecordingsFile)); err != nil {
		t.Fatal(err)
	}
	if err := acc.ListenBrainzLogout(ctx); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{listenbrainzQueueFile, listenbrainzRecordingsFile} {
		if _, err := os.Stat(filepath.Join(a.opts.DataDir, name)); !os.IsNotExist(err) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if a.listenbrainzAccount().Connected {
		t.Error("still connected")
	}
	if c, err := a.listenbrainz.store.Load(); err == nil && c.Token != "" {
		t.Errorf("kept %+v", c)
	}
}

// TestScrobbleTrack tells the services where a track plays from, under
// Last.fm's rules of what counts.
func TestScrobbleTrack(t *testing.T) {
	art := []media.Ref{{Name: "Ann"}}
	for _, c := range []struct {
		name  string
		track media.Track
		want  scrobble.Track
	}{
		{"YouTube Music", media.Track{ID: "yt:abc", Source: media.SourceYouTube, Title: "Song", Artists: art, DurationMs: 180000},
			scrobble.Track{ID: "yt:abc", Artist: "Ann", Title: "Song", Duration: 3 * time.Minute, Service: "music.youtube.com", URL: "https://music.youtube.com/watch?v=abc"}},
		{"Spotify", media.Track{ID: "sp:xyz", Source: media.SourceSpotify, Title: "Song", Artists: art, DurationMs: 180000, ISRC: "USRC17607839"},
			scrobble.Track{ID: "sp:xyz", Artist: "Ann", Title: "Song", Duration: 3 * time.Minute, Service: "spotify.com", URL: "https://open.spotify.com/track/xyz",
				SpotifyURL: "https://open.spotify.com/track/xyz", ISRC: "USRC17607839"}},
		{"a file", media.Track{ID: "file:/music/song.flac", Source: media.SourceFile, Title: "Song", Artists: art, DurationMs: 180000},
			scrobble.Track{ID: "file:/music/song.flac", Artist: "Ann", Title: "Song", Duration: 3 * time.Minute}},
	} {
		tr := c.track
		if _, got := scrobbleTrack(NowPlaying{Track: &tr}); got != c.want {
			t.Errorf("%s: %+v\nwant %+v", c.name, got, c.want)
		}
	}
	if id, got := scrobbleTrack(NowPlaying{}); id != "" || got != (scrobble.Track{}) {
		t.Errorf("nothing: %q %+v", id, got)
	}
}

// TestRecordingBook keeps the latest recordings, in its file.
func TestRecordingBook(t *testing.T) {
	file := filepath.Join(t.TempDir(), listenbrainzRecordingsFile)
	b := openRecordings(file)
	b.keep(scrobble.Track{ID: "yt:a"}, "msid-a")
	b.keep(scrobble.Track{}, "no track")
	if got := openRecordings(file).msid("yt:a"); got != "msid-a" {
		t.Errorf("read back %q", got)
	}
	for i := range maxRecordings {
		b.ids[string(rune(0x1000+i))] = keptRecording{MSID: "x", At: int64(i) + 10}
	}
	b.ids["yt:a"] = keptRecording{MSID: "msid-a", At: 1}
	b.keep(scrobble.Track{ID: "yt:b"}, "msid-b")
	if len(b.ids) != maxRecordings || b.msid("yt:a") != "" || b.msid("yt:b") != "msid-b" {
		t.Errorf("%d kept, a %q, b %q", len(b.ids), b.msid("yt:a"), b.msid("yt:b"))
	}
}
