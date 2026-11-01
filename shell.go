package main

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/fzn0x/mixmusic/internal/media"
)

// view builds the main window: the rail, the top bar over the main pane
// and the side panel, and the player bar across the bottom.
func (s *shell) view(c *ui.Context) {
	s.st, s.capBound = s.p.State(), false
	s.kept = s.svc.Downloads.Snapshot()
	s.applyZoom(c)
	// A window that turns narrow or wide drops the fold the user gave
	// its rail.
	if w, _ := c.Size(); (w <= narrowWidth) != s.narrow {
		s.narrow, s.narrowFold = w <= narrowWidth, nil
	}
	s.followArtwork()
	c.SetTheme(&s.lk.theme)
	s.toasts(c)
	s.runShortcuts(c)
	k := &s.lk
	root := ui.Column(c).Fill().Background(k.bg).TextColor(k.text).Font(k.body)
	s.ground(c, root)
	root.Children(func() {
		ui.Row(c).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
			s.rail(c)
			ui.Column(c).Grow(1).MinWidth(0).Children(func() {
				s.topBar(c)
				ui.Row(c).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
					s.mainPane(c)
					if s.panel != "" {
						s.sidePanel(c)
					}
				})
			})
		})
		s.playerBar(c)
		s.overlay(c)
	})
	s.palette(c)
	s.updateOffer(c)
	s.questionDialog(c)
	s.promptDialog(c)
	s.sweepDialogs()
	s.sweepViews()
}

// mainPane shows the page of the router's path. Pages scroll themselves,
// as pageScroll does, or as lists, which build only the rows in view.
func (s *shell) mainPane(c *ui.Context) {
	s.router.View(c, func(r *ui.Route) { s.page(c, r) }).Grow(1).MinWidth(0)
}

// page builds the page of a route.
func (s *shell) page(c *ui.Context, r *ui.Route) {
	s.pagePath = r.Path()
	switch {
	case r.Match("/"):
		r.Title("Home")
		s.homePage(c)
	case r.Match("/search"):
		r.Title("Search")
		s.searchPage(c, r.Query("q"))
	case r.Match("/album/{id}"):
		s.albumPage(c, r.Param("id"))
	case r.Match("/artist/{id}"):
		s.artistPage(c, r.Param("id"))
	case r.Match("/playlist/{id}"):
		s.playlistPage(c, r.Param("id"))
	case r.Match("/library"):
		s.libraryPage(c, "")
	case r.Match("/library/{tab}"):
		s.libraryPage(c, r.Param("tab"))
	case r.Match("/now-playing"):
		s.nowPlayingPage(c)
	case r.Match("/settings"):
		s.settingsPage(c, "")
	case r.Match("/settings/{tab}"):
		s.settingsPage(c, r.Param("tab"))
	case r.Match("/explore"):
		s.explorePage(c, "")
	case r.Match("/explore/{tab}"):
		s.explorePage(c, r.Param("tab"))
	case r.Match("/mood/{params}"):
		s.moodPage(c, r.Param("params"))
	case r.Match("/stats"):
		s.statsPage(c)
	case r.Match("/plugin/{plugin}/{page}"):
		s.pluginPage(c, "/plugin/"+r.Param("plugin")+"/"+r.Param("page"))
	case r.Match("/marketplace"):
		s.marketplacePage(c, "")
	case r.Match("/marketplace/{tab}"):
		s.marketplacePage(c, r.Param("tab"))
	default:
		s.lost(c, "This page isn't one of MixMusic's.")
	}
}

// lost stands for a page there is none of, saying why.
func (s *shell) lost(c *ui.Context, why string) {
	k := &s.lk
	s.pageScroll(c, func() {
		s.pageHeader(c, "lost", nil, nil)
		ui.Text(c, "nothing lives here").FontSize(k.size(3.5)).FontWeight(300).Font(k.display).TextColor(k.accent)
		ui.Text(c, why).FontSize(k.subtitleSize()).FontWeight(300).Margin(0, 0, 24)
		if s.button(c, "go-home", "go home", true).Clicked() {
			s.navigate("/")
		}
	})
}

// toasts shows what the player told, as toasts above the player bar.
func (s *shell) toasts(c *ui.Context) {
	for _, n := range s.notes {
		switch {
		case n.Settings:
			c.ToastAction(n.Message, "settings", func() { s.navigate("/settings/playback") })
		case n.Error:
			c.AddToast(ui.Toast{Title: n.Message, Type: "error"})
		default:
			c.Toast(n.Message)
		}
	}
	s.notes = s.notes[:0]
	for _, a := range s.actionToasts {
		c.ToastAction(a.message, a.action, a.run)
	}
	s.actionToasts = s.actionToasts[:0]
}

// actionToast is a toast with an action, as "use it" once a theme
// installed.
type actionToast struct {
	message, action string
	run             func()
}

// showMini shows the mini player.
func (s *shell) showMini() { go s.svc.System.ShowMiniPlayer() }

// Paths of the pages of things.

func albumPath(id string) string    { return "/album/" + url.PathEscape(id) }
func artistPath(id string) string   { return "/artist/" + url.PathEscape(id) }
func playlistPath(id string) string { return "/playlist/" + url.PathEscape(id) }
func searchPath(q string) string    { return "/search?q=" + url.QueryEscape(q) }

// itemPath is the page of a shelf's item, or "" for a track.
func itemPath(it media.Item) string {
	switch {
	case it.Album != nil:
		return albumPath(it.Album.ID)
	case it.Artist != nil:
		return artistPath(it.Artist.ID)
	case it.Playlist != nil:
		return playlistPath(it.Playlist.ID)
	}
	return ""
}

// count writes n, small, as a pivot's count.
func count(n int) string { return strconv.Itoa(n) }

// lower is s in lower case, as the interface's own words are.
func lower(s string) string { return strings.ToLower(s) }
