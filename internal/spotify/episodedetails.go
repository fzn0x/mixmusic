package spotify

import (
	"context"
	"net/url"
	"strings"

	"github.com/fzn0x/mixmusic/internal/media"
)

// episodeDetails is an episode of GET /episodes/{id}, with its
// description: plain text, and HTML.
type episodeDetails struct {
	episodeEntry
	Description     string `json:"description"`
	HTMLDescription string `json:"html_description"`
}

// EpisodeDetails returns the details of the episode id: its show, day and
// length, and its description, plain text whose web addresses and times,
// as 12:34, become links and times. Spotify tells no plays.
func (c *Client) EpisodeDetails(ctx context.Context, id string) (media.EpisodeDetails, error) {
	var e episodeDetails
	if err := c.get(ctx, "/episodes/"+url.PathEscape(id), nil, &e); err != nil {
		return media.EpisodeDetails{}, err
	}
	text := e.Description
	if strings.TrimSpace(text) == "" {
		text = media.PlainText(e.HTMLDescription)
	}
	d := media.EpisodeDetails{Released: e.ReleaseDate, DurationMs: e.DurationMs, Description: media.ParseDescription(text)}
	if s := e.Show; s != nil && s.ID != "" {
		d.Show = media.Ref{ID: media.ID(media.SourceSpotify, s.ID), Name: s.Name}
	}
	return d, nil
}
