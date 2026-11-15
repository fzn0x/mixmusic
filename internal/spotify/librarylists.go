package spotify

import (
	"context"
	"net/url"
	"strconv"

	"github.com/fzn0x/mixmusic/internal/media"
)

// listPage reads the page of a list of the user's library, path's, that
// starts at offset.
func listPage[T any](ctx context.Context, c *Client, path string, offset int) (paging[T], error) {
	var page paging[T]
	q := url.Values{"limit": {strconv.Itoa(pageLimit)}, "offset": {strconv.Itoa(offset)}}
	err := c.get(ctx, path, q, &page)
	return page, err
}

// homePlaylists returns the first page of the playlists the user made or
// follows, which Home shows: it doesn't wait for the rest.
func (c *Client) homePlaylists(ctx context.Context, userID string) ([]media.Playlist, error) {
	page, err := listPage[*playlist](ctx, c, "/me/playlists", 0)
	if err != nil {
		return nil, err
	}
	out := []media.Playlist{}
	for _, p := range page.Items {
		if p != nil {
			out = append(out, toPlaylist(p, userID))
		}
	}
	return out, nil
}

// savedAlbum is an item of the albums the user saved.
type savedAlbum struct {
	Album *album `json:"album"`
}

// savedAlbums are the albums of items, without their tracks.
func savedAlbums(items []savedAlbum) []media.Album {
	out := []media.Album{}
	for _, it := range items {
		if it.Album != nil {
			al := toAlbum(it.Album)
			al.Tracks = []media.Track{}
			out = append(out, al)
		}
	}
	return out
}

// homeAlbums returns the first page of the albums the user saved, which
// Home shows.
func (c *Client) homeAlbums(ctx context.Context) ([]media.Album, error) {
	page, err := listPage[savedAlbum](ctx, c, "/me/albums", 0)
	if err != nil {
		return nil, err
	}
	return savedAlbums(page.Items), nil
}
