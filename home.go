package main

import (
	"context"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/library"
	"github.com/fzn0x/mixmusic/internal/media"
)

// homeFilters are the home page's pivots, and the sources they keep.
var homeFilters = []struct {
	label  string
	source media.Source
}{{"everything", ""}, {"yours", media.SourceLocal}, {"spotify", media.SourceSpotify}, {"youtube music", media.SourceYouTube}}

// homePage shows the user's own tiles, then the shelves of every service.
func (s *shell) homePage(c *ui.Context) {
	k := &s.lk
	s.pageScroll(c, func() {
		labels := make([]string, len(homeFilters))
		for i, f := range homeFilters {
			labels[i] = f.label
		}
		s.pageHeader(c, "home", labels, &s.homeFilter)
		s.toolProblems(c)
		home := fetch(s, "home", s.svc.Catalog.Home)
		recent := fetch(s, "history", func(context.Context) ([]library.Play, error) { return s.svc.Collection.History(1), nil })
		accounts := fetch(s, "accounts", func(ctx context.Context) (app.AccountsStatus, error) { return s.svc.Accounts.Status(ctx), nil })

		var last *media.Track
		if len(recent.value) > 0 {
			last = &recent.value[0].Track
		} else if cur, ok := s.st.Current(); ok {
			last = &cur.Track
		}
		s.tileGrid(c, func(columns int) {
			title, caption, label := "start here", "search for something to play", "Search for music"
			var art []media.Image
			if last != nil {
				title, caption, label = "jump back in", trackLine(*last), "Play "+last.Title+" again"
				art = last.Artwork
			}
			if s.flatTile(c, "tile-last", 0, tileLarge, columns, title, caption, label, art) {
				if last != nil {
					s.p.PlayTracks([]media.Track{*last}, 0)
				} else {
					s.navigate("/search")
				}
			}
			offer := accounts.done && !accounts.value.Spotify.SignedIn
			span := tileWide
			if offer {
				span = tileSmall
				if s.flatTile(c, "tile-spotify", 3, tileTall, columns, "add spotify", "sign in to search and play your spotify library here", "Add Spotify", nil) {
					s.navigate("/settings/accounts")
				}
			}
			if s.flatTile(c, "tile-liked", 1, span, columns, "liked songs", plural(len(s.svc.Collection.Liked()), "track"), "Liked songs", nil) {
				s.navigate("/library/liked")
			}
			if s.flatTile(c, "tile-mixes", 2, span, columns, "your mixes", plural(len(s.svc.Collection.Playlists()), "mix", "mixes"), "Your mixes", nil) {
				s.navigate("/library/mixes")
			}
		})
		switch {
		case home.err != nil && !home.done:
		case home.err != nil:
			s.problem(c, home.err, func() { s.forget("home") })
			return
		case !home.done:
			s.loading(c)
			return
		}
		source := homeFilters[s.homeFilter].source
		var shelves []media.Shelf
		for _, sh := range home.value {
			if source == "" || sh.Source == source {
				shelves = append(shelves, sh)
			}
		}
		if len(shelves) == 0 {
			switch source {
			case media.SourceSpotify:
				s.emptyLine(c, "Sign in to Spotify in Settings › Accounts to see your playlists and recent tracks here.")
			case media.SourceLocal:
				s.emptyLine(c, "Play some tracks and make a mix: what you collect shows here.")
			default:
				s.emptyLine(c, "Nothing here yet.")
			}
			return
		}
		s.shelves(c, "home", shelves)
		_ = k
	})
}

// pageScroll is a page that scrolls, inside the pane's gutter.
func (s *shell) pageScroll(c *ui.Context, content func()) {
	page := ui.Scroll(c).Grow(1).MinWidth(0).MinHeight(0)
	// Capture mode scrolls the page the router shows (act:scroll): not the
	// one it leaves, which shows beside it while it slides in.
	if s.capScroll != nil && !s.capBound && s.pagePath == s.router.Path() {
		s.capBound = true
		page.TrackScroll(s.capScroll)
	}
	page.Children(func() {
		ui.Column(c).Padding(0, s.lk.gutter, 64).Children(content)
	})
}
