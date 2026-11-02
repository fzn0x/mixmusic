package app

import (
	"context"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/discord"
	"github.com/fzn0x/mixmusic/internal/media"
)

// TestTrackStartsOnce tells that a track started once it sounds: not while
// it loads, nor as the engine holds it before any audio comes, nor when it
// plays again after a pause; again after nothing played (#145).
func TestTrackStartsOnce(t *testing.T) {
	var a App
	ta, tb := &media.Track{ID: "yt:a"}, &media.Track{ID: "yt:b"}
	for i, c := range []struct {
		np      NowPlaying
		started bool
	}{
		{NowPlaying{Track: ta, Loading: true}, false},
		{NowPlaying{Track: ta, Playing: true, Loading: true}, false},
		{NowPlaying{Track: ta, Playing: true}, true},
		{NowPlaying{Track: ta, Playing: true, PositionMs: 1000}, false},
		{NowPlaying{Track: ta}, false}, // paused
		{NowPlaying{Track: ta, Playing: true}, false},
		{NowPlaying{Track: tb, Loading: true}, false}, // the next loads
		{NowPlaying{Track: tb, Playing: true}, true},
		{NowPlaying{}, false}, // stopped
		{NowPlaying{Track: tb, Playing: true}, true},
	} {
		if got := a.startOf(c.np); got != c.started {
			t.Errorf("report %d, %+v: started %v, want %v", i, c.np, got, c.started)
		}
	}
}

// TestLoadingPlays has the media controls and the title count a track that
// loads as playing, as it plays once loaded; a track that failed, neither
// playing nor loading, is paused.
func TestLoadingPlays(t *testing.T) {
	track := &media.Track{ID: "yt:a", Title: "Song"}
	if info := controlsInfo(NowPlaying{Track: track, Loading: true}, time.Now()); !info.Playing {
		t.Error("the controls show a track that loads as paused")
	}
	if info := controlsInfo(NowPlaying{Track: track}, time.Now()); info.Playing {
		t.Error("the controls show a paused track as playing")
	}
	if got := playingLine(NowPlaying{Track: track, Loading: true}); got != "Song" {
		t.Errorf("a track that loads is %q", got)
	}
}

// fakeDiscord is a Discord that tells what it was asked.
type fakeDiscord struct{ calls chan string }

func (f *fakeDiscord) SetActivity(_ context.Context, a discord.Activity) error {
	f.calls <- "set " + a.Details
	return nil
}

func (f *fakeDiscord) Clear(context.Context) error {
	f.calls <- "clear"
	return nil
}

func (f *fakeDiscord) Close() error { return nil }

// TestPresenceBetweenTracks keeps the presence from one track to the next:
// the next shows as it loads, with no Clear between them, and a pause
// clears it (#145).
func TestPresenceBetweenTracks(t *testing.T) {
	f := &fakeDiscord{calls: make(chan string, 8)}
	was := newPresenceClient
	newPresenceClient = func(string) presenceClient { return f }
	t.Cleanup(func() { newPresenceClient = was })
	var p presence
	// idle waits for the call update made to return, after which the next
	// report is taken in.
	idle := func() {
		t.Helper()
		for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(time.Millisecond) {
			p.mu.Lock()
			busy := p.busy
			p.mu.Unlock()
			if !busy {
				return
			}
			if time.Now().After(deadline) {
				t.Fatal("Discord's call didn't return")
			}
		}
	}
	ta, tb := &media.Track{ID: "yt:a", Title: "A"}, &media.Track{ID: "yt:b", Title: "B"}
	for _, c := range []struct {
		np   NowPlaying
		want string // "" for no call
	}{
		{NowPlaying{Track: ta, Playing: true}, "set A"},
		{NowPlaying{Track: tb, Loading: true}, "set B"},
		{NowPlaying{Track: tb, Playing: true}, ""},
		{NowPlaying{Track: tb}, "clear"},
	} {
		p.update(true, "app", c.np)
		got := ""
		select {
		case got = <-f.calls:
		case <-time.After(100 * time.Millisecond):
		}
		if got != c.want {
			t.Errorf("%+v: Discord was told %q, want %q", c.np, got, c.want)
		}
		idle()
	}
}
