package youtube

// Why YouTube refused an edit of a playlist, and the add of a video a
// playlist holds already, which YouTube Music's website offers to make
// anyway.

import (
	"context"
	"errors"
	"strings"
)

// ErrAlreadyInPlaylist is an add of a video the playlist holds already,
// which YouTube refuses unless it's asked to add it anyway.
var ErrAlreadyInPlaylist = errors.New("YouTube Music did not add it: it's in that playlist already")

// dedupeSkip is the option of an add that skips YouTube's check for a
// video the playlist holds already, which YouTube Music's Add anyway sends.
const dedupeSkip = "DEDUPE_OPTION_SKIP"

// AddToPlaylistAnyway adds a video to a playlist of the user even when it
// holds the video already, a second time, as YouTube Music's Add anyway
// does.
func (c *Client) AddToPlaylistAnyway(ctx context.Context, playlistID, videoID string) error {
	return c.editPlaylist(ctx, playlistID, map[string]any{"action": "ACTION_ADD_VIDEO", "addedVideoId": videoID, "dedupeOption": dedupeSkip},
		errors.New("YouTube Music did not add it"))
}

// refusedEdit tells why YouTube refused an edit it answered, from its
// answer n: an add of a video the playlist holds already, which the answer
// offers to make anyway; or refused, with YouTube's own words after it
// when the answer has some. A playlist of another account's, and a session
// YouTube ended, it answers with a status of HTTP instead (refusedOrEnded).
func refusedEdit(n node, refused error) error {
	if offersAddAnyway(n.v, maxFindDepth) {
		return ErrAlreadyInPlaylist
	}
	if words := refusalWords(n); words != "" {
		return errors.New(refused.Error() + ": " + words)
	}
	return refused
}

// offersAddAnyway reports whether v, an answer to an add, or a part of it,
// offers the same add with YouTube's check for a video already there
// skipped: the Add anyway of the toast, or of a dialog, YouTube answers a
// duplicate with. That, not the toast's words, which are in the account's
// language, tells a duplicate from other refusals.
func offersAddAnyway(v any, depth int) bool {
	if depth <= 0 {
		return false
	}
	switch v := v.(type) {
	case map[string]any:
		if v["action"] == "ACTION_ADD_VIDEO" && v["dedupeOption"] == dedupeSkip {
			return true
		}
		for _, x := range v {
			if offersAddAnyway(x, depth-1) {
				return true
			}
		}
	case []any:
		for _, x := range v {
			if offersAddAnyway(x, depth-1) {
				return true
			}
		}
	}
	return false
}

// refusalWords returns what YouTube says of a refused edit, in its toast or
// its dialog, without the full stop, or "".
func refusalWords(n node) string {
	words := n.find("responseText").text()
	if words == "" {
		for _, m := range n.find("dialogMessages").list() {
			if words = m.text(); words != "" {
				break
			}
		}
	}
	return strings.TrimSuffix(strings.TrimSpace(words), ".")
}
