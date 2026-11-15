package youtube

import (
	"context"
	"testing"
)

// topVotedItem is the item that ends a page of a playlist sorted by Top
// voted, which has more: its continuationCommand comes wrapped in a
// commandExecutorCommand, beside a playlistVotingRefreshPopupCommand
// (#626). It's hand-built in the shape the issue describes: the test data
// holds no recorded answer of a Top voted playlist of more than 100 songs
// yet, and a recorded one replaces it once it does.
func topVotedItem(commands ...any) any {
	return map[string]any{"continuationItemRenderer": map[string]any{
		"trigger": "CONTINUATION_TRIGGER_ON_ITEM_SHOWN",
		"continuationEndpoint": map[string]any{"commandExecutorCommand": map[string]any{
			"commands": append([]any{map[string]any{"playlistVotingRefreshPopupCommand": map[string]any{}}}, commands...),
		}},
	}}
}

// continueWith is a continuationCommand of a commandExecutorCommand.
func continueWith(token, request string) any {
	return map[string]any{"continuationCommand": map[string]any{"token": token, "request": request}}
}

// browseOn is the continuationCommand of a Top voted playlist's next page.
func browseOn(token string) any { return continueWith(token, "CONTINUATION_REQUEST_TYPE_BROWSE") }

// TestTopVotedPlaylistPages reads all of a playlist sorted by Top voted,
// whose pages end in the wrapped token, past its first 100 songs (#626):
// the recorded playlist's tracks, again and again, in pages of 100.
func TestTopVotedPlaylistPages(t *testing.T) {
	first := fixture(t, "playlist")
	_, shelf := secondaryShelf(first)
	recorded := shelf.get("contents").v.([]any)
	want := tracksOf(parseItems(shelf.get("contents").list()))
	if len(want) == 0 || len(want) != len(recorded) {
		t.Fatalf("the fixture has %d items, of %d tracks", len(recorded), len(want))
	}
	page := func(from, to int, end ...any) []any {
		var items []any
		for i := from; i < to; i++ {
			items = append(items, recorded[i%len(recorded)])
		}
		return append(items, end...)
	}
	appended := func(items []any) any {
		return map[string]any{"onResponseReceivedActions": []any{map[string]any{"appendContinuationItemsAction": map[string]any{
			"continuationItems": items,
		}}}}
	}
	const total = 227
	shelf.v.(map[string]any)["contents"] = page(0, 100, topVotedItem(browseOn("V1")))
	c, _ := pages(t, map[string]any{
		"":   first.v,
		"V1": appended(page(100, 200, topVotedItem(browseOn("V2")))),
		"V2": appended(page(200, total)),
	})
	ctx := context.Background()
	p, err := c.Playlist(ctx, "PLfP6i5T0-DkIMLNRwmJpRBs4PJvxfgwBg")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Tracks) != 100 || p.More != "V1" {
		t.Fatalf("the first page: %d tracks, more %q", len(p.Tracks), p.More)
	}
	if err := p.ReadOn(ctx, c.PlaylistPage, nil); err != nil {
		t.Fatal(err)
	}
	if len(p.Tracks) != total || p.More != "" || p.Cut {
		t.Fatalf("read on: %d tracks, more %q, cut %v; want all %d", len(p.Tracks), p.More, p.Cut, total)
	}
	for i, track := range p.Tracks {
		if track.ID != want[i%len(want)].ID {
			t.Fatalf("track %d is %s, want %s", i, track.ID, want[i%len(want)].ID)
		}
	}
}

// TestTopVotedToken takes the token of the wrapped command that continues
// a browse, else of the first that continues, from the end of a playlist's
// page and of a library list's, and still the plain one.
func TestTopVotedToken(t *testing.T) {
	for _, c := range []struct {
		name string
		item any
		want string
	}{
		{"plain", moreItem("P1"), "P1"},
		{"wrapped", topVotedItem(browseOn("V1")), "V1"},
		{"the browse first", topVotedItem(continueWith("R1", "CONTINUATION_REQUEST_TYPE_REEL_WATCH_SEQUENCE"), browseOn("V1")), "V1"},
		{"else the first", topVotedItem(continueWith("R1", ""), continueWith("R2", "")), "R1"},
		{"none", topVotedItem(), ""},
	} {
		if got := continuationToken(node{c.item}); got != c.want {
			t.Errorf("%s: continuationToken = %q, want %q", c.name, got, c.want)
		}
		page := libraryPage(map[string]any{"musicShelfRenderer": map[string]any{"contents": []any{c.item}}})
		if got := listToken(page); got != c.want {
			t.Errorf("%s: listToken = %q, want %q", c.name, got, c.want)
		}
	}
	items, token := listContinuation(node{map[string]any{"onResponseReceivedActions": []any{map[string]any{"appendContinuationItemsAction": map[string]any{
		"continuationItems": []any{topVotedItem(browseOn("V2"))},
	}}}}})
	if len(items) != 1 || token != "V2" {
		t.Errorf("listContinuation: %d items, token %q, want V2", len(items), token)
	}
}
