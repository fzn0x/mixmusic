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

// SongSaved reports whether a YouTube Music song is in the user's library
// there, as YouTube Music tells of the song itself: for one the songs read
// may lack, of a list that came in part.
func (c *Collection) SongSaved(ctx context.Context, track media.Track) (bool, error) {
	videoID, err := c.librarySong(track)
	if err != nil {
		return false, err
	}
	return c.a.youtube.SongSaved(ctx, videoID)
}

// SetSongSaved saves a YouTube Music song to the user's library there, or
// takes it out, without liking it or taking the like back.
func (c *Collection) SetSongSaved(ctx context.Context, track media.Track, saved bool) error {
	videoID, err := c.librarySong(track)
	if err != nil {
		return err
	}
	return c.a.youtube.SetSongSaved(ctx, videoID, saved)
}

// librarySong returns the video ID of a YouTube Music song, which the
// user's library there may keep, while they're signed in.
func (c *Collection) librarySong(track media.Track) (string, error) {
	if track.Source != media.SourceYouTube || track.IsEpisode() {
		return "", errSongsOnYouTube
	}
	_, videoID, err := media.ParseID(track.ID)
	switch {
	case err != nil:
		return "", err
	case !c.a.youtube.SignedIn():
		return "", errNotSignedIn
	}
	return videoID, nil
}
