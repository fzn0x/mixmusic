package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/spotify"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

// ErrNoEpisodeDetails is why an episode has no details to show: its
// service has none of it, and asking again won't find any.
var ErrNoEpisodeDetails = errors.New("no details of the episode")

// EpisodeDetails returns the details of an episode, which the player shows
// where a song shows its lyrics: its show, views, day and length, and its
// description, with its links, times and chapters. They are asked for the
// first time the player shows them, then kept.
func (c *Catalog) EpisodeDetails(ctx context.Context, t media.Track) (media.EpisodeDetails, error) {
	if d, ok := c.a.episodeDetails.get(t.ID); ok {
		return d, nil
	}
	source, id, err := media.ParseID(t.ID)
	if err != nil || !t.IsEpisode() {
		return media.EpisodeDetails{}, detailsError{"This track isn't a podcast's episode.", ErrNoEpisodeDetails}
	}
	var d media.EpisodeDetails
	switch source {
	case media.SourceYouTube:
		d, err = c.a.youtube.EpisodeDetails(ctx, id)
	case media.SourceSpotify:
		if !c.a.spotify.SignedIn() {
			return media.EpisodeDetails{}, detailsError{"Sign in to Spotify in Settings › Accounts to see this episode's details.", errSpotifyShows}
		}
		d, err = c.a.spotify.EpisodeDetails(ctx, id)
	default:
		err = fmt.Errorf("no episode %s: %w", t.ID, ErrNoEpisodeDetails)
	}
	switch {
	case noDetails(err):
		return media.EpisodeDetails{}, detailsError{"No details found for this episode.", errors.Join(ErrNoEpisodeDetails, err)}
	case err != nil:
		log.Printf("mixmusic: the details of %s: %v", t.ID, err)
		return media.EpisodeDetails{}, detailsError{"The episode's details can't be loaded right now. Check your connection, or try again in a moment.", err}
	}
	d.Chapters = media.ChaptersOf(d.Description)
	c.a.episodeDetails.put(t.ID, d)
	return d, nil
}

// noDetails reports whether err is a service's answer that it has no such
// episode, or none of its details.
func noDetails(err error) bool {
	var yt *youtube.APIError
	var sp *spotify.APIError
	return errors.Is(err, youtube.ErrNotFound) || errors.Is(err, ErrNoEpisodeDetails) ||
		errors.As(err, &yt) && yt.Status == http.StatusNotFound || errors.As(err, &sp) && sp.Status == http.StatusNotFound
}

// detailsError is an error of EpisodeDetails, in the interface's words.
type detailsError struct {
	text string
	err  error
}

func (e detailsError) Error() string { return e.text }
func (e detailsError) Unwrap() error { return e.err }
