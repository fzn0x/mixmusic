package app

import (
	"context"
	"errors"
	"log"

	"github.com/fzn0x/mixmusic/internal/library"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/spotify"
)

var errSpotifyOnly = errors.New("only Spotify tracks can be added to a Spotify playlist")

// Collection is the user's library: liked and disliked tracks and
// playlists mixing every service, the history, and their Spotify library.
type Collection struct{ a *App }

// Liked returns the liked tracks, the last liked first.
func (c *Collection) Liked() []media.Track { return c.a.library.Liked() }

// LikedIDs returns the IDs of the liked tracks, for lists to show hearts.
func (c *Collection) LikedIDs() []string {
	liked := c.a.library.Liked()
	ids := make([]string, len(liked))
	for i, t := range liked {
		ids[i] = t.ID
	}
	return ids
}

// SetLiked likes or unlikes a track. Spotify tracks are also saved to, or
// removed from, the user's Spotify library when they signed in, and YouTube
// Music tracks liked or unliked on YouTube Music. Liking a disliked track
// takes the dislike back; the player hears of that from the caller.
func (c *Collection) SetLiked(ctx context.Context, track media.Track, liked bool) error {
	if err := c.a.library.SetLiked(track, liked); err != nil {
		return err
	}
	c.a.syncYouTubeLike(ctx, track, liked)
	c.a.syncSpotifyLike(ctx, track, liked)
	return nil
}

// syncSpotifyLike saves a Spotify track to the user's Spotify library, or
// removes it, when they signed in.
func (a *App) syncSpotifyLike(ctx context.Context, track media.Track, liked bool) {
	if track.Source != media.SourceSpotify || !a.spotify.SignedIn() {
		return
	}
	_, id, _ := media.ParseID(track.ID)
	uri := spotify.URI("track", id)
	var err error
	if liked {
		err = a.spotify.Save(ctx, uri)
	} else {
		err = a.spotify.Remove(ctx, uri)
	}
	if err != nil {
		log.Printf("mixmusic: syncing a like with Spotify: %v", err)
	}
}

// Disliked returns the disliked tracks, the last disliked first.
func (c *Collection) Disliked() []media.Track { return c.a.library.Disliked() }

// SetDisliked dislikes a track, or takes the dislike back, in MixMusic's
// library, and on YouTube Music for its tracks when the user signed in.
// Spotify takes no dislikes: a Spotify track the dislike unliked leaves
// the user's Spotify library, as an unlike does. The player hears of the
// dislike from the caller, which tells it as the user asks.
func (c *Collection) SetDisliked(ctx context.Context, track media.Track, disliked bool) error {
	unliked, err := c.a.library.SetDisliked(track, disliked)
	if err != nil {
		return err
	}
	c.a.syncYouTubeDislike(ctx, track, disliked)
	if unliked {
		c.a.syncSpotifyLike(ctx, track, false)
	}
	return nil
}

// Playlists returns the playlists made in MixMusic, without their tracks.
func (c *Collection) Playlists() []media.Playlist { return c.a.library.Playlists() }

// localID returns the library's ID of a playlist made in MixMusic.
func localID(id string) (string, error) {
	source, native, err := media.ParseID(id)
	if err != nil || source != media.SourceLocal {
		return "", library.ErrNoPlaylist
	}
	return native, nil
}

// CreatePlaylist makes a playlist, holding tracks.
func (c *Collection) CreatePlaylist(title, description string, tracks []media.Track) (media.Playlist, error) {
	return c.a.library.CreatePlaylist(title, description, tracks)
}

// UpdatePlaylist changes the title and the description of a playlist.
func (c *Collection) UpdatePlaylist(id, title, description string) error {
	native, err := localID(id)
	if err != nil {
		return err
	}
	return c.a.library.UpdatePlaylist(native, title, description)
}

// DeletePlaylist deletes a playlist.
func (c *Collection) DeletePlaylist(id string) error {
	native, err := localID(id)
	if err != nil {
		return err
	}
	return c.a.library.DeletePlaylist(native)
}

