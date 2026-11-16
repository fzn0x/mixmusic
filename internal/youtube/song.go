package youtube

import (
	"context"

	"github.com/fzn0x/mixmusic/internal/media"
)

// Song returns the song or video of videoID as YouTube Music tells it: its
// title, artists, album, length, covers and whether it's explicit. A
// room's host reads a guest's song so, rather than trust what the guest
// says of it (#734).
func (c *Client) Song(ctx context.Context, videoID string) (media.Track, error) {
	n, err := c.call(ctx, "next", map[string]any{"videoId": videoID, "isAudioOnly": true}, nil)
	if err != nil {
		return media.Track{}, err
	}
	return songOf(n, videoID)
}

// songOf finds the track of videoID in the queue of a "next" response,
// which starts with it.
func songOf(n node, videoID string) (media.Track, error) {
	for _, t := range parseQueue(n) {
		if t.ID == media.ID(media.SourceYouTube, videoID) {
			return t, nil
		}
	}
	return media.Track{}, ErrNotFound
}
