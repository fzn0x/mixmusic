package app

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/fzn0x/mixmusic/internal/lastfm"
	"github.com/fzn0x/mixmusic/internal/online"
	"github.com/fzn0x/mixmusic/internal/scrobble"
)

// downAPI is a scrobbling service that fails with err until it's cleared.
type downAPI struct {
	mu    sync.Mutex
	err   error
	calls int
	sent  int
}

func (f *downAPI) NowPlaying(context.Context, string, scrobble.Track) error { return nil }

func (f *downAPI) Scrobble(_ context.Context, _ string, plays []scrobble.Play) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.err != nil {
		return f.err
	}
	f.sent += len(plays)
	return nil
}

func (f *downAPI) set(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

func (f *downAPI) count() (calls, sent int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls, f.sent
}

// connectedLastfm is an App connected to Last.fm through api, whose
// network check fails until up is set.
func connectedLastfm(t *testing.T, api scrobble.Service, up *bool) *App {
	t.Helper()
	var mu sync.Mutex
	a := &App{opts: Options{DataDir: t.TempDir()}, settings: testSettings(t), lastfmStore: lastfmStore{testSecret(t, newKeeper(), nil)}}
	a.online = online.New(func(context.Context) error {
		mu.Lock()
		defer mu.Unlock()
		if !*up {
			return errors.New("no route")
		}
		return nil
	})
	a.startScrobbling(lastfmCredentials{APIKey: "key", Secret: "secret", Session: lastfm.Session{Name: "ann", Key: "sk"}})
	a.lastfm.scrobbler = scrobble.New(api, "sk", scrobble.Options{})
	t.Cleanup(a.stopScrobbling)
	return a
}

// TestScrobblesGoWhenTheNetworkComesBack sends the plays a failed send
// kept as soon as the network answers again, before the timer.
func TestScrobblesGoWhenTheNetworkComesBack(t *testing.T) {
	api := &downAPI{err: &lastfm.APIError{Status: 503}}
	up := false
	a := connectedLastfm(t, api, &up)
	if a.online.Check(context.Background()) {
		t.Fatal("the network answers")
	}
	l := a.lastfm
	l.scrobbler.SetQueue(make([]scrobble.Play, 2))
	a.sendScrobbles(&l.scrobbling)
	eventually(t, "a send", func() bool { calls, _ := api.count(); return calls == 1 })
	api.set(nil)
	a.online.Seen() // audio came: the network answers
	eventually(t, "the plays going", func() bool { _, sent := api.count(); return sent == 2 })
	if l.scrobbler.Waiting() != 0 {
		t.Errorf("%d waiting", l.scrobbler.Waiting())
	}
}

// TestScrobblesRefusedSession marks the connection expired once Last.fm
// refuses its session, and keeps the plays.
func TestScrobblesRefusedSession(t *testing.T) {
	api := &downAPI{err: lastfm.ErrSessionExpired}
	up := true
	a := connectedLastfm(t, api, &up)
	l := a.lastfm
	l.scrobbler.SetQueue(make([]scrobble.Play, 3))
	a.sendScrobbles(&l.scrobbling)
	eventually(t, "the connection expiring", func() bool { return a.lastfmAccount().Expired })
	if got := a.lastfmAccount(); got.Waiting != 3 {
		t.Errorf("account = %+v", got)
	}
}

// TestLastfmLogoutStopsTheSends stops the loop before the queue's file
// goes, and none starts again.
func TestLastfmLogoutStopsTheSends(t *testing.T) {
	api := &downAPI{}
	up := true
	a := connectedLastfm(t, api, &up)
	l := a.lastfm
	a.sendScrobbles(&l.scrobbling)
	if err := (&Accounts{a: a}).LastfmLogout(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-l.sends.done:
	default:
		t.Error("the sends go on")
	}
	a.sendScrobbles(&l.scrobbling)
	if l.sends.done == nil || l.sends.cancel == nil || !l.sends.stopped {
		t.Error("the sends started again")
	}
}

// TestWaitingTold tells the windows how many plays wait while sends fail,
// and once none wait, but not of plays that go at once.
func TestWaitingTold(t *testing.T) {
	var w waitingTold
	steps := []struct {
		fail    *bool
		waiting int
		tell    bool
	}{
		{nil, 1, false}, // a play counts, and goes at once
		{nil, 0, false},
		{nil, 1, false},
		{ptr(true), 1, true}, // its send failed
		{nil, 2, true},       // another joins
		{nil, 2, false},
		{ptr(false), 2, false}, // the network is back: the first batch goes
		{nil, 0, true},
	}
	for i, s := range steps {
		if s.fail != nil {
			w.fail(*s.fail)
		}
		if got := w.changed(s.waiting); got != s.tell {
			t.Errorf("step %d: tells %v, want %v", i, got, s.tell)
		}
	}
}

func ptr[T any](v T) *T { return &v }
