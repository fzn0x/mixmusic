package main

import (
	"context"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/locales"
)

var (
	searchPivots = []string{"all", "songs", "videos", "albums", "artists", "playlists", "podcasts"}
	// searchScopes are the services a search covers, by their names: the
	// product names stay as they are, and searchScopeLabels translates "both".
	searchScopes = []string{"both", "Spotify", "YouTube Music", "YouTube"}
)

// searchPivotLabels are the names of searchPivots, in the language the app
// shows.
func searchPivotLabels() []string {
	return []string{
		locales.SearchPivotAll(), locales.SearchPivotSongs(), locales.SearchPivotVideos(), locales.SearchPivotAlbums(),
		locales.SearchPivotArtists(), locales.SearchPivotPlaylists(), locales.SearchPivotPodcasts(),
	}
}

// searchScopeLabels are the names of searchScopes, in the language the app
// shows.
func searchScopeLabels() []string {
	labels := slices.Clone(searchScopes)
	labels[0] = locales.SearchScopeBoth()
	return labels
}

// youtubeScope is the index of YouTube's own search in searchScopes: the
// website's videos, which YouTube Music doesn't list.
const youtubeScope = 3

// interleave alternates the items of two lists, so both services lead.
func interleave[T any](a, b []T) []T {
	out := make([]T, 0, len(a)+len(b))
	for i := 0; i < max(len(a), len(b)); i++ {
		if i < len(a) {
			out = append(out, a[i])
		}
		if i < len(b) {
			out = append(out, b[i])
		}
	}
	return out
}

// mergeResults are the results of the services of scope: 0 both, 1
// Spotify, 2 YouTube Music. YouTube's own search is asked apart, by
// youtubeResults.
func mergeResults(r app.SearchResponse, scope int) media.SearchResults {
	switch scope {
	case 1:
		return r.Spotify
	case 2:
		return r.YouTube
	}
	return media.SearchResults{
		Tracks:    interleave(r.Spotify.Tracks, r.YouTube.Tracks),
		Videos:    interleave(r.Spotify.Videos, r.YouTube.Videos),
		Albums:    interleave(r.Spotify.Albums, r.YouTube.Albums),
		Artists:   interleave(r.Spotify.Artists, r.YouTube.Artists),
		Playlists: interleave(r.Spotify.Playlists, r.YouTube.Playlists),
	}
}

