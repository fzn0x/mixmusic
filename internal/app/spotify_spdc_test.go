package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/builtin"
	"github.com/fzn0x/mixmusic/internal/browser"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/spotify"
)

func TestParseSPDC(t *testing.T) {
	cases := []struct {
		name                   string
		in                     string
		wantDC, wantKey, wantT string
	}{
		{
			name:   "raw token",
			in:     "  AQA1234567890abcdef  ",
			wantDC: "AQA1234567890abcdef",
		},
		{
			name:    "cookie header",
			in:      "Cookie: sp_t=t-val; sp_dc=dc-val-1234567890; sp_key=key-val",
			wantDC:  "dc-val-1234567890",
			wantKey: "key-val",
			wantT:   "t-val",
		},
		{
			name:   "netscape cookies.txt",
			in:     "# Netscape HTTP Cookie File\n.spotify.com\tTRUE\t/\tTRUE\t2534023007\tsp_dc\tdc-netscape\n.spotify.com\tTRUE\t/\tTRUE\t2534023007\tsp_key\tkey-netscape\n",
			wantDC: "dc-netscape", wantKey: "key-netscape",
		},
		{
			name:   "invalid short text",
			in:     "not a cookie",
			wantDC: "",
		},
	}
	for _, tc := range cases {
		dc, key, spt := parseSPDC(tc.in)
		if dc != tc.wantDC || key != tc.wantKey || spt != tc.wantT {
			t.Errorf("%s: got (%q, %q, %q), want (%q, %q, %q)", tc.name, dc, key, spt, tc.wantDC, tc.wantKey, tc.wantT)
		}
	}
}

