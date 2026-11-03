package youtube

import (
	"context"
	"slices"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
)

// moreItem is the item that ends a page of a list that has more, with the
// token of the next.
func moreItem(token string) any {
	return map[string]any{"continuationItemRenderer": map[string]any{"continuationEndpoint": map[string]any{
		"continuationCommand": map[string]any{"token": token}}}}
}

// TestPlaylistPages reads a playlist's first page at once, with the token
// of the next, and the pages after it, in either of YouTube's two forms:
// the recorded playlist, cut into pages of two tracks.
func TestPlaylistPages(t *testing.T) {
	first := fixture(t, "playlist")
	_, shelf := secondaryShelf(first)
	items := shelf.get("contents").v.([]any)
	if len(items) < 6 {
		t.Fatalf("the fixture has %d tracks", len(items))
	}
	want := tracksOf(parseItems(shelf.get("contents").list()))
	shelf.v.(map[string]any)["contents"] = append(slices.Clone(items[:2]), moreItem("P1"))
	c, asked := pages(t, map[string]any{
		"": first.v,
		"P1": map[string]any{"onResponseReceivedActions": []any{map[string]any{"appendContinuationItemsAction": map[string]any{
			"continuationItems": append(slices.Clone(items[2:4]), moreItem("P2")),
		}}}},
		"P2": map[string]any{"continuationContents": map[string]any{"musicPlaylistShelfContinuation": map[string]any{
			"contents": items[4:6],
		}}},
	})
	ctx := context.Background()
	p, err := c.Playlist(ctx, "PLfP6i5T0-DkIMLNRwmJpRBs4PJvxfgwBg")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Tracks) != 2 || p.More != "P1" || p.TrackCount != 56 || asked.Load() != 1 {
		t.Fatalf("the first page: %d tracks, more %q, count %d, %d asks", len(p.Tracks), p.More, p.TrackCount, asked.Load())
	}
	tracks, next, err := c.PlaylistPage(ctx, "P1")
	if err != nil || len(tracks) != 2 || tracks[0].ID != want[2].ID || next != "P2" {
		t.Fatalf("the second page: %d tracks, next %q, %v", len(tracks), next, err)
	}
	tracks, next, err = c.PlaylistPage(ctx, "P2")
	if err != nil || len(tracks) != 2 || tracks[1].ID != want[5].ID || next != "" {
		t.Fatalf("the last page: %d tracks, next %q, %v", len(tracks), next, err)
	}

	if err := p.ReadOn(ctx, c.PlaylistPage, nil); err != nil {
		t.Fatal(err)
	}
	if !slices.EqualFunc(p.Tracks, want, func(a, b media.Track) bool { return a.ID == b.ID }) || p.More != "" {
		t.Errorf("read on: %d tracks, more %q", len(p.Tracks), p.More)
	}
}
