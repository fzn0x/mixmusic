package spotify

import (
	"context"
	"slices"
	"testing"
)

// TestHomeShelfKeys names each of Home's Spotify shelves by its kind,
// which Home keeps it by in any language, and leaves out those with
// nothing in them.
func TestHomeShelfKeys(t *testing.T) {
	c, _ := newFake(t, map[string]string{
		"GET /me/playlists":              `{"items": [{"id": "p1", "name": "Mix", "owner": {"id": "u1", "display_name": "Me"}, "items": {"total": 3}}]}`,
		"GET /me/player/recently-played": `{"items": [{"track": ` + trackJSON + `}]}`,
		"GET /me/top/tracks":             `{"items": [` + trackJSON + `]}`,
		"GET /me/albums":                 `{"items": []}`,
	})
	shelves, err := c.Home(context.Background(), "u1")
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, sh := range shelves {
		keys = append(keys, sh.Key)
	}
	if want := []string{"recent", "top", "playlists"}; !slices.Equal(keys, want) {
		t.Errorf("keys %q, want %q", keys, want)
	}
}
