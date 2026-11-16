package youtube

import (
	"context"
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

// TestSongSignedOut asks for a song as a visitor YouTube doesn't know,
// though the user is signed in: a room's guest chooses which.
func TestSongSignedOut(t *testing.T) {
	c, requests := labelServer(t, "next", false)
	c.SetCookies(session)
	c.SetChannel("PAGE")
	if _, err := c.Song(context.Background(), "w3XzgjlKFjc"); err != nil {
		t.Fatal(err)
	}
	got := requests()
	if len(got) != 1 {
		t.Fatalf("%d requests", len(got))
	}
	for _, h := range []string{"Cookie", "Authorization", "X-Goog-AuthUser", "X-Goog-PageId", "X-Goog-Visitor-Id"} {
		if v := got[0].header.Get(h); v != "" {
			t.Errorf("it sent %s: %q", h, v)
		}
	}
	if user := parseNode(got[0].body).get("context", "user", "onBehalfOfUser"); user.exists() {
		t.Errorf("it asked on behalf of %v", user.v)
	}
}
