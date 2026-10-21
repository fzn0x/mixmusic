package native

import (
	"math"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
)

// navItem is a page of the rail.
type navItem struct {
	id, label, icon, path string
}

var nav = []navItem{
	{"home", "home", "home", "/"},
	{"search", "search", "search", "/search"},
	{"explore", "explore", "compass", "/explore"},
	{"library", "library", "library", "/library"},
	{"stats", "stats", "stats", "/stats"},
	{"marketplace", "marketplace", "store", "/marketplace"},
	{"settings", "settings", "settings", "/settings"},
}

// section is the rail's item of path: its first part, with a mood's page
// in explore's.
func section(path string) string {
	first, _, _ := strings.Cut(strings.TrimPrefix(path, "/"), "/")
	switch first {
	case "":
		return "home"
	case "mood":
		return "explore"
	}
	return first
}

// narrowWidth is the widest window whose rail folds to its icons.
const narrowWidth = 1180

// railFolded tells whether the rail shows its icons only: as the user
// chose, or in a narrow window, unless the user unfolded it there.
func (s *shell) railFolded() bool {
	if s.narrowFold != nil {
		return *s.narrowFold
	}
	return s.set.SidebarCollapsed || s.narrow
}

// railWidth is the rail's width, folded or not.
func (s *shell) railWidth() float32 {
	if s.railFolded() {
		return 76
	}
	return s.lk.rail
}

// toggleRail folds or unfolds the rail: in the settings, or in a narrow
// window until it widens.
func (s *shell) toggleRail() {
	folded := !s.railFolded()
	if s.narrow {
		s.narrowFold = &folded
		return
	}
	st := s.svc.Preferences.Get()
	st.SidebarCollapsed = folded
	if _, err := s.svc.Preferences.Set(st); err != nil {
		s.notes = append(s.notes, noticeOf(err))
	}
}

// rail is the sidebar: where to go, the user's mixes, and a live tile of
// what plays.
func (s *shell) rail(c *ui.Context) {
	k := &s.lk
	collapsed := s.railFolded()
	path := s.router.Path()
	current := section(path)
	pad := float32(16)
	if collapsed {
		pad = 17
	}
	ui.Column(c).Width(s.railWidth()).Shrink(0).Padding(0, pad, 16).Gap(24).Background(k.bg).Clip().Children(func() {
		s.brand(c, collapsed)
		ui.Column(c).Gap(4).Shrink(0).Children(func() {
			for _, n := range nav {
				s.navEntry(c, "nav-"+n.id, n.label, icon(n.icon), n.path, n.id == current)
			}
			for _, it := range s.plugins.SidebarItems() {
				s.navEntry(c, "plugin-"+it.ID, strings.ToLower(it.Label), s.svgOf(it.Icon), it.Path, path == it.Path)
			}
		})
		if !collapsed {
			s.mixes(c, path)
		} else {
			ui.Spacer(c)
		}
		_, h := c.Size()
		if h >= 720 {
			s.liveTile(c, collapsed)
		}
	})
}

// brand is MixMusic's mark and name, where the window drags, and the
// button that collapses the rail.
func (s *shell) brand(c *ui.Context, collapsed bool) {
	k := &s.lk
	bar := c.TitleBar()
	ui.Row(c).Height(max(bar.Height, 48)).Shrink(0).Padding(0, 0, 0, bar.Left).Gap(12).DragWindow().Children(func() {
		if !collapsed {
			ui.Box(c).Size(26, 26).Shrink(0).Draw(func(p *ui.Painter, r ui.Rect) {
				var a, b ui.Path
				a.Circle(r.X+r.W*12/32, r.Y+r.H/2, r.W*8.5/32)
				b.Circle(r.X+r.W*20/32, r.Y+r.H/2, r.W*8.5/32)
				p.StrokePath(&a, r.W*2.4/32, k.text)
				p.StrokePath(&b, r.W*2.4/32, k.accent)
			})
			ui.Text(c, "mixmusic").FontSize(24).FontWeight(300).Font(k.display).LetterSpacing(-0.03 * 24).
				SingleLine().Grow(1).MinWidth(0)
		}
		label := "Collapse the sidebar"
		if collapsed {
			label = "Expand the sidebar"
		}
		if s.iconButton(c, "collapse", "sidebar", label, false, 18).Clicked() {
			s.toggleRail()
		}
	})
}

