package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/lastfm"
	"github.com/fzn0x/mixmusic/internal/scrobble"
)

// fakeScrobbleAPI is a Last.fm that takes or refuses what it is sent.
type fakeScrobbleAPI struct {
	refuse bool
	got    map[string]int // session key -> scrobbles accepted
}

func (f *fakeScrobbleAPI) NowPlaying(context.Context, string, lastfm.Track) error {
	if f.refuse {
		return lastfm.ErrSessionExpired
	}
	return nil
}

func (f *fakeScrobbleAPI) Scrobble(_ context.Context, key string, plays []lastfm.Play) error {
	if f.refuse {
		return lastfm.ErrSessionExpired
	}
	f.got[key] += len(plays)
	return nil
}

// connectedWithRefused is an App connected to Last.fm as ann whose
// session Last.fm refuses, with two plays waiting in the saved queue.
func connectedWithRefused(t *testing.T) (*App, *Accounts, *fakeScrobbleAPI) {
	t.Helper()
	dir := t.TempDir()
	store := lastfmStore{testSecret(t, newKeeper(), nil)}
	c := lastfmCredentials{APIKey: "key", Secret: "secret", Session: lastfm.Session{Name: "ann", Key: "old"}}
	if err := store.Save(c); err != nil {
		t.Fatal(err)
	}
	a := &App{opts: Options{DataDir: dir}, lastfmStore: store}
	a.startScrobbling(c)
	api := &fakeScrobbleAPI{refuse: true, got: map[string]int{}}
	l := a.lastfm
	l.scrobbler = scrobble.New(api, "old", scrobble.Options{})
	l.scrobbler.OnQueue = a.lastfmQueueSaver(filepath.Join(dir, "lastfm-queue.json"))
	l.scrobbler.SetQueue([]lastfm.Play{
		{Track: lastfm.Track{Artist: "A", Title: "One", Duration: time.Minute}, Start: time.Unix(1, 0)},
		{Track: lastfm.Track{Artist: "A", Title: "Two", Duration: time.Minute}, Start: time.Unix(2, 0)},
	})
	if err := l.scrobbler.Send(context.Background()); err == nil {
		t.Fatal("the refused session sent")
	}
	l.expired = true
	return a, &Accounts{a: a}, api
}

func TestLastfmReconnectKeepsTheQueue(t *testing.T) {
	a, acc, api := connectedWithRefused(t)
	if got := a.lastfmAccount(); !got.Expired || got.Waiting != 2 {
		t.Fatalf("account = %+v", got)
	}
	api.refuse = false
	if err := a.lastfmNewSession(lastfmCredentials{APIKey: "key", Secret: "secret"}, lastfm.Session{Name: "ann", Key: "new"}); err != nil {
		t.Fatal(err)
	}
	if got := a.lastfmAccount(); got.Expired || got.Other != "" || got.Name != "ann" {
		t.Errorf("account = %+v", got)
	}
	if err := a.lastfm.scrobbler.Send(context.Background()); err != nil {
		t.Fatal(err)
	}
	if api.got["new"] != 2 || len(a.lastfm.scrobbler.Queue()) != 0 {
		t.Errorf("sent %v, %d waiting", api.got, len(a.lastfm.scrobbler.Queue()))
	}
	if c, err := a.lastfmStore.Load(); err != nil || c.Session.Key != "new" {
		t.Errorf("kept %+v, %v", c, err)
	}
	_ = acc
}

func TestLastfmReconnectAsAnotherUser(t *testing.T) {
	for _, send := range []bool{true, false} {
		a, acc, api := connectedWithRefused(t)
		api.refuse = false
		if err := a.lastfmNewSession(lastfmCredentials{APIKey: "key", Secret: "secret"}, lastfm.Session{Name: "bo", Key: "bos"}); err != nil {
			t.Fatal(err)
		}
		// Nothing changes until the user chooses.
		if got := a.lastfmAccount(); got.Other != "bo" || got.Name != "ann" || !got.Expired || got.Waiting != 2 {
			t.Fatalf("account = %+v", got)
		}
		if c, _ := a.lastfmStore.Load(); c.Session.Key != "old" {
			t.Errorf("kept %+v before the choice", c)
		}
		if err := acc.LastfmResolve(context.Background(), send); err != nil {
			t.Fatal(err)
		}
		if err := a.lastfm.scrobbler.Send(context.Background()); err != nil {
			t.Fatal(err)
		}
		want := 0
		if send {
			want = 2
		}
		if got := a.lastfmAccount(); got.Name != "bo" || got.Expired || got.Other != "" || api.got["bos"] != want {
			t.Errorf("send %v: account %+v, sent %v", send, got, api.got)
		}
		data, err := os.ReadFile(filepath.Join(a.opts.DataDir, "lastfm-queue.json"))
		if err != nil || (!send && string(data) != "[]" && string(data) != "null") {
			t.Errorf("send %v: saved queue %q, %v", send, data, err)
		}
	}
}

func TestLastfmLogoutDropsTheQueue(t *testing.T) {
	a, acc, _ := connectedWithRefused(t)
	if err := acc.LastfmLogout(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(a.opts.DataDir, "lastfm-queue.json")); !os.IsNotExist(err) {
		t.Errorf("queue file: %v", err)
	}
	if got := a.lastfmAccount(); got.Connected {
		t.Errorf("account = %+v", got)
	}
}
