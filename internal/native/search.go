package native

import (
	"context"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
)

var (
	searchPivots = []string{"all", "songs", "albums", "artists", "playlists"}
	searchScopes = []string{"both", "spotify", "youtube music"}
)

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
// Spotify, 2 YouTube Music.
func mergeResults(r app.SearchResponse, scope int) media.SearchResults {
	switch scope {
	case 1:
		return r.Spotify
	case 2:
		return r.YouTube
	}
	return media.SearchResults{
		Tracks:    interleave(r.Spotify.Tracks, r.YouTube.Tracks),
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
		if res.done && res.err == nil {
			r := mergeResults(res.value, s.searchScope)
			counts = []int{-1, len(r.Tracks), len(r.Albums), len(r.Artists), len(r.Playlists)}
		}
		s.pageHeaderCounts(c, q, searchPivots, counts, &s.searchPivot)
		ui.Row(c).Margin(0, 0, 24).Children(func() {
			s.segmented(c, "scope", "Services", &s.searchScope, searchScopes...)
		})
		switch {
		case !res.done:
			s.loading(c)
			return
		case res.err != nil:
			s.problem(c, res.err, func() { s.forget("search:" + q) })
			return
		}
		if !res.value.SpotifySignedIn && s.searchScope != 2 {
			ui.RichText(c).FontSize(k.smallSize()).TextColor(k.muted).Margin(0, 0, 16).Children(func() {
				ui.Text(c, "Searching YouTube Music only. ")
				ui.Link(c, "Sign in to Spotify", "/settings/accounts")
				ui.Text(c, " to search it too.")
			})
		}
		if len(res.value.Errors) > 0 {
			ui.Text(c, strings.Join(res.value.Errors, " · ")).FontSize(k.smallSize()).TextColor(k.danger).Margin(0, 0, 16)
		}
		r := mergeResults(res.value, s.searchScope)
		pivot := searchPivots[s.searchPivot]
		if pivot == "all" || pivot == "songs" {
			tracks := r.Tracks
			if pivot == "all" {
				s.sectionTitle(c, "songs")
				tracks = tracks[:min(len(tracks), 10)]
			}
			ui.Column(c).Margin(0, 0, 48).Children(func() {
				s.trackList(c, "results", tracks, trackListOptions{art: true, album: true, empty: "No songs match."})
			})
		}
		switch pivot {
		case "all":
			if len(r.Artists) > 0 {
				s.shelf(c, "artists", "artists", func() {
					for i, a := range r.Artists {
						s.artistTile(c, "artist-"+count(i), a)
					}
				})
			}
			if len(r.Albums) > 0 {
				s.shelf(c, "albums", "albums", func() {
					for i, a := range r.Albums {
						s.albumTile(c, "album-"+count(i), a)
					}
				})
			}
			if len(r.Playlists) > 0 {
				s.shelf(c, "playlists", "playlists", func() {
					for i, p := range r.Playlists {
						s.playlistTile(c, "playlist-"+count(i), p)
					}
				})
			}
		case "albums":
			s.tileWrap(c, func() {
				for i, a := range r.Albums {
					s.albumTile(c, "album-"+count(i), a)
				}
			})
		case "artists":
			s.tileWrap(c, func() {
				for i, a := range r.Artists {
					s.artistTile(c, "artist-"+count(i), a)
				}
			})
		case "playlists":
			s.tileWrap(c, func() {
				for i, p := range r.Playlists {
					s.playlistTile(c, "playlist-"+count(i), p)
				}
			})
		}
	})
}

// tileWrap lays artwork tiles in rows that wrap.
func (s *shell) tileWrap(c *ui.Context, tiles func()) {
	ui.Row(c).Wrap().GapX(16).GapY(24).AlignItems(ui.Start).Children(tiles)
}

// recentSearches invites a search, and lists the recent ones.
func (s *shell) recentSearches(c *ui.Context) {
	k := &s.lk
	s.pageHeader(c, "search", nil, nil)
	ui.Column(c).Gap(16).Margin(0, 0, 48).Children(func() {
		ui.Text(c, "find anything").FontSize(k.size(3.5)).FontWeight(300).Font(k.display).TextColor(k.accent)
		ui.Text(c, "One search looks through YouTube Music and, once you sign in, Spotify.").
			FontSize(k.subtitleSize()).FontWeight(300).MaxWidth(640)
		if s.button(c, "start-typing", "start typing", true).Clicked() {
			s.searchBox.Focus()
		}
	})
	searches := s.svc.Collection.Searches()
	if len(searches) == 0 {
		return
	}
	ui.Row(c).Justify(ui.SpaceBetween).Children(func() {
		ui.Column(c).Grow(1).Children(func() { s.sectionTitle(c, "recent") })
	})
	ui.Row(c).Wrap().Gap(8).Children(func() {
		for i, q := range searches {
			chip := ui.ButtonBase(c.Key("recent-"+count(i))).Height(34).PaddingX(14).Border(2, k.outline).
				Radius(k.radiusSm).Label(q).Cursor(ui.CursorPointer)
			if chip.Hovered() {
				chip.Background(k.hover)
			}
			if chip.Clicked() {
				s.query = q
				s.navigate(searchPath(q))
			}
			chip.Children(func() { ui.Text(c, q).FontSize(k.smallSize()).SingleLine() })
		}
	})
	clear := ui.ButtonBase(c.Key("clear-searches")).Margin(16, 0, 0).AlignSelf(ui.Start).Label("Clear the recent searches").Cursor(ui.CursorPointer)
	if clear.Clicked() {
		if err := s.svc.Collection.ClearSearches(); err != nil {
			s.notes = append(s.notes, noticeOf(err))
		}
	}
	clear.Children(func() { ui.Text(c, "clear").FontSize(k.smallSize()).TextColor(k.accent) })
}
