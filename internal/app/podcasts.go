package app

// Podcasts: shows and their episodes, on YouTube Music and Spotify, the
// shows the user follows, and where they left the episodes.

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/fzn0x/mixmusic/internal/library"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/spotify"
)

// errSpotifyEpisode is why a Spotify episode doesn't play from YouTube
// Music: no recording of it is there.
var errSpotifyEpisode = errors.New("Spotify's podcasts play only from Spotify: choose In MixMusic, from Spotify or On Spotify Connect in Settings › Playback")

// errSpotifyShows is why Spotify's podcasts don't show signed out.
var errSpotifyShows = errors.New("sign in to Spotify in Settings › Accounts to open its podcasts")

// PodcastSearch holds the podcasts and episodes each service found.
type PodcastSearch struct {
	YouTube media.PodcastResults `json:"youtube"`
	Spotify media.PodcastResults `json:"spotify"`
	// SpotifySignedIn tells whether Spotify was searched.
	SpotifySignedIn bool `json:"spotifySignedIn"`
	// Errors are the services' errors, for those that failed.
	Errors []string `json:"errors"`
}

// SearchPodcasts searches YouTube Music's podcasts and episodes, and
// Spotify's when the user signed in.
func (c *Catalog) SearchPodcasts(ctx context.Context, query string) (PodcastSearch, error) {
	query = strings.TrimSpace(query)
	res := PodcastSearch{YouTube: media.EmptyPodcastResults(), Spotify: media.EmptyPodcastResults(),
		SpotifySignedIn: c.a.spotify.SignedIn(), Errors: []string{}}
	if query == "" {
		return res, nil
	}
	var (
		wg           sync.WaitGroup
		ytErr, spErr error
	)
	wg.Go(func() { res.YouTube, ytErr = c.a.youtube.SearchPodcasts(ctx, query) })
	if res.SpotifySignedIn {
		wg.Go(func() { res.Spotify, spErr = c.a.spotify.SearchPodcasts(ctx, query) })
	}
	wg.Wait()
	if ytErr != nil {
		res.YouTube = media.EmptyPodcastResults()
		res.Errors = append(res.Errors, ytErr.Error())
	}
	if spErr != nil {
		res.Spotify = media.EmptyPodcastResults()
		res.Errors = append(res.Errors, spErr.Error())
	}
	if ytErr != nil && (spErr != nil || !res.SpotifySignedIn) {
		return res, ytErr
	}
	return res, nil
}

// Podcast returns a podcast with its newest episodes, and whether the
// user saved it on its service.
func (c *Catalog) Podcast(ctx context.Context, id string) (media.Podcast, error) {
	source, native, err := media.ParseID(id)
	if err != nil {
		return media.Podcast{}, err
	}
	switch source {
	case media.SourceYouTube:
		return c.a.youtube.Podcast(ctx, native)
	case media.SourceSpotify:
		if !c.a.spotify.SignedIn() {
			return media.Podcast{}, errSpotifyShows
		}
		p, err := c.a.spotify.Show(ctx, native)
		if err == nil {
			p.Saved = c.a.spotifyHas(ctx, spotify.URI("show", native))
		}
		return p, err
	}
	return media.Podcast{}, fmt.Errorf("no podcast %s", id)
}

// MoreEpisodes returns the page of a podcast's episodes that more, the
// podcast's More, reads. show is the podcast, which the episodes name.
func (c *Catalog) MoreEpisodes(ctx context.Context, show media.Ref, more string) (media.EpisodePage, error) {
	source, _, err := media.ParseID(show.ID)
	if err != nil {
		return media.EpisodePage{}, err
	}
	switch source {
	case media.SourceYouTube:
		return c.a.youtube.PodcastEpisodes(ctx, show, more)
	case media.SourceSpotify:
		return c.a.spotify.ShowEpisodes(ctx, show, more)
	}
	return media.EpisodePage{}, fmt.Errorf("no podcast %s", show.ID)
}

// SpotifyEpisodesPlay reports whether Spotify's episodes play, as the
// settings play Spotify: from Spotify, not from YouTube Music.
func (c *Catalog) SpotifyEpisodesPlay() bool {
	return c.a.settings.Get().SpotifyPlayback != settings.SpotifyPlaybackMatch
}

// Podcasts returns the podcasts the user follows in MixMusic, the last
// followed first.
func (c *Collection) Podcasts() []media.Podcast { return c.a.library.Podcasts() }

// Follows reports whether the user follows the podcast id in MixMusic.
func (c *Collection) Follows(id string) bool { return c.a.library.Follows(id) }

// SetPodcastFollowed follows a podcast in MixMusic, or stops, and saves it
// to the user's library of its service, or takes it out, when they signed
// in there: that it can't is logged, as for likes.
func (c *Collection) SetPodcastFollowed(ctx context.Context, p media.Podcast, on bool) error {
	if err := c.a.library.SetFollowed(p, on); err != nil {
		return err
	}
	_, native, _ := media.ParseID(p.ID)
	var err error
	switch {
	case p.Source == media.SourceYouTube && c.a.youtube.SignedIn() && p.PlaylistID != "":
		err = c.a.youtube.SetSaved(ctx, p.PlaylistID, on)
	case p.Source == media.SourceSpotify && c.a.spotify.SignedIn():
		err = c.a.spotifySave(ctx, spotify.URI("show", native), on)
	}
	if err != nil {
		log.Printf("mixmusic: saving a podcast on its service: %v", err)
	}
	return nil
}

// YouTubePodcasts returns the podcasts the user saved on YouTube Music.
func (c *Collection) YouTubePodcasts(ctx context.Context) ([]media.Podcast, error) {
	return c.a.youtube.LibraryPodcasts(ctx)
}

// SpotifyPodcasts returns the shows the user saved on Spotify.
func (c *Collection) SpotifyPodcasts(ctx context.Context) ([]media.Podcast, error) {
	return c.a.spotify.SavedShows(ctx)
}

// Position returns where the user left an episode.
func (c *Collection) Position(id string) (library.Position, bool) { return c.a.positions.Get(id) }

// InProgress returns the episodes the user left partway, the latest
// first, at most limit.
func (c *Collection) InProgress(limit int) []library.Position {
	return c.a.positions.InProgress(limit)
}

// PositionsVersion counts the changes of where episodes were left.
func (c *Collection) PositionsVersion() uint64 { return c.a.positions.Version() }

// SetPlayed marks an episode played, or not played, which forgets where
// it was left.
func (c *Collection) SetPlayed(t media.Track, played bool) { c.a.positions.SetPlayed(t, played) }

// keepPosition keeps where the episode that plays is, as the player
// reports it each second: not while it loads, at where it starts.
func (a *App) keepPosition(np NowPlaying) {
	if np.Track == nil || !np.Track.IsEpisode() || np.Live || np.Loading {
		return
	}
	a.positions.Keep(*np.Track, time.Duration(np.PositionMs)*time.Millisecond, time.Duration(np.DurationMs)*time.Millisecond)
}

// ResumeAt is where an episode plays from as it becomes current.
func (s playerServices) ResumeAt(t media.Track) time.Duration { return s.a.positions.ResumeAt(t) }