// searchPage searches both services at once, with the query as the page's
// word; with nothing to search, it shows the recent searches.
func (s *shell) searchPage(c *ui.Context, q string) {
	q = strings.TrimSpace(q)
	k := &s.lk
	s.pageScroll(c, func() {
		if q == "" {
			s.recentSearches(c)
			return
		}
		res := fetch(s, "search:"+q, func(ctx context.Context) (app.SearchResponse, error) { return s.svc.Catalog.Search(ctx, q) })
		var counts []int
		if res.done && res.err == nil && s.searchScope != youtubeScope {
			r := s.shownResults(mergeResults(res.value, s.searchScope))
			// An album counts once, however many versions it has.
			albums := len(s.albumGroups("album", r.Albums))
			counts = []int{-1, len(r.Tracks), len(r.Videos), albums, len(r.Artists), len(r.Playlists), s.podcastCount(q)}
		}
		// YouTube's own search lists videos only, so it has no pivots.
		pivots, pivotSel := searchPivotLabels(), &s.searchPivot
		if s.searchScope == youtubeScope {
			pivots, pivotSel = nil, nil
		}
		s.pageHeaderCounts(c, q, pivots, counts, pivotSel)
		// YouTube's own search lists videos, so it orders them as the Videos pivot does.
		sortPivot := searchPivots[s.searchPivot]
		if s.searchScope == youtubeScope {
			sortPivot = "videos"
		}
		order := sortBestMatch
		ui.Row(c).Margin(0, 0, 24).Wrap().GapX(16).GapY(12).AlignItems(ui.Center).Children(func() {
			s.segmented(c, "scope", locales.SearchServices(), &s.searchScope, searchScopeLabels()...)
			order = s.sortSelect(c, q, sortPivot, s.searchScope)
		})
		if s.searchScope == youtubeScope {
			s.youtubeResults(c, q, order)
			return
		}
		if s.searchScope == 0 {
			s.computerResults(c, q, searchPivots[s.searchPivot])
		}
		switch {
		case !res.done:
			s.loading(c)
			return
		case res.err != nil:
			s.problem(c, res.err, func() { s.forget("search:" + q) })
			return
		}
		if !res.value.SpotifySignedIn && s.searchScope != 2 && s.searchScope != youtubeScope {
			ui.RichText(c).FontSize(k.smallSize()).TextColor(k.muted).Margin(0, 0, 16).Children(func() {
				inlineMessage(c, locales.SearchYouTubeOnly(mark(0, locales.SearchSignInSpotify())), func(text string) {
					s.linkOf(c, text, "/settings/accounts")
				})
			})
		}
		if len(res.value.Errors) > 0 {
			ui.Text(c, strings.Join(res.value.Errors, " · ")).FontSize(k.smallSize()).TextColor(k.danger).Margin(0, 0, 16)
		}
		r := s.shownResults(mergeResults(res.value, s.searchScope))
		if order == sortMostPlayed {
			r = playedResults(res.value, s.searchScope, s.shownResults)
			if s.searchScope == 0 && res.value.SpotifySignedIn {
				s.sortNote(c)
			}
		}
		pivot := searchPivots[s.searchPivot]
		if pivot == "all" || pivot == "songs" {
			tracks := r.Tracks
			if pivot == "all" {
				s.sectionTitle(c, locales.SearchSongs())
				tracks = tracks[:min(len(tracks), 10)]
			}
			ui.Column(c).Margin(0, 0, 48).Children(func() {
				s.trackList(c, "results", tracks, trackListOptions{art: true, album: true, plays: true, empty: locales.SearchNoSongs()})
			})
		}
		// Videos are YouTube's own, as DJ sets and live recordings, which
		// aren't songs of YouTube Music's catalog: All shows the first few,
		// and the Videos pivot every one.
		if (pivot == "all" && len(r.Videos) > 0) || pivot == "videos" {
			videos := r.Videos
			if pivot == "all" {
				s.sectionTitle(c, locales.SearchVideos())
				videos = videos[:min(len(videos), 5)]
			}
			empty := locales.SearchNoVideos()
			if s.searchScope == 1 {
				empty = locales.SearchOnlyYouTubeVideos()
			}
			ui.Column(c).Margin(0, 0, 48).Children(func() {
				s.trackList(c, "video-results", videos, trackListOptions{art: true, plays: true, empty: empty})
			})
		}
		switch pivot {
		case "all":
			if len(r.Artists) > 0 {
				s.shelf(c, "artists", locales.SearchArtists(), func() {
					for i, a := range r.Artists {
						s.artistTile(c, "artist-"+count(i), a)
					}
				})
			}
			if len(r.Albums) > 0 {
				s.shelf(c, "albums", locales.SearchAlbums(), func() { s.albumTiles(c, "album", r.Albums) })
			}
			if len(r.Playlists) > 0 {
				s.shelf(c, "playlists", locales.SearchPlaylists(), func() {
					for i, p := range r.Playlists {
						s.playlistTile(c, "playlist-"+count(i), p)
					}
				})
			}
		case "albums":
			s.tileWrap(c, func() {
				if s.searchSort.albums == sortNewest {
					s.newestAlbumTiles(c, "album", r.Albums)
					return
				}
				s.albumTiles(c, "album", r.Albums)
			})
		case "artists":
			s.tileWrap(c, func() {
				for i, a := range r.Artists {
					s.artistTile(c, "artist-"+count(i), a)
				}
			})
		case "playlists":
			// Spotify's search lists the playlists of others too, whose
			// tracks it shows apps none of: they're left out (#530).
			if s.searchScope == 1 && res.value.SpotifySignedIn {
				ui.Text(c, locales.SearchSpotifyPlaylists()).
					FontSize(k.smallSize()).TextColor(k.muted).Margin(0, 0, 16)
			}
			s.tileWrap(c, func() {
				for i, p := range r.Playlists {
					s.playlistTile(c, "playlist-"+count(i), p)
				}
			})
		case "podcasts":
			s.podcastSearch(c, q)
		}
	})
}

