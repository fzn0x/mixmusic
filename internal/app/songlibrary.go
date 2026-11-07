package app

import (
	"context"
	"errors"

	"github.com/fzn0x/mixmusic/internal/media"
)

// errSongsOnYouTube is returned for a track of another service: only
// YouTube Music keeps songs in its library apart from those liked.
var errSongsOnYouTube = errors.New("only YouTube Music keeps songs in your library without liking them")

// YouTubeSongs returns the songs the user saved to their library on
// YouTube Music, apart from those they liked, the last saved first.
func (c *Collection) YouTubeSongs(ctx context.Context) ([]media.Track, error) {
	return c.a.youtube.LibrarySongs(ctx)
}

// SetSongSaved saves a YouTube Music song to the user's library there, or
// takes it out, without liking it or taking the like back.
func (c *Collection) SetSongSaved(ctx context.Context, track media.Track, saved bool) error {
	if track.Source != media.SourceYouTube || track.IsEpisode() {
		return errSongsOnYouTube
	}
	_, videoID, err := media.ParseID(track.ID)
	switch {
	case err != nil:
		return err
	case !c.a.youtube.SignedIn():
		return errNotSignedIn
	}
	return c.a.youtube.SetSongSaved(ctx, videoID, saved)
}
