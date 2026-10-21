package spotify

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// testClient returns a client of srv, signed in with an access token that
// expired and a refresh token the token server of the test accepts.
func testClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	tokens := tokenServer(t, func(url.Values) {})
	a := NewAuth("client")
	a.TokenURL = tokens.URL
	c := NewClient(a)
	c.BaseURL = srv.URL
	c.SetToken(Token{AccessToken: "EXPIRED", RefreshToken: "REFRESH", Expiry: time.Now().Add(-time.Hour)})
	return c
}

func TestClientRefreshesExpiredTokens(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer ACCESS" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Write([]byte(`{"id": "me"}`))
	}))
	defer srv.Close()
	c := testClient(t, srv)
	saved := make(chan Token, 1)
	c.OnToken = func(tok Token) { saved <- tok }
	var me struct{ ID string }
	if err := c.get(context.Background(), "/me", nil, &me); err != nil || me.ID != "me" {
		t.Fatalf("me = %+v, %v", me, err)
	}
	select {
	case tok := <-saved:
		if tok.AccessToken != "ACCESS" {
			t.Errorf("saved %+v", tok)
		}
	case <-time.After(5 * time.Second):
		t.Error("the new token was not handed to OnToken")
	}
}

func TestClientRetriesOnUnauthorized(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusUnauthorized) // revoked early
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	c := testClient(t, srv)
	c.SetToken(Token{AccessToken: "STALE", RefreshToken: "REFRESH", Expiry: time.Now().Add(time.Hour)})
	if err := c.do(context.Background(), http.MethodPut, "/me/player/pause", nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Errorf("calls = %d", calls.Load())
	}
}

func TestClientReportsAPIErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error": {"status": 403, "message": "Player command failed: Premium required", "reason": "PREMIUM_REQUIRED"}}`))
	}))
	defer srv.Close()
	err := testClient(t, srv).do(context.Background(), http.MethodPut, "/me/player/play", nil, map[string]any{}, nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Reason != "PREMIUM_REQUIRED" {
		t.Fatalf("err = %v", err)
	}
	if apiErr.Error() != "spotify: playback control needs Spotify Premium" {
		t.Errorf("message = %q", apiErr.Error())
	}
}

// TestClientReadsPlainTextErrors keeps the words of the refusals Spotify
// answers in plain text, and none of an HTML page.
func TestClientReadsPlainTextErrors(t *testing.T) {
	for _, c := range []struct{ body, want string }{
		{"Check settings on https://developer.spotify.com/dashboard, the user may not be registered.\n",
			"Check settings on https://developer.spotify.com/dashboard, the user may not be registered."},
		{"<html><body><h1>403 Forbidden</h1></body></html>", ""},
		{"", ""},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(c.body))
		}))
		err := testClient(t, srv).get(context.Background(), "/me", nil, nil)
		srv.Close()
		var apiErr *APIError
		if !errors.As(err, &apiErr) || apiErr.Status != http.StatusForbidden || apiErr.Message != c.want {
			t.Errorf("%q: err = %#v, want the message %q", c.body, err, c.want)
		}
	}
}

func TestClientNotSignedIn(t *testing.T) {
	c := NewClient(NewAuth("client"))
	if err := c.get(context.Background(), "/me", nil, nil); !errors.Is(err, ErrNotSignedIn) {
		t.Errorf("err = %v", err)
	}
	if c.SignedIn() {
		t.Error("SignedIn without a token")
	}
}

func TestGranted(t *testing.T) {
	c := NewClient(NewAuth("client"))
	c.SetToken(Token{AccessToken: "A", Scope: "user-library-read streaming"})
	if !c.Granted("streaming") || c.Granted("user-read-private") || c.Granted("stream") {
		t.Error("Granted does not read the token's scopes")
	}
	c.SetToken(Token{})
	if c.Granted("streaming") {
		t.Error("a signed-out client grants streaming")
	}
}

func TestRefreshWaitsForNoOne(t *testing.T) {
	// The token server takes its time, as on a bad network.
	started, answer := make(chan struct{}), make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-answer
		w.Write([]byte(`{"access_token": "NEW", "expires_in": 3600}`))
	}))
	t.Cleanup(srv.Close)
	var once sync.Once
	release := func() { once.Do(func() { close(answer) }) }
	t.Cleanup(release) // before the server closes, which waits for its handlers

	a := NewAuth("client")
	a.TokenURL = srv.URL
	c := NewClient(a)
	c.SetToken(Token{AccessToken: "OLD", RefreshToken: "REFRESH", Expiry: time.Now().Add(-time.Hour), Scope: "streaming"})
	got := make(chan error, 1)
	go func() {
		_, err := c.accessToken(context.Background(), false)
		got <- err
	}()
	<-started

	asked := make(chan bool, 1)
	go func() { asked <- c.SignedIn() && c.Granted("streaming") }()
	select {
	case ok := <-asked:
		if !ok {
			t.Error("the user is not signed in with streaming while the token refreshes")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("SignedIn waited for the token to refresh")
	}

	// Signing out while it refreshes stands.
	c.SetToken(Token{})
	release()
	if err := <-got; !errors.Is(err, ErrNotSignedIn) {
		t.Errorf("refresh after signing out = %v, want ErrNotSignedIn", err)
	}
	if c.SignedIn() {
		t.Error("the refreshed token signed the user back in")
	}
}
