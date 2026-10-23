package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/spotify"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

func TestAccountsStatusWaitsForNoService(t *testing.T) {
	// Spotify takes its time to answer, as on a bad network.
	answer := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-answer
		w.Write([]byte(`{"id": "me", "display_name": "Me"}`))
	}))
	t.Cleanup(srv.Close)
	var once sync.Once
	release := func() { once.Do(func() { close(answer) }) }
	t.Cleanup(release) // before the server closes, which waits for its handlers

	c := spotify.NewClient(spotify.NewAuth("client"))
	c.BaseURL = srv.URL
	c.SetToken(spotify.Token{AccessToken: "ACCESS", Expiry: time.Now().Add(time.Hour)})
	accounts := &Accounts{a: &App{auth: spotify.NewAuth("client"), spotify: c, youtube: youtube.New()}}

	got := make(chan AccountsStatus, 1)
	go func() { got <- accounts.Status(context.Background()) }()
	select {
	case s := <-got:
		if !s.Spotify.SignedIn || s.Spotify.Profile != nil {
			t.Errorf("status before Spotify answered = %+v", s.Spotify)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Status waited for Spotify")
	}

	// The profile read in the background comes with the next status.
	release()
	deadline := time.Now().Add(5 * time.Second)
	for {
		s := accounts.Status(context.Background())
		if s.Spotify.Profile != nil {
			if s.Spotify.Profile.Name != "Me" {
				t.Errorf("profile = %+v", s.Spotify.Profile)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the profile was never read")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestRefusedProfile says why Spotify refused the profile, and reads it
// again only after a while: every window asks for the status again once a
// reading tells it, and reading again at once read it again and again,
// while the windows showed the account signed out between the answers
// (#50).
func TestRefusedProfile(t *testing.T) {
	var reads atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reads.Add(1)
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("Check settings on https://developer.spotify.com/dashboard, the user may not be registered."))
	}))
	t.Cleanup(srv.Close)
	c := spotify.NewClient(spotify.NewAuth("client"))
	c.BaseURL = srv.URL
	c.SetToken(spotify.Token{AccessToken: "ACCESS", Expiry: time.Now().Add(time.Hour)})
	a := &App{auth: spotify.NewAuth("client"), spotify: c, youtube: youtube.New()}
	accounts := &Accounts{a: a}

	accounts.Status(context.Background())
	waitProfiles(t, a, func() bool { return reads.Load() == 1 })
	s := accounts.Status(context.Background())
	if !s.Spotify.SignedIn || !strings.Contains(s.Spotify.Error, "Users Management") {
		t.Errorf("status after the refusal = %+v", s.Spotify)
	}
	if a.readingProfiles.Load() {
		t.Error("asking again reads the refused profile again at once")
	}

	// A while later, the profile is read again.
	a.mu.Lock()
	a.profileErrs.at = a.profileErrs.at.Add(-profileRetry)
	a.mu.Unlock()
	accounts.Status(context.Background())
	waitProfiles(t, a, func() bool { return reads.Load() == 2 })
}

// TestRefusedSpotifySignIn fails a sign-in Spotify refuses, as it refuses
// every app whose owner has no Premium, rather than say it signed in, and
// the accounts say why at once.
func TestRefusedSpotifySignIn(t *testing.T) {
	var reads atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reads.Add(1)
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("Active premium subscription required for the owner of the app. When the subscription status changes, it can take a few hours to be applied."))
	}))
	t.Cleanup(srv.Close)
	c := spotify.NewClient(spotify.NewAuth("client"))
	c.BaseURL = srv.URL
	c.SetToken(spotify.Token{AccessToken: "ACCESS", Expiry: time.Now().Add(time.Hour)})
	a := &App{auth: spotify.NewAuth("client"), spotify: c, youtube: youtube.New()}

	status, err := a.spotifySignedIn(context.Background())
	if err == nil || !strings.Contains(err.Error(), "Premium") {
		t.Errorf("signing in = %v, want the refusal", err)
	}
	if !status.Spotify.SignedIn || status.Spotify.Error == "" {
		t.Errorf("status after the refusal = %+v", status.Spotify)
	}
	s := (&Accounts{a: a}).Status(context.Background())
	if s.Spotify.Error == "" || a.readingProfiles.Load() {
		t.Errorf("the accounts don't say why at once: %+v, reading again %v", s.Spotify, a.readingProfiles.Load())
	}
	if n := reads.Load(); n != 1 {
		t.Errorf("the profile was read %d times", n)
	}
}

// waitProfiles waits for the reading of the profiles in the background to
// end with done.
func waitProfiles(t *testing.T, a *App, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for a.readingProfiles.Load() || !done() {
		if time.Now().After(deadline) {
			t.Fatal("the profiles were never read")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
