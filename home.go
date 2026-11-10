package main

import (
	"context"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
)

// homeFilters are the home page's pivots, and the sources they keep.
var homeFilters = []struct {
	label  string
	source media.Source
}{{"Everything", ""}, {"Yours", media.SourceLocal}, {"Spotify", media.SourceSpotify}, {"YouTube Music", media.SourceYouTube}}

// networkOnline tells whether the network answers, as last checked; tests
// answer for it.
var networkOnline = func(s *shell) bool { return s.svc.Downloads.Online() }

// homePage shows the user's own tiles, then the shelves of every service,
// in the order the user arranged them (homelayout.go), or, while they
// edit it, the list they arrange them in (homeedit.go).
func (s *shell) homePage(c *ui.Context) {
	k := &s.lk
	s.pageScroll(c, func() {
		if s.home.editing {
			s.pageHeader(c, "Home", nil, nil)
			s.homeEditor(c)
			return
		}
		labels := make([]string, len(homeFilters))
		for i, f := range homeFilters {
			labels[i] = f.label
		}
		s.pageHeader(c, "Home", labels, &s.homeFilter)
		s.toolProblems(c)
		accounts := fetch(s, "accounts", func(ctx context.Context) (app.AccountsStatus, error) { return s.svc.Accounts.Status(ctx), nil })

		// The track played last, as the library has it now.
		var last *media.Track
		if recent := s.svc.Collection.History(1); len(recent) > 0 {
			last = &recent[0].Track
		} else if cur, ok := s.st.Current(); ok {
			last = &cur.Track
		}
		if last != nil && s.hides(*last) {
			last = nil // the settings hide it (explicit.go)
		}
		ui.Row(c).Justify(ui.End).Margin(0, 0, 16).Children(func() {
			if s.button(c, "home-edit", "edit home", false).Clicked() {
				s.home.editing = true
			}
		})
		s.tileGrid(c, func(columns int) {
			title, caption, label := "Start here", k.words("Search for something to play"), "Search for music"
			var art []media.Image
			if last != nil {
				title, caption, label = "Jump back in", trackLine(*last), "Play "+last.Title+" again"
				art = last.Artwork
			}
			if s.flatTile(c, "tile-last", 0, tileLarge, columns, title, caption, label, icon("search"), art) {
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
				if s.flatTile(c, "tile-spotify", 3, tileTall, columns, "Add Spotify", k.words("Sign in to search and play your Spotify library here"), "Add Spotify", icon("plus"), nil) {
					s.navigate("/settings/accounts")
				}
			}
			if s.flatTile(c, "tile-liked", 1, span, columns, "Liked songs", plural(len(s.svc.Collection.Liked()), "track"), "Liked songs", solidIcons["heart"], nil) {
				s.navigate("/library/liked")
			}
			if s.flatTile(c, "tile-mixes", 2, span, columns, "Your mixes", plural(len(s.svc.Collection.Playlists()), "mix", "mixes"), "Your mixes", icon("queue"), nil) {
				s.navigate("/library/mixes")
			}
		})
		// The user's own shelves show at once, then those of each service
		// once they came: one that still loads holds back those after it,
		// which would move once it came (homePlaces).
		source := homeFilters[s.homeFilter].source
		layout := s.set.HomeShelves
		shown, failed := 0, false
		for _, p := range homePlaces(layout, s.homeGroups(layout), source) {
			g := p.group
			switch {
			case p.shelf != nil:
				shown += s.showHomeShelf(c, p.shelf, source)
			case !g.done:
				s.loading(c)
				return
			case g.added:
				// A shelf the user added that couldn't come says why where
				// it shows.
				key := g.key
				ui.Column(c.Key("home-added-"+key)).Margin(0, 0, k.sections()).Children(func() {
					s.sectionTitle(c, g.title)
					s.problem(c, g.err, func() { s.forget(key) })
				})
			default:
				key := g.key
				s.problem(c, g.err, func() { s.forget(key) })
				// Offline, the downloads still play: they're offered once,
				// under the first service that failed.
				if !failed && !networkOnline(s) && s.kept.Status().Tracks > 0 {
					ui.Row(c).Margin(16, 0, 0).Children(func() {
						if s.button(c, "home-downloads", "open downloads", true).Clicked() {
							s.navigate("/library/downloads")
						}
					})
				}
				failed = true
			}
		}
		if shown == 0 && !failed {
			switch source {
			case media.SourceSpotify:
				s.emptyLine(c, "Sign in to Spotify in Settings › Accounts to see your playlists and recent tracks here.")
			case media.SourceLocal:
				s.emptyLine(c, "Play some tracks and make a mix: what you collect shows here.")
			default:
				s.emptyLine(c, "Nothing here yet.")
			}
		}
	})
}

// showHomeShelf shows the shelf sh of Home, of the pivot of source, and
// returns how many items it shows.
func (s *shell) showHomeShelf(c *ui.Context, sh *homeShelf, source media.Source) int {
	if sh.build != nil {
		return sh.build(c, sh.ui, source)
	}
	s.itemShelf(c, sh.ui, sh.shelf)
	return len(sh.shelf.Items)
}

// localShelves are the home page's shelves of what the user played and
// made, built again only once the library changed.
func (s *shell) localShelves() []media.Shelf {
	if v := s.svc.Collection.Version(); !s.localBuilt || v != s.localVersion {
		s.local, s.localVersion, s.localBuilt = s.svc.Catalog.LocalShelves(), v, true
	}
	return s.local
}

// pageScroll is a page that scrolls, inside the pane's gutter. The page
// the router shows keeps where it scrolled for the top bar and the rows
// pinned under it (showPage), or, in capture mode, has it scrolled
// (act:scroll): not the one it leaves, which shows beside it while it
// slides in. The page shown may be pulled down past its top to refresh it,
// which opens a strip above its content (pullStrip): the content is keyed,
// so that what it holds keeps its state as the strip comes and goes.
func (s *shell) pageScroll(c *ui.Context, content func()) {
	page := ui.Scroll(c).Grow(1).MinWidth(0).MinHeight(0)
	var track *ui.ScrollState
	if s.pagePath == s.router.Path() && !s.here.built {
		track = s.showPage(page)
	}
	page.Children(func() {
		if track != nil {
			s.pullStrip(c)
		}
		col := ui.Column(c.Key("page-content")).Padding(0, s.lk.gutter, 64)
		if track != nil {
			s.takeContent(page, col)
		}
		col.Children(content)
		if track != nil && s.here.room > 0 {
			col.MinHeight(s.here.room)
		}
	})
	// Once the content took where the page is: it may have moved it.
	if track != nil {
		page.TrackScroll(track).HandleInput(s.pullInput)
	}
}