// youtubeResults shows what YouTube's own search found: videos only, in
// the order it ranks them. It is asked apart from the music catalog's
// search, so that a slow or blocked youtube.com never holds that one up.
func (s *shell) youtubeResults(c *ui.Context, q, order string) {
	res := fetch(s, "searchweb:"+q, func(ctx context.Context) (media.SearchResults, error) {
		return s.svc.Catalog.SearchYouTube(ctx, q)
	})
	switch {
	case !res.done:
		s.loading(c)
		return
	case res.err != nil:
		s.problem(c, res.err, func() { s.forget("searchweb:" + q) })
		return
	}
	videos := s.shownResults(res.value).Videos
	if order == sortMostPlayed {
		videos = sortTracks(videos)
	}
	ui.Column(c).Margin(0, 0, 48).Children(func() {
		s.trackList(c, "youtube-results", videos, trackListOptions{art: true, plays: true, empty: locales.SearchNoVideos()})
	})
}

// tileWrap lays artwork tiles in rows that wrap.
func (s *shell) tileWrap(c *ui.Context, tiles func()) {
	ui.Row(c).Wrap().GapX(s.lk.gap).GapY(1.5 * s.lk.gap).AlignItems(ui.Start).Children(tiles)
}

// recentSearches invites a search, and lists the recent ones.
func (s *shell) recentSearches(c *ui.Context) {
	k := &s.lk
	s.pageHeader(c, locales.SearchTitle(), nil, nil)
	ui.Column(c).Gap(16).Margin(0, 0, 48).Children(func() {
		ui.Text(c, k.words(locales.SearchFindAnything())).FontSize(k.bigSize()).FontWeight(k.displayWeight()).Font(k.display).TextColor(k.accent)
		ui.Text(c, locales.SearchIntro()).
			FontSize(k.subtitleSize()).FontWeight(k.leadWeight()).MaxWidth(640)
		if s.button(c, "start-typing", locales.SearchStartTyping(), true).Clicked() {
			s.searchBox.Focus()
		}
	})
	searches := s.svc.Collection.Searches()
	if len(searches) == 0 {
		return
	}
	ui.Row(c).Justify(ui.SpaceBetween).Children(func() {
		ui.Column(c).Grow(1).Children(func() { s.sectionTitle(c, locales.SearchRecent()) })
	})
	ui.Row(c).Wrap().Gap(8).Children(func() {
		for i, q := range searches {
			chip := s.buttonBase(c.Key("recent-" + count(i))).Height(34).PaddingX(14).
				Radius(k.round(34)).Label(q).Cursor(ui.CursorPointer)
			switch {
			case k.tonal && chip.Hovered():
				chip.Background(k.surface3)
			case k.tonal:
				chip.Background(k.surface2)
			case chip.Hovered():
				chip.Background(k.hover).Border(2, k.outline)
			default:
				chip.Border(2, k.outline)
			}
			if chip.Clicked() {
				s.query = q
				s.navigate(searchPath(q))
			}
			chip.Children(func() { ui.Text(c, q).FontSize(k.smallSize()).SingleLine() })
		}
	})
	clear := s.buttonBase(c.Key("clear-searches")).Margin(16, 0, 0).AlignSelf(ui.Start).Label(locales.SearchClearRecent()).Cursor(ui.CursorPointer)
	if clear.Clicked() {
		if err := s.svc.Collection.ClearSearches(); err != nil {
			s.notes = append(s.notes, noticeOf(err))
		}
	}
	clear.Children(func() { ui.Text(c, locales.SearchClear()).FontSize(k.smallSize()).TextColor(k.accent) })
}
