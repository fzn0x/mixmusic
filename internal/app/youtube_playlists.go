package app

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

// youtubeVideoID returns the video of a track on YouTube Music: its own, or
// that of its recording there for a Spotify track.
func (a *App) youtubeVideoID(ctx context.Context, t media.Track) (string, error) {
	if t.Source != media.SourceYouTube {
		rec, err := a.matcher.Match(ctx, t)
		if err != nil {
			return "", err
		}
		t = rec
	}
	_, id, err := media.ParseID(t.ID)
	return id, err
}

// youtubePlaylistID returns the playlist ID of a playlist of YouTube Music,
// by its MixMusic ID.
func youtubePlaylistID(id string) (string, error) {
	source, native, err := media.ParseID(id)
	if err != nil {
		return "", err
	}
	if source != media.SourceYouTube {
		return "", errors.New("that is not a playlist of YouTube Music")
	}
	return native, nil
}

// CreateYouTubePlaylist makes a playlist on YouTube Music, private to the
// account, of tracks of any service: Spotify tracks go in as their
// recordings on YouTube Music, and those without one stay out. The
// playlist is the user's, to add to and to delete.
func (c *Collection) CreateYouTubePlaylist(ctx context.Context, title string, tracks []media.Track) (media.Playlist, error) {
	return c.createOnYouTube(ctx, title, tracks, media.Private)
}

// createOnYouTube is CreateYouTubePlaylist of a playlist that privacy says
// who may open.
func (c *Collection) createOnYouTube(ctx context.Context, title string, tracks []media.Track, privacy media.Privacy) (media.Playlist, error) {
	if !c.a.youtube.SignedIn() {
		return media.Playlist{}, errNotSignedIn
	}
	var ids []string
	for _, t := range tracks {
		if id, err := c.a.youtubeVideoID(ctx, t); err == nil {
			ids = append(ids, id)
		}
	}
	if len(tracks) > 0 && len(ids) == 0 {
		return media.Playlist{}, errors.New("none of these tracks is on YouTube Music")
	}
	id, err := c.a.youtube.CreatePlaylistAs(ctx, title, ids, privacy)
	if err != nil {
		return media.Playlist{}, err
	}
	return media.Playlist{
		ID:         media.ID(media.SourceYouTube, id),
		Source:     media.SourceYouTube,
		Title:      strings.TrimSpace(title),
		TrackCount: len(ids),
		Tracks:     []media.Track{},
		Artwork:    []media.Image{},
		Editable:   true,
		Deletable:  true,
		Mine:       true,
		Privacy:    privacy,
		URL:        "https://music.youtube.com/playlist?list=" + id,
	}, nil
}

// AddToYouTubePlaylist adds a track of any service to a playlist of the
// user on YouTube Music. One the playlist holds already YouTube refuses,
// which is ErrAlreadyInPlaylist: AddToYouTubePlaylistAnyway adds it again.
func (c *Collection) AddToYouTubePlaylist(ctx context.Context, playlistID string, track media.Track) error {
	return c.addToYouTube(ctx, playlistID, track, c.a.youtube.AddToPlaylist)
}

// addToYouTube adds a track of any service to a playlist of the user on
// YouTube Music with add, by its video there.
func (c *Collection) addToYouTube(ctx context.Context, playlistID string, track media.Track, add func(ctx context.Context, list, video string) error) error {
	if !c.a.youtube.SignedIn() {
		return errNotSignedIn
	}
	list, err := youtubePlaylistID(playlistID)
	if err != nil {
		return err
	}
	video, err := c.a.youtubeVideoID(ctx, track)
	if err != nil {
		return err
	}
	if err = add(ctx, list, video); errors.Is(err, youtube.ErrAlreadyInPlaylist) {
		return ErrAlreadyInPlaylist
	}
	return err
}

// RemoveFromYouTubePlaylist takes a track, as read from the playlist, out
// of a playlist of the user on YouTube Music.
func (c *Collection) RemoveFromYouTubePlaylist(ctx context.Context, playlistID string, track media.Track) error {
	if !c.a.youtube.SignedIn() {
		return errNotSignedIn
	}
	list, err := youtubePlaylistID(playlistID)
	if err != nil {
		return err
	}
	_, video, err := media.ParseID(track.ID)
	if err != nil {
		return err
	}
	return c.a.youtube.RemoveFromPlaylist(ctx, list, track.EntryID, video)
}

// DeleteYouTubePlaylist deletes a playlist the user made on YouTube Music,
// by its MixMusic ID: it goes from YouTube too. Mixes copied from it keep
// their tracks, and the queue its.
func (c *Collection) DeleteYouTubePlaylist(ctx context.Context, playlistID string) error {
	if !c.a.youtube.SignedIn() {
		return errNotSignedIn
	}
	list, err := youtubePlaylistID(playlistID)
	if err != nil {
		return err
	}
	return c.a.youtube.DeletePlaylist(ctx, list)
}

// reportYouTubePlay adds a play of a YouTube Music track to the user's
// history there, when they signed in and keep it on.
func (a *App) reportYouTubePlay(t media.Track) {
	if t.Source != media.SourceYouTube || !a.settings.Get().YouTubeHistory || !a.youtube.SignedIn() {
		return
	}
	_, id, err := media.ParseID(t.ID)
	if err != nil {
		return
	}
	// An episode's play goes with the reports of how much of it is heard.
	if t.IsEpisode() {
		a.countWatchtime(t)
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := a.youtube.ReportPlay(ctx, id); err != nil && !errors.Is(err, youtube.ErrSessionRejected) {
			log.Printf("mixmusic: adding a play to the YouTube Music history: %v", err)
		}
	}()
}
