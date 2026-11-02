package youtube

import (
	"context"
	"strings"

	"github.com/fzn0x/mixmusic/internal/media"
)

// maxPlaylistPages bounds how many pages of a long playlist are read, of
// about a hundred tracks each.
const maxPlaylistPages = 10

// Playlist returns a playlist with its tracks, by its playlist ID (PL…,
// RD…, OLAK5uy_…), reading up to about a thousand tracks.
func (c *Client) Playlist(ctx context.Context, playlistID string) (media.Playlist, error) {
	n, err := c.browse(ctx, "VL"+playlistID)
	if err != nil {
		return media.Playlist{}, err
	}
	p, token, err := parsePlaylist(n, playlistID)
	if err != nil {
		return p, err
	}
	for page := 1; token != "" && page < maxPlaylistPages; page++ {
		more, err := c.call(ctx, "browse", map[string]any{"continuation": token}, nil)
		if err != nil {
			break // keep the tracks read so far
		}
		var items []node
		items, token = continuationItems(more)
		p.Tracks = append(p.Tracks, tracksOf(parseItems(items))...)
	}
	if p.TrackCount == 0 {
		p.TrackCount = len(p.Tracks)
	}
	return p, nil
}

// parsePlaylist reads the first page of a playlist, and returns the token
// of the next page, if any.

func parsePlaylist(n node, playlistID string) (media.Playlist, string, error) {
	h := responsiveHeader(n)
	if !h.exists() {
		return media.Playlist{}, "", ErrNotFound
	}
	owner := ""
	if artists := parseByline(h.get("straplineTextOne").runs()).fallbackArtists(); len(artists) > 0 {
		owner = artists[0].Name
	}
	p := media.Playlist{
		ID:          media.ID(media.SourceYouTube, playlistID),
		Source:      media.SourceYouTube,
		Title:       h.get("title").text(),
		Description: headerDescription(h),
		Owner:       owner,
		Artwork:     artwork(thumbnailList(h)),
		URL:         "https://music.youtube.com/playlist?list=" + playlistID,
	}
	for _, part := range strings.Split(h.get("secondSubtitle").text(), " • ") {
		if count := trackCount(part); count > 0 {
			p.TrackCount = count
		}
	}
	p.Saved = savedInLibrary(h)
	// YouTube Music shows the header of a playlist the user owns with the
	// tools to edit it, and offers to delete it, in the header's menu or
	// its tools: the tracks, in the other column, have menus of their own.
	head := n.get("contents", "twoColumnBrowseResultsRenderer", "tabs", 0, "tabRenderer", "content", "sectionListRenderer", "contents", 0)
	p.Editable = head.get("musicEditablePlaylistDetailHeaderRenderer").exists()
	p.Deletable = head.find("deletePlaylistEndpoint").exists()
	_, shelf := secondaryShelf(n)
	contents := shelf.get("contents").list()
	p.Tracks = tracksOf(parseItems(contents))
	token := ""
	if len(contents) > 0 {
		token = continuationToken(contents[len(contents)-1])
	}
	if token == "" {
		token = shelf.get("continuations", 0, "nextContinuationData", "continuation").str()
	}
	return p, token, nil
}

// continuationToken returns the token of a continuationItemRenderer, the
// last item of a list that has more.
func continuationToken(item node) string {
	return item.get("continuationItemRenderer", "continuationEndpoint", "continuationCommand", "token").str()
}

// continuationItems returns the items of a continuation response and the
// token of the page after it.
func continuationItems(n node) ([]node, string) {
	if items := n.get("onResponseReceivedActions", 0, "appendContinuationItemsAction", "continuationItems").list(); len(items) > 0 {
		return items, continuationToken(items[len(items)-1])
	}
	shelf := n.get("continuationContents", "musicPlaylistShelfContinuation")
	return shelf.get("contents").list(), shelf.get("continuations", 0, "nextContinuationData", "continuation").str()
}
