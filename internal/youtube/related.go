package youtube

import (
	"context"
	"strings"

	"github.com/fzn0x/mixmusic/internal/media"
)

// Related returns the shelves YouTube Music relates to a track: songs you
// might also like, playlists, other performances and similar artists.
func (c *Client) Related(ctx context.Context, videoID string) ([]media.Shelf, error) {
	n, err := c.call(ctx, "next", map[string]any{"videoId": videoID, "isAudioOnly": true}, nil)
	if err != nil {
		return nil, err
	}
	browseID := relatedBrowseID(n)
	if browseID == "" {
		return []media.Shelf{}, nil
	}
	page, err := c.browse(ctx, browseID)
	if err != nil {
		return nil, err
	}
	return sectionShelves(page.get("contents", "sectionListRenderer", "contents")), nil
}

// relatedBrowseID returns the browse ID of a track's or an episode's related
// tab. The tab is found by its page type, since an episode's ID does not
// start with MPTRt; that prefix still finds a tab that names no page type.
func relatedBrowseID(n node) string {
	for _, tab := range watchNextTabs(n) {
		e := tab.get("tabRenderer", "endpoint")
		id := e.get("browseEndpoint", "browseId").str()
		switch pageType(e) {
		case "MUSIC_PAGE_TYPE_TRACK_RELATED", "MUSIC_PAGE_TYPE_NON_MUSIC_AUDIO_RELATED":
			if id != "" {
				return id
			}
		}
		if strings.HasPrefix(id, "MPTRt") {
			return id
		}
	}
	return ""
}
