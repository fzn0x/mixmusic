package youtube

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

// TestLiveLinkLists reads what pasted links name by their lists, as a
// visitor: an album by the playlist of its tracks (OLAK5uy_…), one of
// YouTube Music's own playlists (RDCLAK5uy_…), and a radio of youtube.com
// (RD…) from its song, which it starts with. It runs with MIXMUSIC_LIVE=1:
//
//	MIXMUSIC_LIVE=1 go test ./internal/youtube -run TestLiveLinkLists -v
func TestLiveLinkLists(t *testing.T) {
	if os.Getenv("MIXMUSIC_LIVE") == "" {
		t.Skip("set MIXMUSIC_LIVE=1 to read the lists of links on YouTube Music")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	c := New()
	// Q-ENA II, the album of the recorded album.json.
	album, err := c.AlbumOf(ctx, "OLAK5uy_mByMItZzRVpiPOPfqNdCyzqCEs1k7Ow14")
	if err != nil || album != "MPREb_I0lA998lMLT" {
		t.Errorf("the album's playlist is of %q, %v", album, err)
	}
	// Chill Bangers, of the recorded suggest-playlists.json.
	p, err := c.Playlist(ctx, "RDCLAK5uy_kTh6wa62hZCbKX0oevcV-_LD-A8l7CInk")
	if err != nil || p.Title == "" || len(p.Tracks) == 0 {
		t.Errorf("YouTube Music's playlist: %q, %d tracks, %v", p.Title, len(p.Tracks), err)
	}
	if _, err := c.AlbumOf(ctx, "RDCLAK5uy_kTh6wa62hZCbKX0oevcV-_LD-A8l7CInk"); !errors.Is(err, ErrNotFound) {
		t.Errorf("YouTube Music's playlist is an album's: %v", err)
	}
	// Daft Punk's One More Time, as an Art Track.
	tracks, err := c.RadioOf(ctx, "FGBhQbmPwH8", "RDFGBhQbmPwH8")
	if err != nil || len(tracks) < 2 || tracks[0].ID != "yt:FGBhQbmPwH8" {
		t.Fatalf("the radio: %d tracks, %v", len(tracks), err)
	}
	t.Logf("the radio of %s: %d tracks, then %s", tracks[0].Title, len(tracks)-1, tracks[1].Title)
}
