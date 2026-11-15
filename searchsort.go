package main

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
)

// The orders of a search's results (#630). Best match is the services'
// own; Most played counts what YouTube Music says of a song, a video or a
// playlist; Newest orders albums by their year.
const (
	sortBestMatch  = "Best match"
	sortMostPlayed = "Most played"
	sortNewest     = "Newest"
)

// searchSorting is the order chosen for the results of the search query:
// one for the pivots that list songs, videos and playlists, one for the
// albums.
type searchSorting struct {
	query           string
	general, albums string
}

// searchSortOptions are the orders the pivot offers, or none where only
// the services' own is there: Spotify gives apps no play counts, so with
// only Spotify chosen, Most played isn't offered. Artists and podcasts
// have no order to choose.
func searchSortOptions(pivot string, scope int) []string {
	switch pivot {
	case "albums":
		return []string{sortBestMatch, sortNewest}
	case "all", "songs", "videos", "playlists":
		if scope != 1 {
			return []string{sortBestMatch, sortMostPlayed}
		}
	}
	return nil
}

// sortSelect is the Sort by select, beside Services, for the options of
// the pivot, and what it chose. The choices go back to Best match on each
// new search, and the shared one where only Spotify is chosen, which has
// no counts; the pivots with no order (artists, podcasts) show no select
// and keep them.
func (s *shell) sortSelect(c *ui.Context, q, pivot string, scope int) string {
	st := &s.searchSort
	if st.query != q {
		*st = searchSorting{query: q}
	}
	options := searchSortOptions(pivot, scope)
	if len(options) == 0 {
		if scope == 1 {
			st.general = sortBestMatch
		}
		return sortBestMatch
	}
	chosen := &st.general
	if pivot == "albums" {
		chosen = &st.albums
	}
	if !slices.Contains(options, *chosen) {
		*chosen = sortBestMatch
	}
	s.selectOf(c.Key("sort"), chosen, options).Width(170).Label("Sort by")
	return *chosen
}

// byPlayed orders by the count of plays or views, the most first, and
// keeps the order of those that tie or have none, which come last.
func byPlayed[T any](count func(T) int64) func(a, b T) int {
	return func(a, b T) int { return cmp.Compare(max(count(b), 0), max(count(a), 0)) }
}

// sortTracks is tracks with the most played first, as a copy.
func sortTracks(tracks []media.Track) []media.Track {
	out := slices.Clone(tracks)
	slices.SortStableFunc(out, byPlayed(func(t media.Track) int64 { return t.Plays }))
	return out
}

// sortPlaylists is playlists with the most viewed first, as a copy.
func sortPlaylists(playlists []media.Playlist) []media.Playlist {
	out := slices.Clone(playlists)
	slices.SortStableFunc(out, byPlayed(func(p media.Playlist) int64 { return p.Views }))
	return out
}

// albumYear reads the year of an album, or 0 where it has none.
func albumYear(a media.Album) int {
	y, err := strconv.Atoi(strings.TrimSpace(a.Year))
	if err != nil || y <= 0 {
		return 0
	}
	return y
}

// newestFirst is the order of albums, by the year of each, the newest
// first; those with no year come last, and ties keep their order.
func newestFirst(albums []media.Album) []media.Album {
	out := slices.Clone(albums)
	slices.SortStableFunc(out, func(a, b media.Album) int { return cmp.Compare(albumYear(b), albumYear(a)) })
	return out
}

// playedResults are the results of the services of scope, for the order
// of Most played: with both, YouTube Music's first, as Spotify has no
// counts and goes after those that have, then each sorted by its count.
func playedResults(r app.SearchResponse, scope int, shown func(media.SearchResults) media.SearchResults) media.SearchResults {
	out := mergeResults(r, scope)
	if scope == 0 {
		out.Tracks = slices.Concat(r.YouTube.Tracks, r.Spotify.Tracks)
		out.Videos = slices.Concat(r.YouTube.Videos, r.Spotify.Videos)
		out.Playlists = slices.Concat(r.YouTube.Playlists, r.Spotify.Playlists)
	}
	out = shown(out)
	out.Tracks = sortTracks(out.Tracks)
	out.Videos = sortTracks(out.Videos)
	out.Playlists = sortPlaylists(out.Playlists)
	return out
}

// newestAlbumTiles are the tiles of albums with the newest first: an
// album of several versions is one tile, placed by the year of the
// version it shows.
func (s *shell) newestAlbumTiles(c *ui.Context, key string, albums []media.Album) {
	groups := s.albumGroups(key, albums)
	shown := make([]media.Album, len(groups))
	for i, g := range groups {
		shown[i] = s.preferredVersion(g)
	}
	order := make([]int, len(shown))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(a, b int) int { return cmp.Compare(albumYear(shown[b]), albumYear(shown[a])) })
	for n, i := range order {
		s.albumTile(c, key+"-"+count(n), shown[i])
	}
}

// sortNote tells, while Most played orders the results of both services,
// where Spotify's go.
func (s *shell) sortNote(c *ui.Context) {
	ui.Text(c, "Spotify gives apps no play counts, so its results come after YouTube Music's.").
		FontSize(s.lk.smallSize()).TextColor(s.lk.muted).Margin(0, 0, 16)
}