// AddToPlaylist appends tracks to a playlist made in MixMusic, or to a
// Spotify playlist the user may edit.
func (c *Collection) AddToPlaylist(ctx context.Context, id string, tracks []media.Track) error {
	source, native, err := media.ParseID(id)
	if err != nil {
		return err
	}
	if source == media.SourceSpotify {
		var uris []string
		for _, t := range tracks {
			if t.Source == media.SourceSpotify {
				_, tid, _ := media.ParseID(t.ID)
				uris = append(uris, spotify.URI("track", tid))
			}
		}
		if len(uris) == 0 {
			return errSpotifyOnly
		}
		return c.a.spotify.AddToPlaylist(ctx, native, uris...)
	}
	native, err = localID(id)
	if err != nil {
		return err
	}
	return c.a.library.AddToPlaylist(native, tracks)
}

// RemoveFromPlaylist removes the track at index from a playlist.
func (c *Collection) RemoveFromPlaylist(id string, index int) error {
	native, err := localID(id)
	if err != nil {
		return err
	}
	return c.a.library.RemoveFromPlaylist(native, index)
}

// MoveInPlaylist moves the track at from to the position to.
func (c *Collection) MoveInPlaylist(id string, from, to int) error {
	native, err := localID(id)
	if err != nil {
		return err
	}
	return c.a.library.MoveInPlaylist(native, from, to)
}

// ImportPlaylist copies a playlist of a service into a playlist of
// MixMusic, where tracks of other services can join it.
func (c *Collection) ImportPlaylist(ctx context.Context, id string) (media.Playlist, error) {
	p, err := c.a.playlist(ctx, id)
	if err != nil {
		return media.Playlist{}, err
	}
	return c.a.library.CreatePlaylist(p.Title, p.Description, p.Tracks)
}

// RecordPlay adds a track to the history, once it played long enough to
// count.
func (c *Collection) RecordPlay(track media.Track) error {
	err := c.a.library.RecordPlay(track)
	c.a.reportYouTubePlay(track)
	return err
}

// History returns the last plays, the latest first.
func (c *Collection) History(limit int) []library.Play { return c.a.library.History(limit) }

// ClearHistory forgets what the user played.
func (c *Collection) ClearHistory() error { return c.a.library.ClearHistory() }

// Stats sums up the plays of the last days, or of all time when days is 0.
func (c *Collection) Stats(days int) library.Stats { return c.a.library.Stats(days) }

// Searches returns the recent searches.
func (c *Collection) Searches() []string { return c.a.library.Searches() }

// ClearSearches forgets the recent searches.
func (c *Collection) ClearSearches() error { return c.a.library.ClearSearches() }

// TrackPage is a page of a long list of tracks.
type TrackPage struct {
	Tracks []media.Track `json:"tracks"`
	Total  int           `json:"total"`
}

// SpotifyLikedTracks returns a page of the user's liked songs on Spotify.
func (c *Collection) SpotifyLikedTracks(ctx context.Context, offset int) (TrackPage, error) {
	tracks, total, err := c.a.spotify.SavedTracks(ctx, offset)
	return TrackPage{Tracks: tracks, Total: total}, err
}

// SpotifyPlaylists returns the playlists the user made or follows on
// Spotify.
func (c *Collection) SpotifyPlaylists(ctx context.Context) ([]media.Playlist, error) {
	return c.a.spotify.MyPlaylists(ctx, c.a.spotifyUserID(ctx))
}

// SpotifyAlbums returns the albums the user saved on Spotify.
func (c *Collection) SpotifyAlbums(ctx context.Context) ([]media.Album, error) {
	return c.a.spotify.SavedAlbums(ctx)
}

// SpotifyArtists returns the artists the user follows on Spotify.
func (c *Collection) SpotifyArtists(ctx context.Context) ([]media.Artist, error) {
	return c.a.spotify.FollowedArtists(ctx)
}
