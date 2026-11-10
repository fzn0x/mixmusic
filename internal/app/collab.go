package app

// Collaborating on playlists: inviting collaborators to a playlist of the
// user's on YouTube Music, making one of theirs on Spotify collaborative,
// moving the tracks of a Spotify playlist they may change, and making a
// mix a playlist on YouTube Music to share it.

import (
	"context"
	"errors"
	"fmt"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/spotify"
)

// errNotSpotify is a change of a Spotify playlist asked of another.
var errNotSpotify = errors.New("that is not a playlist of Spotify")

// errSignInToCollaborate is a change of a playlist of a service the user
// isn't signed in to.
var errSignInToCollaborate = errors.New("sign in to that service in Settings › Accounts to change its playlists")

// InviteYouTubeCollaborators turns YouTube Music's collaboration on for a
// playlist the user made there, by its MixMusic ID, and returns its invite
// link. Collaborators can't open a private playlist: unlist makes it
// unlisted first, which the user agreed to.
func (c *Collection) InviteYouTubeCollaborators(ctx context.Context, playlistID string, unlist bool) (string, error) {
	if !c.a.youtube.SignedIn() {
		return "", errSignInToCollaborate
	}
	list, err := youtubePlaylistID(playlistID)
	if err != nil {
		return "", err
	}
	if unlist {
		if err := c.a.youtube.SetPlaylistPrivacy(ctx, list, media.Unlisted); err != nil {
			return "", err
		}
	}
	link, err := c.a.youtube.InviteCollaborators(ctx, list)
	if err != nil && unlist {
		err = fmt.Errorf("%w; the playlist is unlisted now", err)
	}
	return link, err
}

// StopYouTubeCollaboration turns YouTube Music's collaboration off for a
// playlist the user made there, by its MixMusic ID.
func (c *Collection) StopYouTubeCollaboration(ctx context.Context, playlistID string) error {
	if !c.a.youtube.SignedIn() {
		return errSignInToCollaborate
	}
	list, err := youtubePlaylistID(playlistID)
	if err != nil {
		return err
	}
	return c.a.youtube.StopCollaboration(ctx, list)
}

// spotifyPlaylistID returns the playlist ID of a playlist of Spotify, by
// its MixMusic ID, once the user signed in to Spotify.
func (c *Collection) spotifyPlaylistID(id string) (string, error) {
	if !c.a.spotify.SignedIn() {
		return "", errSignInToCollaborate
	}
	source, native, err := media.ParseID(id)
	if err != nil {
		return "", err
	}
	if source != media.SourceSpotify {
		return "", errNotSpotify
	}
	return native, nil
}

// SetSpotifyCollaborative makes a playlist the user owns on Spotify, by its
// MixMusic ID, collaborative, or theirs alone again. Made collaborative,
// it leaves their profile, as Spotify keeps collaborative playlists.
func (c *Collection) SetSpotifyCollaborative(ctx context.Context, playlistID string, on bool) error {
	native, err := c.spotifyPlaylistID(playlistID)
	if err != nil {
		return err
	}
	return c.a.spotify.SetCollaborative(ctx, native, on)
}

// MoveInSpotifyPlaylist moves the track at from of a playlist of Spotify
// the user may change, by its MixMusic ID, to the place to.
func (c *Collection) MoveInSpotifyPlaylist(ctx context.Context, playlistID string, from, to int) error {
	native, err := c.spotifyPlaylistID(playlistID)
	if err != nil {
		return err
	}
	return c.a.spotify.MoveInPlaylist(ctx, native, from, to)
}

// AddToSpotifyPlaylist appends Spotify tracks to a playlist of Spotify the
// user may change, by its MixMusic ID: theirs, or one they collaborate on.
func (c *Collection) AddToSpotifyPlaylist(ctx context.Context, playlistID string, tracks []media.Track) error {
	native, err := c.spotifyPlaylistID(playlistID)
	if err != nil {
		return err
	}
	var uris []string
	for _, t := range tracks {
		if t.Source == media.SourceSpotify {
			uris = append(uris, spotify.TrackURI(t))
		}
	}
	if len(uris) == 0 {
		return errSpotifyOnly
	}
	return c.a.spotify.AddToPlaylist(ctx, native, uris...)
}

// ShareOnYouTube makes a playlist on YouTube Music of tracks of any
// service, as CreateYouTubePlaylist does, that anyone with its link can
// open, to share it: it shows on none of the account's pages.
func (c *Collection) ShareOnYouTube(ctx context.Context, title string, tracks []media.Track) (media.Playlist, error) {
	return c.createOnYouTube(ctx, title, tracks, media.Unlisted)
}
