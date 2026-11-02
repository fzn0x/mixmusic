package youtube

import (
	"context"
	"errors"
	"sync"

	"github.com/fzn0x/mixmusic/internal/media"
)

// Filter narrows a search to one kind of item.
type Filter string

const (
	FilterSongs     Filter = "songs"
	FilterVideos    Filter = "videos"
	FilterAlbums    Filter = "albums"
	FilterArtists   Filter = "artists"
	FilterPlaylists Filter = "playlists"
	// FilterPodcasts finds shows, and FilterEpisodes their episodes.
	FilterPodcasts Filter = "podcasts"
	FilterEpisodes Filter = "episodes"
)

// filterParams are the opaque search parameters of each filter, as the
// website sends them.
var filterParams = map[Filter]string{
	FilterSongs:     "EgWKAQIIAWoKEAkQBRAKEAMQBA%3D%3D",
	FilterVideos:    "EgWKAQIQAWoKEAkQChAFEAMQBA%3D%3D",
	FilterAlbums:    "EgWKAQIYAWoKEAkQChAFEAMQBA%3D%3D",
	FilterArtists:   "EgWKAQIgAWoKEAkQChAFEAMQBA%3D%3D",
	FilterPlaylists: "EgeKAQQoAEABagoQCRAKEAUQAxAE",
	FilterPodcasts:  "EgWKAQJQAWoKEAkQBRAKEAMQBA%3D%3D",
	FilterEpisodes:  "EgWKAQJIAWoKEAkQBRAKEAMQBA%3D%3D",
}

// ErrUnknownFilter is returned for filters YouTube Music has none of.
var ErrUnknownFilter = errors.New("youtube music: unknown search filter")

// Search returns what YouTube Music finds for query, of the kind filter
// selects.
func (c *Client) Search(ctx context.Context, query string, filter Filter) ([]media.Item, error) {
	params, ok := filterParams[filter]
	if !ok {
		return nil, ErrUnknownFilter
	}
	n, err := c.call(ctx, "search", map[string]any{"query": query, "params": params}, nil)
	if err != nil {
		return nil, err
	}
	return parseSearch(n), nil
}

// parseSearch reads the items of every shelf of a search response.
func parseSearch(n node) []media.Item {
	items := []media.Item{}
	sections := n.get("contents", "tabbedSearchResultsRenderer", "tabs", 0, "tabRenderer", "content", "sectionListRenderer", "contents")
	for _, s := range sections.list() {
		if shelf := s.get("musicShelfRenderer"); shelf.exists() {
			items = append(items, parseItems(shelf.get("contents").list())...)
		}
	}
	return items
}

// SearchAll searches songs, albums, artists and playlists at once. It fails
// only when every one of the searches fails.
func (c *Client) SearchAll(ctx context.Context, query string) (media.SearchResults, error) {
	filters := []Filter{FilterSongs, FilterAlbums, FilterArtists, FilterPlaylists}
	found := make([][]media.Item, len(filters))
	errs := make([]error, len(filters))
	var wg sync.WaitGroup
	for i, f := range filters {
		wg.Go(func() { found[i], errs[i] = c.Search(ctx, query, f) })
	}
	wg.Wait()

	results := media.SearchResults{Tracks: []media.Track{}, Albums: []media.Album{}, Artists: []media.Artist{}, Playlists: []media.Playlist{}}
	failed := 0
	for i := range filters {
		if errs[i] != nil {
			failed++
			continue
		}
		for _, item := range found[i] {
			switch item.Kind {
			case media.ItemTrack:
				results.Tracks = append(results.Tracks, *item.Track)
			case media.ItemAlbum:
				results.Albums = append(results.Albums, *item.Album)
			case media.ItemArtist:
				results.Artists = append(results.Artists, *item.Artist)
			case media.ItemPlaylist:
				results.Playlists = append(results.Playlists, *item.Playlist)
			}
		}
	}
	if failed == len(filters) {
		return results, errors.Join(errs...)
	}
	return results, nil
}

// Suggestions returns the searches YouTube Music suggests for what the
// user typed so far.
func (c *Client) Suggestions(ctx context.Context, input string) ([]string, error) {
	n, err := c.call(ctx, "music/get_search_suggestions", map[string]any{"input": input}, nil)
	if err != nil {
		return nil, err
	}
	return parseSuggestions(n), nil
}

func parseSuggestions(n node) []string {
	out := []string{}
	for _, section := range n.get("contents").list() {
		for _, c := range section.get("searchSuggestionsSectionRenderer", "contents").list() {
			s := c.get("searchSuggestionRenderer")
			if !s.exists() {
				continue
			}
			q := s.get("navigationEndpoint", "searchEndpoint", "query").str()
			if q == "" {
				q = s.get("suggestion").text()
			}
			if q != "" {
				out = append(out, q)
			}
		}
	}
	return out
}
