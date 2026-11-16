package spotify

import (
	"context"
	"errors"
	"net/url"

	"github.com/fzn0x/mixmusic/internal/media"
)

// errNotATrack: what Spotify answered of a track's ID is no track, as an
// episode or a local file.
var errNotATrack = errors.New("spotify: not a track")

// Track returns the track of id as Spotify tells it: its title, artists,
// album, length, cover and whether it's explicit. A room's host reads a
// guest's song so, rather than trust what the guest says of it (#734).
func (c *Client) Track(ctx context.Context, id string) (media.Track, error) {
	var t track
	if err := c.get(ctx, "/tracks/"+url.PathEscape(id), nil, &t); err != nil {
		return media.Track{}, err
	}
	mt, ok := toTrack(&t, nil)
	if !ok || mt.ID != media.ID(media.SourceSpotify, id) {
		return media.Track{}, errNotATrack
	}
	return mt, nil
}