// navEntry is an outlined box of an icon with its lowercase label; the
// current page floods its box with the accent.
func (s *shell) navEntry(c *ui.Context, key, label string, svg *ui.SVG, path string, current bool) {
	k := &s.lk
	row := ui.ButtonBase(c.Key(key)).Gap(12).Justify(ui.Start).Label(label).KeepFocus().Cursor(ui.CursorPointer)
	if s.railFolded() {
		row.Tooltip(label)
	}
	if row.Clicked() {
		s.navigate(path)
	}
	hovered, pressed := row.Hovered(), row.Pressed()
	row.Children(func() {
		box := ui.Box(c).Size(42, 42).Shrink(0).Center().Radius(k.radiusSm)
		ink := k.text
		switch {
		case current:
			box.Background(k.accent).Border(2, k.accent)
			ink = k.accentText
		case pressed:
			box.Background(k.text).Border(2, k.text)
			ink = k.bg
		case hovered:
			box.Background(k.hover).Border(2, k.outline)
		default:
			box.Border(2, k.outline)
		}
		box.Children(func() { ui.Icon(c, svg).FontSize(20).TextColor(ink) })
		if !s.railFolded() {
			ink := k.text
			if current {
				ink = k.accent
			}
			ui.Text(c, label).FontSize(17 * k.scale).Font(faceSemilight).TextColor(ink).SingleLine()
		}
	})
}

// mixes lists liked songs and the user's mixes, with their counts.
func (s *shell) mixes(c *ui.Context, path string) {
	k := &s.lk
	ui.Column(c).Grow(1).MinHeight(88).Children(func() {
		ui.Row(c).Justify(ui.SpaceBetween).Margin(0, 0, 8).Children(func() {
			ui.Text(c, "your mixes").FontSize(k.bodySize()).TextColor(k.accent)
			if s.iconButton(c, "new-mix", "plus", "New mix", false, 18).Clicked() {
				s.newMix()
			}
		})
		ui.Scroll(c).Grow(1).MinHeight(0).Children(func() {
			s.sideLink(c, "liked", "liked songs", len(s.svc.Collection.LikedIDs()), "/library/liked", path)
			for _, m := range s.svc.Collection.Playlists() {
				s.sideLink(c, "mix-"+m.ID, m.Title, m.TrackCount, playlistPath(m.ID), path)
			}
		})
	})
}

func (s *shell) sideLink(c *ui.Context, key, title string, n int, to, path string) {
	k := &s.lk
	link := ui.ButtonBase(c.Key(key)).Justify(ui.SpaceBetween).Gap(12).Padding(7, 0).Label(title).KeepFocus().Cursor(ui.CursorPointer)
	if link.Clicked() {
		s.navigate(to)
	}
	ink := k.text
	if link.Hovered() || path == to {
		ink = k.accent
	}
	link.Children(func() {
		ui.Text(c, title).FontSize(k.bodySize()).TextColor(ink).SingleLine().Grow(1).MinWidth(0)
		ui.Text(c, count(n)).FontSize(k.smallSize()).TextColor(k.accent).FontFeatures("tnum")
	})
}

// newMix asks for the name of a mix, makes it and opens it.
func (s *shell) newMix() {
	s.promptName("New mix", "Name", "", "create", func(name string) {
		p, err := s.svc.Collection.CreatePlaylist(name, "", nil)
		if err != nil {
			s.notes = append(s.notes, noticeOf(err))
			return
		}
		s.notes = append(s.notes, player.Notice{Message: "Created " + p.Title + ": add tracks of any service to it"})
		s.navigate(playlistPath(p.ID))
	})
}

