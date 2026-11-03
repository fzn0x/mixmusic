package spotify

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"

	"github.com/fzn0x/mixmusic/internal/media"
)

// episodeLimit is the most episodes of a page of a show's.
const episodeLimit = 50

// show is a podcast of the Web API. Apps in development mode get no
// publisher since February 2026: it is read when Spotify sends one.
type show struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Description   string                 `json:"description"`
	Publisher     string                 `json:"publisher"`
	Images        []image                `json:"images"`
	TotalEpisodes int                    `json:"total_episodes"`
	ExternalURLs  externalURLs           `json:"external_urls"`
	Episodes      *paging[*episodeEntry] `json:"episodes"`
}

// episodeEntry is an episode of the Web API. A show's list of episodes
// leaves the show out of each.
type episodeEntry struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Type        string       `json:"type"`
	DurationMs  int          `json:"duration_ms"`
	Explicit    bool         `json:"explicit"`
	ReleaseDate string       `json:"release_date"`
	Images      []image      `json:"images"`
	External    externalURLs `json:"external_urls"`
	Show        *show        `json:"show"`
	// ResumePoint is where the user left the episode, for a sign-in that
	// may read it (user-read-playback-position).
	ResumePoint *struct {
		FullyPlayed      bool `json:"fully_played"`
		ResumePositionMs int  `json:"resume_position_ms"`
	} `json:"resume_point"`
}

// toPodcast converts a show, with the first page of its episodes when it
// has them.
func toPodcast(s *show) media.Podcast {
	p := media.Podcast{
		ID:           media.ID(media.SourceSpotify, s.ID),
		Source:       media.SourceSpotify,
		Title:        s.Name,
		Authors:      []media.Ref{},
		Description:  s.Description,
		Artwork:      images(s.Images),
		EpisodeCount: s.TotalEpisodes,
		Episodes:     []media.Track{},
		URL:          s.ExternalURLs.Spotify,
	}
	if s.Publisher != "" {
		p.Authors = []media.Ref{{Name: s.Publisher}}
	}
	if s.Episodes != nil {
		page := toEpisodePage(s.Episodes, media.Ref{ID: p.ID, Name: p.Title}, 0)
		p.Episodes, p.More = page.Episodes, page.More
	}
	return p
}

// toEpisode converts an episode; on a show's list, which leaves the show
// out of each, show is the show.
func toEpisode(e *episodeEntry, of media.Ref) (media.Track, bool) {
	if e == nil || e.ID == "" || (e.Type != "" && e.Type != "episode") {
		return media.Track{}, false
	}
	if e.Show != nil && e.Show.ID != "" {
		of = media.Ref{ID: media.ID(media.SourceSpotify, e.Show.ID), Name: e.Show.Name}
	}
	ep := &media.Episode{Show: of, Released: e.ReleaseDate}
	if r := e.ResumePoint; r != nil {
		ep.Played, ep.ResumeMs = r.FullyPlayed, r.ResumePositionMs
	}
	t := media.Track{
		ID:         media.ID(media.SourceSpotify, e.ID),
		Source:     media.SourceSpotify,
		Title:      e.Name,
		Artists:    []media.Ref{},
		DurationMs: e.DurationMs,
		Explicit:   e.Explicit,
		Artwork:    images(e.Images),
		URL:        e.External.Spotify,
		Episode:    ep,
	}
	if of.Name != "" {
		t.Artists = []media.Ref{{Name: of.Name}}
	}
	return t, true
}

// toEpisodePage converts a page of a show's episodes, read from offset,
// and tells the offset of the next.
func toEpisodePage(p *paging[*episodeEntry], of media.Ref, offset int) media.EpisodePage {
	page := media.EpisodePage{Episodes: []media.Track{}}
	for _, e := range p.Items {
		if t, ok := toEpisode(e, of); ok {
			page.Episodes = append(page.Episodes, t)
		}
	}
	if p.Next != "" {
		page.More = strconv.Itoa(offset + len(p.Items))
	}
	return page
}

