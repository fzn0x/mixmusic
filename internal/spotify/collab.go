package spotify

// Changing playlists: their owner makes one collaborative, as Spotify's own
// apps do, and its owner and, once it's collaborative, those who follow it
// add to it and move its tracks.

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
)

// ErrPlaylistRefused is a change of a playlist that Spotify refused, for an
// account it lets in: one of another account's that isn't collaborative,
// or a change only the owner may make.
var ErrPlaylistRefused = errors.New("spotify: Spotify refused to change the playlist")

// SetCollaborative makes a playlist the user owns collaborative, which
// those who follow it may then add to and put in order, or makes it theirs
// alone again. Spotify keeps collaborative playlists off their owner's
// profile: one made collaborative leaves it.
func (c *Client) SetCollaborative(ctx context.Context, playlistID string, on bool) error {
	body := map[string]any{"collaborative": on}
	if on {
		body["public"] = false
	}
	return refusedChange(c.do(ctx, http.MethodPut, "/playlists/"+url.PathEscape(playlistID), nil, body, nil))
}

// MoveInPlaylist moves the item at from of a playlist the user may change
// to the place to, both counted from 0 as the playlist holds its items.
func (c *Client) MoveInPlaylist(ctx context.Context, playlistID string, from, to int) error {
	if from < 0 || to < 0 {
		return errors.New("spotify: no such place in the playlist")
	}
	if from == to {
		return nil
	}
	// Spotify takes the place of the item that the moved one goes before,
	// as the playlist was: past to, when it moves down.
	before := to
	if to > from {
		before = to + 1
	}
	body := map[string]any{"range_start": from, "insert_before": before, "range_length": 1}
	return refusedChange(c.do(ctx, http.MethodPut, "/playlists/"+url.PathEscape(playlistID)+"/items", nil, body, nil))
}

// playlistRefusal is a 403 of a change of a playlist, which isn't one of
// the refusals of an app in development mode: those tell what to do
// already.
type playlistRefusal struct{ api *APIError }

func (e *playlistRefusal) Error() string {
	msg := "spotify: Spotify refused to change the playlist: only its owner can make it collaborative, " +
		"and others can add to it or move its tracks only once it's collaborative"
	if e.api.Message != "" {
		msg += fmt.Sprintf(" (Spotify says: %s)", e.api.Message)
	}
	return msg
}

func (e *playlistRefusal) Unwrap() []error { return []error{ErrPlaylistRefused, e.api} }

// refusedChange is err, the error of a change of a playlist, as a
// playlistRefusal when Spotify refused it with 403 for a reason other than
// those of an app in development mode.
func refusedChange(err error) error {
	var api *APIError
	if !errors.As(err, &api) || api.Status != http.StatusForbidden || api.Is(ErrUnregistered) || api.Is(ErrOwnerWithoutPremium) {
		return err
	}
	return &playlistRefusal{api}
}
