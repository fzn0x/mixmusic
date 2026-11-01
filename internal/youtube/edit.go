package youtube

import (
	"context"
	"errors"
	"strings"
)

// newPlaylistPrivacy keeps a playlist made in MixMusic to the account that
// made it: the user shares it from YouTube Music when they want to.
const newPlaylistPrivacy = "PRIVATE"

// CreatePlaylist makes a playlist on YouTube Music named title, private to
// the account, holding the videos given, and returns its playlist ID.
func (c *Client) CreatePlaylist(ctx context.Context, title string, videoIDs []string) (string, error) {
	if err := c.session(); err != nil {
		return "", err
	}
	title = strings.TrimSpace(title)
	switch {
	case title == "":
		return "", errors.New("a playlist needs a name")
	case strings.ContainsAny(title, "<>"):
		return "", errors.New("a playlist's name cannot hold < or >")
	}
	body := map[string]any{"title": title, "description": "", "privacyStatus": newPlaylistPrivacy}
	// The first tracks go with the playlist itself: YouTube turns down
	// edits of a playlist made a moment ago.
	if ids := uniqueIDs(videoIDs); len(ids) > 0 {
		body["videoIds"] = ids
	}
	n, err := c.call(ctx, "playlist/create", body, nil)
	if err != nil {
		return "", err
	}
	id := n.get("playlistId").str()
	if id == "" {
		return "", errRefused
	}
	return id, nil
}

// AddToPlaylist adds a video to a playlist of the user.
func (c *Client) AddToPlaylist(ctx context.Context, playlistID, videoID string) error {
	return c.editPlaylist(ctx, playlistID, map[string]any{"action": "ACTION_ADD_VIDEO", "addedVideoId": videoID},
		errors.New("YouTube Music did not add it: it may be in that playlist already"))
}

// RemoveFromPlaylist takes a video out of a playlist of the user. entryID
// names its place in the playlist, so that of a track there twice, one
// goes.
func (c *Client) RemoveFromPlaylist(ctx context.Context, playlistID, entryID, videoID string) error {
	if entryID == "" {
		return errors.New("YouTube Music gave no place in the playlist for this track: open the playlist again")
	}
	return c.editPlaylist(ctx, playlistID, map[string]any{"action": "ACTION_REMOVE_VIDEO", "setVideoId": entryID, "removedVideoId": videoID},
		errors.New("YouTube Music did not take it out of the playlist"))
}

// editPlaylist applies one action to a playlist. YouTube answers a refused
// edit with a status that does not say it succeeded, and refused; an edit
// of a playlist that isn't the account's, with 401 or 403, which leaves
// the session as it was.
func (c *Client) editPlaylist(ctx context.Context, playlistID string, action map[string]any, refused error) error {
	if err := c.session(); err != nil {
		return err
	}
	playlistID = strings.TrimPrefix(playlistID, "VL")
	if playlistID == "" {
		return errors.New("no playlist given")
	}
	n, err := c.call(ctx, "browse/edit_playlist", map[string]any{"playlistId": playlistID, "actions": []any{action}}, nil)
	if err != nil {
		return c.refusedOrEnded(ctx, err, errNotYours)
	}
	if !strings.Contains(n.get("status").str(), "SUCCEEDED") {
		return refused
	}
	return nil
}

// errRefused is a write YouTube answered without making it, as it does for
// an account that must first set up a channel.
var errRefused = errors.New("YouTube Music refused: the account may need a channel first, on youtube.com")

// errNotYours is an edit of a playlist YouTube refused, while the session
// still works.
var errNotYours = errors.New("YouTube Music refused: the playlist may be another account's")

// uniqueIDs returns ids without the empty ones and the repeats.
func uniqueIDs(ids []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, id := range ids {
		if id = strings.TrimSpace(id); id != "" && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}
