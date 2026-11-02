package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/spotify"
)

// errNotSignedIn is returned for following or saving on a service the user
// is not signed in to.
var errNotSignedIn = errors.New("sign in to that service in Settings › Accounts to keep it in your library there")

// YouTubeLikedTracks returns the user's liked songs on YouTube Music.
func (c *Collection) YouTubeLikedTracks(ctx context.Context) ([]media.Track, error) {
	p, err := c.a.youtube.LikedSongs(ctx)
	if err != nil {
		return nil, err
	}
	return p.Tracks, nil
}

// YouTubePlaylists returns the playlists the user made or saved on YouTube
// Music. Those the user made are Editable, for tracks to go into them: the
// ones YouTube offers to delete, and the others of the account's name.
func (c *Collection) YouTubePlaylists(ctx context.Context) ([]media.Playlist, error) {
	playlists, err := c.a.youtube.LibraryPlaylists(ctx)
	if len(playlists) > 0 {
		if me := c.a.youtubeAccount(ctx).Account; me != nil {
			markOwn(playlists, me.Name)
		}
	}
	return playlists, err
}

// markOwn marks Editable the playlists of the library that are the user's
// by their owner's name: the account's, as the library names the owner,
// or none. It leaves those YouTube says the user saved, which are someone
// else's, and its automatic ones, which aren't PL….
func markOwn(playlists []media.Playlist, account string) {
	for i := range playlists {
		p := &playlists[i]
		_, id, _ := media.ParseID(p.ID)
		if !p.Editable && !p.Saved && account != "" && strings.HasPrefix(id, "PL") &&
			(p.Owner == "" || strings.EqualFold(p.Owner, account)) {
			p.Editable = true
		}
	}
}

// YouTubeAlbums returns the albums the user saved on YouTube Music.
func (c *Collection) YouTubeAlbums(ctx context.Context) ([]media.Album, error) {
	return c.a.youtube.LibraryAlbums(ctx)
}

// YouTubeArtists returns the artists the user subscribes to on YouTube
// Music.
func (c *Collection) YouTubeArtists(ctx context.Context) ([]media.Artist, error) {
	return c.a.youtube.LibraryArtists(ctx)
}

// syncYouTubeLike likes or unlikes a YouTube Music track on YouTube Music
// too, when the user signed in.
func (a *App) syncYouTubeLike(ctx context.Context, track media.Track, liked bool) {
	if track.Source != media.SourceYouTube || !a.youtube.SignedIn() {
		return
	}
	_, id, _ := media.ParseID(track.ID)
	if err := a.youtube.SetLiked(ctx, id, liked); err != nil {
		log.Printf("mixmusic: syncing a like with YouTube Music: %v", err)
	}
}

// SetFollowing follows or unfollows an artist on its service: subscribes
// on YouTube Music, follows on Spotify.
func (c *Collection) SetFollowing(ctx context.Context, artistID string, on bool) error {
	source, native, err := media.ParseID(artistID)
	if err != nil {
		return err
	}
	switch source {
	case media.SourceYouTube:
		if !c.a.youtube.SignedIn() {
			return errNotSignedIn
		}
		return c.a.youtube.SetSubscribed(ctx, native, on)
	case media.SourceSpotify:
		return c.a.spotifySave(ctx, spotify.URI("artist", native), on)
	}
	return fmt.Errorf("artists of %s cannot be followed", source)
}

// SetSaved saves an album or a playlist to the user's library on its
// service, or takes it out. kind is "album" or "playlist"; playlistID is
// the playlist of a YouTube Music album's tracks.
func (c *Collection) SetSaved(ctx context.Context, kind, id, playlistID string, on bool) error {
	source, native, err := media.ParseID(id)
	if err != nil {
		return err
	}
	switch source {
	case media.SourceYouTube:
		if !c.a.youtube.SignedIn() {
			return errNotSignedIn
		}
		if kind == "album" {
			if playlistID == "" {
				return errors.New("this album cannot be saved: YouTube Music gave no playlist of its tracks")
			}
			native = playlistID
		}
		return c.a.youtube.SetSaved(ctx, native, on)
	case media.SourceSpotify:
		if kind == "playlist" {
			return c.a.spotifyFollowPlaylist(ctx, native, on)
		}
		return c.a.spotifySave(ctx, spotify.URI("album", native), on)
	}
	return fmt.Errorf("%s of %s cannot be saved", kind, source)
}

func (a *App) spotifySave(ctx context.Context, uri string, on bool) error {
	if !a.spotify.SignedIn() {
		return errNotSignedIn
	}
	if on {
		return a.spotify.Save(ctx, uri)
	}
	return a.spotify.Remove(ctx, uri)
}

func (a *App) spotifyFollowPlaylist(ctx context.Context, id string, on bool) error {
	if !a.spotify.SignedIn() {
		return errNotSignedIn
	}
	return a.spotify.FollowPlaylist(ctx, id, on)
}

// spotifyHas reports whether an item, by its Spotify URI, is in the user's
// Spotify library; false when they did not sign in or Spotify did not say.
func (a *App) spotifyHas(ctx context.Context, uri string) bool {
	if !a.spotify.SignedIn() {
		return false
	}
	has, err := a.spotify.Contains(ctx, uri)
	return err == nil && len(has) == 1 && has[0]
}
