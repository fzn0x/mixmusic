package app

import (
	"context"
	"slices"

	"github.com/fzn0x/mixmusic/internal/media"
)

// PlaylistSuggestions are songs a service suggests adding to a playlist of
// the user's, a few at a time.
type PlaylistSuggestions struct {
	Tracks []media.Track
	// Refresh is the token of the next few, "" when there are none.
	Refresh string
}

// PlaylistSuggestions returns the songs YouTube Music suggests adding to a
// playlist of the user's there, by token: the playlist's Suggest, or the
// Refresh of the suggestions before, for the next ones. The songs the user
// dislikes stay out.
func (c *Collection) PlaylistSuggestions(ctx context.Context, token string) (PlaylistSuggestions, error) {
	if !c.a.youtube.SignedIn() {
		return PlaylistSuggestions{}, errNotSignedIn
	}
	tracks, refresh, err := c.a.youtube.PlaylistSuggestions(ctx, token)
	if err != nil {
		return PlaylistSuggestions{}, err
	}
	tracks = slices.DeleteFunc(tracks, func(t media.Track) bool { return c.a.IsDisliked(t.ID) })
	return PlaylistSuggestions{Tracks: tracks, Refresh: refresh}, nil
}
