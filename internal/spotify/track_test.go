package spotify

import (
	"context"
	"testing"
)

// TestTrack reads a track by its ID as Spotify tells it, and refuses an
// answer of another track.
func TestTrack(t *testing.T) {
	c, _ := newFake(t, map[string]string{
		"GET /tracks/t1": trackJSON,
		"GET /tracks/t2": trackJSON,
	})
	got, err := c.Track(context.Background(), "t1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "sp:t1" || got.Title != "One More Time" || len(got.Artists) != 1 || got.Album == nil || len(got.Artwork) != 2 {
		t.Fatalf("track %+v", got)
	}
	if _, err := c.Track(context.Background(), "t2"); err == nil {
		t.Fatal("took the answer of another track")
	}
}