func spdcTestApp(t *testing.T) (*App, *Accounts) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/nuances.json":
			w.Write([]byte(`[{"s":"GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ","v":61}]`))
		case "/server-time":
			w.Write([]byte(`{"serverTime":1795664820}`))
		case "/api/token":
			if !strings.Contains(r.Header.Get("Cookie"), "sp_dc=valid-spdc-token") {
				w.Write([]byte(`{"accessToken":"","isAnonymous":true}`))
				return
			}
			exp := time.Now().Add(time.Hour).UnixMilli()
			w.Write([]byte(`{"accessToken":"WEB-ACCESS-TOK","accessTokenExpirationTimestampMs":` + formatInt64(exp) + `,"isAnonymous":false}`))
		case "/pathfinder/v2/query":
			w.Write([]byte(`{"data":{"me":{"profile":{"uri":"spotify:user:sp1","name":"Web User"}}}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	a, err := New(Options{
		DataDir:  filepath.Join(dir, "data"),
		CacheDir: filepath.Join(dir, "cache"),
		Builtin:  builtin.FS,
		Version:  "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.settings.Update(func(s *settings.Settings) error {
		s.SpotifyPlayback = settings.SpotifyPlaybackHere
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	auth := spotify.NewAuth("")
	auth.WebTokenURL = srv.URL + "/api/token"
	auth.ServerTimeURL = srv.URL + "/server-time"
	auth.NuancesURL = srv.URL + "/nuances.json"
	auth.NuancesFallbackURL = ""

	client := spotify.NewClient(auth)
	client.GQLURL = srv.URL + "/pathfinder/v2/query"
	client.HashesURL = ""

	a.auth = auth
	a.spotify = client
	a.tokens = tokenStore{&secretStore{keep: &memKeeper{}, key: "spotify-token"}}
	client.OnToken = func(tok spotify.Token) {
		_ = a.tokens.Save(tok)
	}
	return a, &Accounts{a: a}
}

type memKeeper struct{ val []byte }

func (m *memKeeper) Get(string) ([]byte, error) {
	if m.val == nil {
		return nil, osErrNotFound
	}
	return m.val, nil
}
func (m *memKeeper) Set(_ string, v []byte) error { m.val = append([]byte(nil), v...); return nil }
func (m *memKeeper) Delete(string) error          { m.val = nil; return nil }

var osErrNotFound = spotify.ErrNotSignedIn

func formatInt64(v int64) string {
	var b [24]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	return string(b[i:])
}

func TestSpotifyPasteSPDCAndPlaybackFallback(t *testing.T) {
	a, accts := spdcTestApp(t)

	if _, err := accts.SpotifyPasteSPDC(context.Background(), "bad"); err == nil {
		t.Fatal("expected error for invalid sp_dc input")
	}
	if _, err := accts.SpotifyPasteSPDC(context.Background(), "sp_dc=wrong-cookie-value"); err == nil {
		t.Fatal("expected error when Spotify rejects sp_dc")
	}

	st, err := accts.SpotifyPasteSPDC(context.Background(), "sp_dc=valid-spdc-token")
	if err != nil {
		t.Fatal(err)
	}
	if !st.Spotify.SignedIn || !st.Spotify.WebSession || st.Spotify.Profile == nil || st.Spotify.Profile.Name != "Web User" {
		t.Fatalf("unexpected account status: %+v", st.Spotify)
	}
	if got := a.settings.Get().SpotifyPlayback; got != settings.SpotifyPlaybackMatch {
		t.Errorf("SpotifyPlayback = %q, want %q", got, settings.SpotifyPlaybackMatch)
	}

	// Changing SpotifyClientID should not log out an sp_dc web session.
	prev := a.settings.Get()
	next := prev
	next.SpotifyClientID = "new-client-id"
	a.settingsChanged(prev, next, false)
	if !a.spotify.SignedIn() || !a.spotify.IsWebSession() {
		t.Error("changing SpotifyClientID signed out the sp_dc web session")
	}

	// Even if SpotifyPlaybackHere is forced, a web session routes through PrepareStream instead of failing with missing streaming scope.
	_ = a.settings.Update(func(s *settings.Settings) error {
		s.SpotifyPlayback = settings.SpotifyPlaybackHere
		return nil
	})
	src, err := (&Player{a: a}).Prepare(context.Background(), media.Track{
		ID:     "sp:ep1",
		Source: media.SourceSpotify,
		Episode: &media.Episode{
			Show: media.Ref{ID: "sp:sh1", Name: "Show"},
		},
	})
	if err != errSpotifyEpisode {
		t.Errorf("Prepare for web-session episode = (%+v, %v), want errSpotifyEpisode from PrepareStream", src, err)
	}
}

func TestSpotifyBrowserLogin(t *testing.T) {
	a, accts := spdcTestApp(t)
	a.opts.Browsers = &BrowserSignIn{
		Find: func() []browser.Browser {
			return []browser.Browser{{ID: "firefox", Name: "Firefox"}}
		},
		SignIn: func(ctx context.Context, b browser.Browser, opts browser.Options) ([]*http.Cookie, error) {
			cookies := []*http.Cookie{
				{Name: "sp_dc", Value: "valid-spdc-token", Domain: ".spotify.com", Path: "/"},
				{Name: "sp_t", Value: "t1", Domain: ".spotify.com", Path: "/"},
			}
			if !opts.SignedIn(cookies) {
				t.Error("SignedIn callback rejected valid sp_dc cookie")
			}
			return cookies, nil
		},
	}
	st, err := accts.SpotifyBrowserLogin(context.Background(), "firefox")
	if err != nil {
		t.Fatal(err)
	}
	if !st.Spotify.SignedIn || !st.Spotify.WebSession {
		t.Fatalf("expected signed-in web session, got %+v", st.Spotify)
	}
}

func TestSpotifySPDCPreservesConnectPlayback(t *testing.T) {
	a, accts := spdcTestApp(t)
	if err := a.settings.Update(func(s *settings.Settings) error {
		s.SpotifyPlayback = settings.SpotifyPlaybackConnect
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := accts.SpotifyPasteSPDC(context.Background(), "valid-spdc-token"); err != nil {
		t.Fatal(err)
	}
	if got := a.settings.Get().SpotifyPlayback; got != settings.SpotifyPlaybackConnect {
		t.Errorf("SpotifyPlayback = %q, want %q preserved on sp_dc sign-in", got, settings.SpotifyPlaybackConnect)
	}
}

func TestWebViewMissingSpotifySignIn(t *testing.T) {
	for _, goos := range []string{"windows", "linux", "darwin"} {
		err := webViewMissing(webViewSpotifySignIn, goos)
		msg := ErrorWords(err)
		if !strings.HasPrefix(msg, "Signing in to Spotify needs ") {
			t.Errorf("%s: ErrorWords(webViewMissing(webViewSpotifySignIn)) = %q", goos, msg)
		}
	}
}
