package main

import (
	"context"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// TestLongPlaylistSaysItWaits has the page say that the service asked for
// a wait while the read keeps it, and the read stop waiting once the loads
// no longer keep it.
func TestLongPlaylistSaysItWaits(t *testing.T) {
	s := newTestShell(t)
	key := "playlist:" + longPlaylistID
	ended := make(chan error, 1)
	read := readPlaylistOn
	readPlaylistOn = func(s *shell, ctx context.Context, p media.Playlist, onPage func(media.Playlist)) (media.Playlist, error) {
		_, err := media.PageWait(ctx, 30*time.Second, time.Minute)
		ended <- err
		return p, err
	}
	t.Cleanup(func() { readPlaylistOn = read })
	s.loads[key] = longHead(longPlaylistID)
	tt := tester(t, s)
	show(s, tt, playlistPath(longPlaylistID))
	r, _ := s.loads[key+" read"].(*playlistRead)
	waitFor(t, s, func() bool { return r.waiting })
	tt.Frame()
	if !tt.HasText("Spotify asked MixMusic to wait · 2 of 6") || tt.HasText("Loading tracks · 2 of 6") {
		t.Fatalf("the page shows %q", tt.Texts())
	}
	delete(s.loads, key+" read")
	// The loads' updates run on the page's side: drain them as they come.
	for deadline := time.Now().Add(10 * time.Second); ; {
		s.drain()
		select {
		case err := <-ended:
			if err == nil {
				t.Error("the wait went on to its end")
			}
			return
		case <-time.After(10 * time.Millisecond):
		}
		if time.Now().After(deadline) {
			t.Fatal("the dropped read went on waiting")
		}
	}
}
