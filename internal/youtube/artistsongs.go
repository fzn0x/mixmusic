package youtube

import (
	"strings"

	"github.com/fzn0x/mixmusic/internal/media"
)

// songsList is the ID of the playlist of all an artist's songs, which the
// title of the Top songs of their page and its Show all link open, or ""
// when neither opens a playlist (#543).
func songsList(shelf node) string {
	for _, end := range []node{shelf.get("title", "runs", 0, "navigationEndpoint"), shelf.get("bottomEndpoint")} {
		id := end.get("browseEndpoint", "browseId").str()
		if strings.HasPrefix(id, "VL") && pageType(end) == pageTypePlaylist {
			return media.ID(media.SourceYouTube, strings.TrimPrefix(id, "VL"))
		}
	}
	return ""
}
