package spotify

import (
	"context"
	"strings"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
)

// TestPlaylistCountsWhatItLeavesOut counts, by kind, the items of a
// playlist's pages that aren't tracks MixMusic plays (#625): the first
// page's and those read after it.
func TestPlaylistCountsWhatItLeavesOut(t *testing.T) {
	local := `{"item": {"id": "", "name": "Demo", "is_local": true, "type": "track"}}`
	episode := `{"item": {"id": "e1", "name": "Show", "type": "episode"}}`
	gone := `{"item": null}`
	t2 := strings.Replace(trackJSON, `"t1"`, `"t2"`, 1)
	c, _ := newFake(t, map[string]string{
		"GET /playlists/p1": `{"id": "p1", "name": "Mine", "owner": {"id": "me", "display_name": "Me"},
			"items": {"total": 7, "items": [{"item": ` + trackJSON + `}, ` + local + `, ` + episode + `],
			"next": "{{base}}/playlists/p1/items?offset=3"}}`,
		"GET /playlists/p1/items": `{"total": 7, "items": [` + local + `, ` + gone + `, {"item": ` + t2 + `}, ` + local + `]}`,
	})
	ctx := context.Background()
	p, err := c.Playlist(ctx, "p1", "me")
	if err != nil {
		t.Fatal(err)
	}
	if want := (media.Unplayable{Local: 1, Episodes: 1}); p.Unplayable != want {
		t.Fatalf("first page left out %+v, want %+v", p.Unplayable, want)
	}
	if err := p.ReadOnCounting(ctx, c.PlaylistPageCounting, nil); err != nil {
		t.Fatal(err)
	}
	if want := (media.Unplayable{Local: 3, Episodes: 1, Gone: 1}); p.Unplayable != want {
		t.Errorf("left out %+v, want %+v", p.Unplayable, want)
	}
	if got := len(p.Tracks) + p.Unplayable.Total(); got != p.TrackCount {
		t.Errorf("%d rows and %d left out are not the %d tracks Spotify counts", len(p.Tracks), p.Unplayable.Total(), p.TrackCount)
	}
}