// liveTile shows what plays, and opens now playing.
func (s *shell) liveTile(c *ui.Context, collapsed bool) {
	k := &s.lk
	cur, ok := s.st.Current()
	if !ok {
		if !collapsed {
			ui.Column(c).Padding(12, 0).BorderWidth(1, 0, 0, 0).BorderColor(k.rule).Shrink(0).Children(func() {
				ui.Text(c, "nothing playing yet").FontSize(k.smallSize()).TextColor(k.muted)
			})
		}
		return
	}
	size := float32(76)
	if collapsed {
		size = 42
	}
	t := cur.Track
	// While a track plays with another up next, the artwork turns over to
	// it now and then.
	next, flips := s.st.UpNext()
	flips = flips && s.st.Playing && next.Track.ID != t.ID && !collapsed && !c.Preferences().ReduceMotion
	switch {
	case !flips:
		s.liveSince = time.Time{}
	case s.liveSince.IsZero():
		s.liveSince = c.Now()
	}
	tile := ui.ButtonBase(c.Key("live")).Gap(12).Justify(ui.Start).Shrink(0).Label("Open now playing").Cursor(ui.CursorPointer)
	if tile.Clicked() {
		s.navigate("/now-playing")
	}
	tile.Children(func() {
		if !flips {
			s.cover(c, "live-cover", t.Artwork, size, false)
		} else {
			s.liveCard(c, t, next.Track, size)
		}
		if !collapsed {
			ui.Column(c).Grow(1).MinWidth(0).Children(func() {
				ui.Text(c, t.Title).FontSize(k.bodySize()).SingleLine()
				ui.Text(c, names(t.Artists)).FontSize(k.smallSize()).TextColor(k.muted).SingleLine()
			})
		}
	})
}

// The live tile's card shows the artwork for liveRest, turns over in
// liveTurn to what plays next, which shows as long, and turns back.
const (
	liveRest = 5040 * time.Millisecond
	liveTurn = 960 * time.Millisecond
)

// liveCard is the live tile's artwork, which turns over to what plays
// next, a tile of the accent, and back, since liveSince.
func (s *shell) liveCard(c *ui.Context, t, next media.Track, size float32) {
	k := &s.lk
	// The card turns half over at the end of each rest, and the next turn
	// wakes the window.
	half := liveRest + liveTurn
	at := c.Now().Sub(s.liveSince)
	turns := int(at / half)
	if at%half >= liveRest {
		turns++
	} else {
		c.After(liveRest - at%half)
	}
	card := ui.Box(c.Key("live-card")).Size(size, size).Shrink(0).Radius(k.radiusSm).Clip()
	// A new run of turns starts from the artwork.
	angle := card.AnimateWith(s.liveSince, 180*float32(turns), liveTurn, ui.EaseInOut)
	turned := math.Mod(float64(angle), 360)
	back := turned > 90 && turned < 270
	if back {
		turned -= 180
	}
	card.RotateX(float32(turned)).Perspective(600)
	card.Children(func() {
		if !back {
			s.cover(c, "live-cover", t.Artwork, size, false)
			return
		}
		ui.Column(c).Fill().Justify(ui.SpaceBetween).Padding(6, 7).Background(k.tile(0)).Children(func() {
			ui.Text(c, "up next").FontSize(11).LineHeight(1).TextColor(k.tileText)
			ui.Text(c, next.Title).FontSize(12).LineHeight(1.15).MaxLines(3).TextColor(k.tileText)
		})
	})
}

// noticeOf tells an error as a notice.
func noticeOf(err error) player.Notice { return player.Notice{Message: err.Error(), Error: true} }

// notice tells something that went well.
func notice(message string) player.Notice { return player.Notice{Message: message} }
