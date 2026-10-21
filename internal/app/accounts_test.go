package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
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
