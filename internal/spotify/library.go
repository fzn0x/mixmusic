package spotify

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
)

// Save adds items to the user's library, by their Spotify URIs: tracks to
// their liked songs, albums to their albums, artists to those they follow.
func (c *Client) Save(ctx context.Context, uris ...string) error {
	return c.do(ctx, http.MethodPut, "/me/library", url.Values{"uris": {joinURIs(uris)}}, nil, nil)
}

// Remove takes items out of the user's library, by their Spotify URIs.
func (c *Client) Remove(ctx context.Context, uris ...string) error {
	return c.do(ctx, http.MethodDelete, "/me/library", url.Values{"uris": {joinURIs(uris)}}, nil, nil)
}

// Contains reports, for each URI, whether the item is in the user's
// library.
func (c *Client) Contains(ctx context.Context, uris ...string) ([]bool, error) {
	var out []bool
	err := c.get(ctx, "/me/library/contains", url.Values{"uris": {joinURIs(uris)}}, &out)
	return out, err
}

// AddToPlaylist appends tracks to a playlist the user may edit.
func (c *Client) AddToPlaylist(ctx context.Context, playlistID string, uris ...string) error {
	return refusedChange(c.do(ctx, http.MethodPost, "/playlists/"+url.PathEscape(playlistID)+"/items", nil, map[string]any{"uris": uris}, nil))
}

// FollowPlaylist follows a playlist, which saves it to the user's library,
// or unfollows it. It goes through /me/library as Save and Remove do, which
// every Client ID has, and falls back to the route Spotify removed in
// February 2026, which a Client ID made before then still answers, when
// the new one answers 403 or 404.
func (c *Client) FollowPlaylist(ctx context.Context, playlistID string, on bool) error {
	_, err := c.followPlaylist(ctx, playlistID, on)
	return err
}

// The routes FollowPlaylist tries, named for the live test.
const (
	followViaLibrary   = "/me/library"
	followViaFollowers = "/playlists/{id}/followers"
)

// followPlaylist is FollowPlaylist, and says which route answered.
func (c *Client) followPlaylist(ctx context.Context, playlistID string, on bool) (route string, err error) {
	method := http.MethodDelete
	if on {
		method = http.MethodPut
	}
	uri := URI("playlist", playlistID)
	err = c.do(ctx, method, "/me/library", url.Values{"uris": {uri}}, nil, nil)
	if !refusedRoute(err) {
		return followViaLibrary, err
	}
	old := c.do(ctx, method, "/playlists/"+url.PathEscape(playlistID)+"/followers", nil, nil, nil)
	if old == nil {
		return followViaFollowers, nil
	}
	return "", fmt.Errorf("spotify: the playlist was not %s: %s answered %w, and %s answered %w",
		map[bool]string{true: "saved", false: "removed"}[on], followViaLibrary, err, followViaFollowers, old)
}

// refusedRoute reports whether err is Spotify answering 403 or 404 to a
// route, as it does one a Client ID has no access to.
func refusedRoute(err error) bool {
	var api *APIError
	return errors.As(err, &api) && (api.Status == http.StatusForbidden || api.Status == http.StatusNotFound)
}
