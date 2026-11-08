package youtube

// The songs YouTube Music suggests adding to a playlist of the user's,
// which its website shows below the playlist's songs, five at a time, with
// a refresh for five more.

import (
	"context"

	"github.com/fzn0x/mixmusic/internal/media"
)

// suggestToken returns the token of what YouTube Music shows below the
// songs of a playlist page, which for a playlist of the user's is the
// shelf of suggestions; "" when the page has nothing below them, as the
// liked songs.
func suggestToken(n node) string {
	list := n.get("contents", "twoColumnBrowseResultsRenderer", "secondaryContents", "sectionListRenderer")
	return list.get("continuations", 0, "nextContinuationData", "continuation").str()
}

// PlaylistSuggestions returns the songs YouTube Music suggests adding to a
// playlist of the user's, by the token of the playlist's Suggest or of the
// refresh of the suggestions before, and the token of the refresh, which
// gives the next ones, "" when there are none. A page that has no shelf
// of suggestions where they would be gives none.
func (c *Client) PlaylistSuggestions(ctx context.Context, token string) ([]media.Track, string, error) {
	if err := c.session(); err != nil {
		return nil, "", err
	}
	if token == "" {
		return []media.Track{}, "", nil
	}
	n, err := c.call(ctx, "browse", map[string]any{"continuation": token}, nil)
	if err != nil {
		return nil, "", err
	}
	tracks, refresh := parsePlaylistSuggestions(n)
	return tracks, refresh, nil
}

// parsePlaylistSuggestions reads the answer to the token of the
// suggestions: the first, a list of sections whose shelf holds them, or a
// refresh, the shelf alone. It returns the songs and the token of the
// refresh.
func parsePlaylistSuggestions(n node) ([]media.Track, string) {
	shelf := n.get("continuationContents", "musicShelfContinuation")
	if !shelf.exists() {
		for _, s := range n.get("continuationContents", "sectionListContinuation", "contents").list() {
			if s.get("musicShelfRenderer").exists() {
				shelf = s.get("musicShelfRenderer")
				break
			}
		}
	}
	refresh := ""
	for _, c := range shelf.get("continuations").list() {
		if t := c.get("reloadContinuationData", "continuation").str(); t != "" {
			refresh = t
			break
		}
	}
	return tracksOf(parseItems(shelf.get("contents").list())), refresh
}
