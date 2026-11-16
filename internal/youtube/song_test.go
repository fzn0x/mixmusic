package youtube

import (
	"errors"
	"testing"
)

// TestSongOf reads a song by its video ID from a "next" response, as
// YouTube Music tells it, and finds none of another ID.
func TestSongOf(t *testing.T) {
	n := fixture(t, "next")
	got, err := songOf(n, "w3XzgjlKFjc")
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Draft Punk" || len(got.Artists) != 1 || got.Artists[0].Name != "DJT" || got.DurationMs != 208000 || len(got.Artwork) == 0 {
		t.Fatalf("song %+v", got)
	}
	if _, err := songOf(n, "notinthere0"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("another ID: %v", err)
	}
}
