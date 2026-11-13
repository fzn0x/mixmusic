package netease

import (
	"context"
	"os"
	"testing"
)

// TestLive asks NetEase itself, signed out: MIXMUSIC_LIVE=1 runs it.
func TestLive(t *testing.T) {
	if os.Getenv("MIXMUSIC_LIVE") == "" {
		t.Skip("MIXMUSIC_LIVE=1 asks NetEase Cloud Music")
	}
	ctx := context.Background()
	c := New()
	res, err := c.Search(ctx, "周杰伦 晴天")
	if err != nil || len(res.Tracks) == 0 {
		t.Fatalf("Search: %v, %d tracks", err, len(res.Tracks))
	}
	t.Logf("first: %s by %s (%s), %d albums %d artists %d playlists", res.Tracks[0].Title, res.Tracks[0].ArtistNames(), res.Tracks[0].ID, len(res.Albums), len(res.Artists), len(res.Playlists))
	if _, err := c.Playlist(ctx, "3778678", ""); err != nil { // the hot songs chart
		t.Fatalf("Playlist: %v", err)
	}
	if lrc, _, err := c.Lyrics(ctx, "186016"); err != nil || lrc == "" {
		t.Fatalf("Lyrics: %v", err)
	}
	if _, err := c.Similar(ctx, "186016"); err != nil {
		t.Fatalf("Similar: %v", err)
	}
	if shelves, err := c.Home(ctx, ""); err != nil || len(shelves) == 0 {
		t.Fatalf("Home: %v", err)
	}
	if roam, err := c.Roam(ctx, RoamTags[0][1]); err != nil || len(roam) == 0 {
		t.Fatalf("Roam: %v, %d tracks", err, len(roam))
	}
	a, err := c.SongAudio(ctx, "1859245776", "exhigh") // a free song
	t.Logf("audio: %+v %v", a.Type, err)
}
