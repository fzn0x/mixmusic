package youtube

import (
	"context"

	"github.com/fzn0x/mixmusic/internal/media"
)

// AlbumOf returns the browse ID (MPREb_…) of the album whose tracks the
// playlist playlistID holds (OLAK5uy_…), as a link to that playlist names
// the album: YouTube Music answers such a playlist with no header of its
// own, but its tracks name their album. It is ErrNotFound for a playlist
// of its own, which has a header, or one whose tracks name several albums,
// or none.
func (c *Client) AlbumOf(ctx context.Context, playlistID string) (string, error) {
	n, err := c.browse(ctx, "VL"+playlistID)
	if err != nil {
		return "", err
	}
	return albumOf(n)
}

// albumOf reads the album of the tracks of a playlist's page n.
func albumOf(n node) (string, error) {
	if responsiveHeader(n).exists() {
		return "", ErrNotFound
	}
	_, shelf := secondaryShelf(n)
	album := ""
	for _, t := range tracksOf(parseItems(shelf.get("contents").list())) {
		if t.Album == nil || t.Album.ID == "" || album != "" && t.Album.ID != album {
			return "", ErrNotFound
		}
		album = t.Album.ID
	}
	source, id, err := media.ParseID(album)
	if err != nil || source != media.SourceYouTube {
		return "", ErrNotFound
	}
	return id, nil
}
