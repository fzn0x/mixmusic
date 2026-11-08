package youtube

import (
	"context"
	"errors"
)

// MoveInPlaylist moves a video of a playlist of the user, by entryID, its
// place in the playlist, to just before the entry before, or to the end of
// the playlist when before is "", as YouTube Music's website does when a
// track is dragged there.
func (c *Client) MoveInPlaylist(ctx context.Context, playlistID, entryID, before string) error {
	if entryID == "" {
		return errors.New("YouTube Music gave no place in the playlist for this track: open the playlist again")
	}
	action := map[string]any{"action": "ACTION_MOVE_VIDEO_BEFORE", "setVideoId": entryID}
	if before != "" {
		action["movedSetVideoIdSuccessor"] = before
	}
	return c.editPlaylist(ctx, playlistID, action, errors.New("YouTube Music did not move it in the playlist"))
}