// SearchPodcasts returns the shows and the episodes Spotify finds for
// query: at most 10 of each, as Spotify answers apps in development mode.
func (c *Client) SearchPodcasts(ctx context.Context, query string) (media.PodcastResults, error) {
	var res struct {
		Shows    paging[*show]         `json:"shows"`
		Episodes paging[*episodeEntry] `json:"episodes"`
	}
	q := url.Values{"q": {query}, "type": {"show,episode"}, "limit": {strconv.Itoa(searchLimit)}}
	if err := c.get(ctx, "/search", q, &res); err != nil {
		return media.PodcastResults{}, err
	}
	out := media.EmptyPodcastResults()
	for _, s := range res.Shows.Items {
		if s != nil && s.ID != "" {
			out.Podcasts = append(out.Podcasts, toPodcast(s).Summary())
		}
	}
	for _, e := range res.Episodes.Items {
		if t, ok := toEpisode(e, media.Ref{}); ok {
			out.Episodes = append(out.Episodes, t)
		}
	}
	return out, nil
}

// Show returns a show with its newest episodes, and what reads the
// episodes after them.
func (c *Client) Show(ctx context.Context, id string) (media.Podcast, error) {
	var s show
	if err := c.get(ctx, "/shows/"+url.PathEscape(id), nil, &s); err != nil {
		return media.Podcast{}, err
	}
	return toPodcast(&s), nil
}

// ShowEpisodes returns the page of a show's episodes that more, a
// podcast's More, reads, and what reads the page after it. of is the
// show, which the episodes name.
func (c *Client) ShowEpisodes(ctx context.Context, of media.Ref, more string) (media.EpisodePage, error) {
	_, id, err := media.ParseID(of.ID)
	if err != nil {
		return media.EpisodePage{}, err
	}
	offset, _ := strconv.Atoi(more)
	var page paging[*episodeEntry]
	q := url.Values{"limit": {strconv.Itoa(episodeLimit)}, "offset": {strconv.Itoa(offset)}}
	if err := c.get(ctx, "/shows/"+url.PathEscape(id)+"/episodes", q, &page); err != nil {
		return media.EpisodePage{}, err
	}
	return toEpisodePage(&page, of, offset), nil
}

// SavedShows returns the shows the user saved, the last saved first.
func (c *Client) SavedShows(ctx context.Context) ([]media.Podcast, error) {
	out := []media.Podcast{}
	next := "/me/shows?limit=" + strconv.Itoa(pageLimit)
	for page := 0; next != "" && page < 4; page++ {
		var res paging[struct {
			Show *show `json:"show"`
		}]
		if err := c.get(ctx, next, nil, &res); err != nil {
			if page == 0 {
				return nil, err
			}
			break
		}
		for _, it := range res.Items {
			if it.Show != nil && it.Show.ID != "" {
				out = append(out, toPodcast(it.Show).Summary())
			}
		}
		next = res.Next
	}
	return out, nil
}

// TrackURI returns the Spotify URI of a Spotify track, or of an episode,
// by its media ID.
func TrackURI(t media.Track) string {
	_, id, _ := media.ParseID(t.ID)
	if t.IsEpisode() {
		return URI("episode", id)
	}
	return URI("track", id)
}

// playingItem is what a device plays, a track or an episode, as the Web
// API tells it to an app that takes episodes.
type playingItem struct{ t *media.Track }

func (p *playingItem) UnmarshalJSON(data []byte) error {
	var kind struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &kind); err != nil {
		return err
	}
	if kind.Type == "episode" {
		var e episodeEntry
		if err := json.Unmarshal(data, &e); err != nil {
			return err
		}
		if t, ok := toEpisode(&e, media.Ref{}); ok {
			p.t = &t
		}
		return nil
	}
	var t track
	if err := json.Unmarshal(data, &t); err != nil {
		return err
	}
	if mt, ok := toTrack(&t, nil); ok {
		p.t = &mt
	}
	return nil
}
