package app

import (
	"context"
	"errors"

	"github.com/fzn0x/mixmusic/internal/media"
)

// ErrNoArtistSongs is why ArtistSongs can't read the songs of an artist of
// a service it doesn't yet read them of.
var ErrNoArtistSongs = errors.New("this artist's songs can't be read all at once")

// ArtistSongs reads all the songs of the artist with the id given, of any
// service that has a way to, for Play on their page: each is given the new
// songs as they are read, in order, so that playing can start with the
// first. A read that came in part returns what came with a
// media.ShortList. Spotify's artists are read from all their albums and
// singles, YouTube Music's from the list of their songs.
func (c *Catalog) ArtistSongs(ctx context.Context, id string, each func([]media.Track)) ([]media.Track, error) {
	source, native, err := media.ParseID(id)
	if err != nil {
		return nil, err
	}
	switch source {
	case media.SourceSpotify:
		return c.a.spotify.ArtistSongs(ctx, native, each)
	case media.SourceYouTube:
		return c.YouTubeArtistSongs(ctx, id, each)
	}
	return nil, ErrNoArtistSongs
}
