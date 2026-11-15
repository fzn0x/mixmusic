package youtube

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// TestLiveArtistSongs reads all an artist's songs from the list their page
// links from its Top songs, as a playlist: without the link's params, past
// its first page, to its end. It runs with MIXMUSIC_LIVE=1:
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
	var pages []int
	songs, err := c.ArtistSongs(ctx, channel, func(batch []media.Track) { pages = append(pages, len(batch)) })
	if err != nil {
		t.Fatalf("after %d songs: %v", len(songs), err)
	}
	t.Logf("%s: %d songs, in pages of %v", found.Artists[0].Name, len(songs), pages)
	if len(pages) < 2 {
		t.Errorf("%d songs in %d pages", len(songs), len(pages))
	}
}
