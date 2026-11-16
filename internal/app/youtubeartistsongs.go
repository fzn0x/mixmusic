package app

import (
	"context"
	"fmt"

	"github.com/fzn0x/mixmusic/internal/media"
)

// YouTubeArtistSongs reads all the songs of a YouTube Music artist, by
// their ID (yt:UC…), in the shape of ArtistSongs: each is given the new
// songs as they're read, in order, and a read that came in part returns
// what came with a media.ShortList. An artist whose page links no list of
// their songs fails with youtube.ErrNoSongList. ArtistSongs routes
// YouTube Music's artists here.
func (c *Catalog) YouTubeArtistSongs(ctx context.Context, id string, each func([]media.Track)) ([]media.Track, error) {
	source, native, err := media.ParseID(id)
	if err != nil {
		return nil, err
	}
	if source != media.SourceYouTube {
		return nil, fmt.Errorf("%s isn't an artist of YouTube Music", id)
	}
	return c.a.youtube.ArtistSongs(ctx, native, each)
}
