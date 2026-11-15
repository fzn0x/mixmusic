package youtube

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// TestLiveArtistSongs reads the playlist of all an artist's songs, which
// their page links from its Top songs, as a playlist: without the link's
// params, past its first page, to its end. It runs with MIXMUSIC_LIVE=1:
//
//	MIXMUSIC_LIVE=1 go test ./internal/youtube -run TestLiveArtistSongs -v
func TestLiveArtistSongs(t *testing.T) {
	if os.Getenv("MIXMUSIC_LIVE") == "" {
		t.Skip("set MIXMUSIC_LIVE=1 to read an artist's songs on YouTube Music")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	c := New()
	found, err := c.SearchAll(ctx, "daft punk")
	if err != nil || len(found.Artists) == 0 {
		t.Fatalf("search: %d artists, %v", len(found.Artists), err)
	}
	_, channel, _ := media.ParseID(found.Artists[0].ID)
	a, err := c.Artist(ctx, channel)
	if err != nil {
		t.Fatal(err)
	}
	if a.SongsID == "" {
		t.Fatalf("%s: no list of their songs, beside %d top songs", a.Name, len(a.TopTracks))
	}
	_, id, _ := media.ParseID(a.SongsID)
	p, err := c.Playlist(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	first := len(p.Tracks)
	if err := p.ReadOn(ctx, c.PlaylistPage, nil); err != nil {
		t.Fatalf("after %d songs: %v", len(p.Tracks), err)
	}
	seen := map[string]bool{}
	twice := 0
	for _, tr := range p.Tracks {
		if seen[tr.ID] {
			twice++
		}
		seen[tr.ID] = true
	}
	t.Logf("%s: %q, %d songs of %d, %d on the first page, %d twice", a.Name, p.Title, len(p.Tracks), p.TrackCount, first, twice)
	if len(p.Tracks) <= first || len(p.Tracks) <= len(a.TopTracks) {
		t.Errorf("%d songs, %d on the first page, %d top songs", len(p.Tracks), first, len(a.TopTracks))
	}
}
