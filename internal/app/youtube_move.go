package app

import (
	"context"

	"github.com/fzn0x/mixmusic/internal/media"
)

// MoveInYouTubePlaylist moves a track, as read from a playlist of the user
// on YouTube Music, to just before another of it, or to its end when
// before is the zero track.
func (c *Collection) MoveInYouTubePlaylist(ctx context.Context, playlistID string, track, before media.Track) error {
	if !c.a.youtube.SignedIn() {
		return errNotSignedIn
	}
	list, err := youtubePlaylistID(playlistID)
	if err != nil {
		return err
	}
	return c.a.youtube.MoveInPlaylist(ctx, list, track.EntryID, before.EntryID)
}
