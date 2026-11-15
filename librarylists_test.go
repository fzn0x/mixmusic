package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// TestYouTubeListsTakeTheirTime gives a list of the library on YouTube
// Music, read page after page, longer than fetch's minute to come.
func TestYouTubeListsTakeTheirTime(t *testing.T) {
	var (
		mu   sync.Mutex
		left time.Duration
	)
	was := listOnYouTube
	listOnYouTube = func(_ *shell, ctx context.Context) ([]media.Playlist, error) {
		deadline, _ := ctx.Deadline()
		mu.Lock()
		left = time.Until(deadline)
		mu.Unlock()
		return []media.Playlist{}, nil
	}
	t.Cleanup(func() { listOnYouTube = was })
	s := newTestShell(t)
	s.loads["accounts"] = signedInTo(true, false)
	tt := tester(t, s)
	until(t, s, tt, "the playlists", func() bool { return s.youtubePlaylists().done })
	mu.Lock()
	defer mu.Unlock()
	if left <= 10*time.Minute {
		t.Errorf("the list had %v to come", left)
	}
}
