package youtube

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestLiveSong reads a song by its video ID, as a room's host reads a
// guest's: its title and artists as YouTube Music tells them. It runs with
// MIXMUSIC_LIVE=1:
//
//	MIXMUSIC_LIVE=1 go test ./internal/youtube -run TestLiveSong -v
func TestLiveSong(t *testing.T) {
	if os.Getenv("MIXMUSIC_LIVE") == "" {
		t.Skip("set MIXMUSIC_LIVE=1 to read a song on YouTube Music")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	// Daft Punk's One More Time, as an Art Track.
	got, err := New().Song(ctx, "FGBhQbmPwH8")
	if err != nil {
		t.Fatal(err)
	}
	if got.Title == "" || len(got.Artists) == 0 || len(got.Artwork) == 0 {
		t.Fatalf("song %+v", got)
	}
	t.Logf("%s by %s, %d ms, explicit %v", got.Title, got.Artists[0].Name, got.DurationMs, got.Explicit)
}
