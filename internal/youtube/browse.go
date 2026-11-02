package youtube

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/fzn0x/mixmusic/internal/media"
)

var countRe = regexp.MustCompile(`^([\d,]+) (?:songs?|tracks?|episodes?)`)

// ErrNotFound is returned for items YouTube Music does not have.
var ErrNotFound = errors.New("youtube music: not found")

// browse reads a page of YouTube Music by its browse ID.
func (c *Client) browse(ctx context.Context, browseID string) (node, error) {
	return c.call(ctx, "browse", map[string]any{"browseId": browseID}, nil)
}

// responsiveHeader returns the header of an album or playlist page.
func responsiveHeader(n node) node {
	section := n.get("contents", "twoColumnBrowseResultsRenderer", "tabs", 0, "tabRenderer", "content", "sectionListRenderer", "contents", 0)
	if h := section.get("musicResponsiveHeaderRenderer"); h.exists() {
		return h
	}
	return section.get("musicEditablePlaylistDetailHeaderRenderer", "header", "musicResponsiveHeaderRenderer")
}

// secondaryShelf returns the shelf of tracks of an album or playlist page.
func secondaryShelf(n node) (string, node) {
	return n.get("contents", "twoColumnBrowseResultsRenderer", "secondaryContents", "sectionListRenderer", "contents", 0).renderer()
}

// savedInLibrary reports whether the "save to library" button of an album
// or playlist header is on, which YouTube Music shows to a signed-in user.
func savedInLibrary(h node) bool {
	return saveToggle(h).get("isToggled").boolean()
}

// saveToggle returns the "save to library" button of an album or playlist
// header, or the zero node. YouTube Music draws it with a bookmark, as
// BOOKMARK_BORDER and BOOKMARK, and drew it with LIBRARY_ADD and
// LIBRARY_SAVED before; signed in, it likes or unlikes the playlist.
func saveToggle(h node) node {
	for _, b := range h.get("buttons").list() {
		t := b.get("toggleButtonRenderer")
		icon := t.get("defaultIcon", "iconType").str()
		if strings.Contains(icon, "LIBRARY") || strings.HasPrefix(icon, "BOOKMARK") ||
			t.get("defaultServiceEndpoint", "likeEndpoint").exists() || t.get("toggledServiceEndpoint", "likeEndpoint").exists() {
			return t
		}
	}
	return node{}
}

// headerDescription returns the description of a responsive header.
func headerDescription(h node) string {
	return strings.TrimSpace(h.get("description", "musicDescriptionShelfRenderer", "description").text())
}

// trackCount reads the count of "8 songs • 20 minutes".
func trackCount(text string) int {
	m := countRe.FindStringSubmatch(text)
	if m == nil {
		return 0
	}
	n, _ := strconv.Atoi(strings.ReplaceAll(m[1], ",", ""))
	return n
}

// tracksOf returns the tracks among items.
func tracksOf(items []media.Item) []media.Track {
	tracks := []media.Track{}
	for _, item := range items {
		if item.Kind == media.ItemTrack {
			tracks = append(tracks, *item.Track)
		}
	}
	return tracks
}
