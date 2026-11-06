package main

import (
	"math"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/resources"
)

// navItem is a page of the rail.
type navItem struct {
	id, label, icon, path string
}

var nav = []navItem{
	{"home", "Home", "home", "/"},
	{"search", "Search", "search", "/search"},
	{"explore", "Explore", "compass", "/explore"},
	{"library", "Library", "library", "/library"},
	{"stats", "Stats", "stats", "/stats"},
	{"marketplace", "Marketplace", "store", "/marketplace"},
	{"settings", "Settings", "settings", "/settings"},
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
	if _, err := s.svc.Preferences.Update(func(st *settings.Settings) { st.SidebarCollapsed = folded }); err != nil {
		s.notes = append(s.notes, noticeOf(err))
	}
}

// The rail's parts are railGap apart, over its foot's padding of railFoot,
// and the mixes take mixesLeast at least: their title and a mix and a
// half. In a short window the entries of tonal controls tighten as far as
// tightEntry, the least height Apple's guidelines give what is clicked,
// to keep the live tile's room.
const (
	railGap    = 24
	railFoot   = 16
	mixesLeast = 88
	tightEntry = 44
)

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
	items := s.plugins.SidebarItems()
	entries := float32(len(nav) + len(items))
	navLeast := entries*s.leastEntry() + (entries-1)*4
	ui.Column(c).Width(s.railWidth()).Shrink(0).Padding(0, pad, railFoot).Gap(railGap).Background(k.bg).Clip().Children(func() {
		least := s.brand(c, collapsed, s.railWidth()-2*pad) + railGap + navLeast + railGap + railFoot
		// Where the rail is short, and only there, as the mixes grow from
		// nothing into the rest, the entries give some of their height.
		ui.Column(c).Gap(4).MinHeight(navLeast).Children(func() {
			for _, n := range nav {
				s.navEntry(c, "nav-"+n.id, k.words(n.label), icon(n.icon), n.path, n.id == current)
			}
			for _, it := range items {
				s.navEntry(c, "plugin-"+it.ID, k.words(it.Label), s.svgOf(it.Icon), it.Path, path == it.Path)
			}
		})
		if !collapsed {
			s.mixes(c, path)
			least += mixesLeast
		} else {
			ui.Spacer(c)
		}
		// The live tile shows where the rail has room for it whole, with
		// the entries tight and the mixes short, above the player bar,
		// which covered it in a short window when the rail ran on (#258).
		_, h := c.Size()
		if h-k.barRoom()-least >= railGap+liveSize(collapsed) {
			s.liveTile(c, collapsed)
		}
	})
}

// brandMark is MixMusic's mark, the pink rings with a play button of its
// icon, and brandPink their colour.
var (
	brandMark = mustSVG(resources.Mark)
	brandPink = ui.RGB(0xe6, 0x00, 0x7e)
)

// mustSVG parses an SVG MixMusic ships.
func mustSVG(data []byte) *ui.SVG {
	svg, err := ui.ParseSVG(data)
	if err != nil {
		panic(err)
	}
	return svg
}

// brand is MixMusic's mark and name, where the window drags under a
// hidden title bar, and the button that collapses the rail, in inner DIPs,
// the rail's width inside its padding. On macOS the window's buttons take
// the left of that row, where the name doesn't fit beside them, nor the
// button in the folded rail (#247): the row keeps them, and the button
// where it fits, and the mark and name go under it. It tells its height.
func (s *shell) brand(c *ui.Context, collapsed bool, inner float32) float32 {
	bar := c.TitleBar()
	tall := max(bar.Height, 48)
	band := func(children func()) {
		row := ui.Row(c).Height(tall).Shrink(0).Padding(0, 0, 0, bar.Left).Gap(12)
		if bar.Height > 0 {
			row.DragWindow()
		}
		row.Children(children)
	}
	if bar.Left == 0 {
		band(func() {
			if !collapsed {
				// The mark, the two gaps and the button leave the name the
				// rest.
				s.logo(c, inner-26-2*12-32)
			}
			s.railButton(c, collapsed)
		})
		return tall
	}
	beside := inner-bar.Left >= 12+32
	ui.Column(c).Shrink(0).Children(func() {
		band(func() {
			if beside {
				ui.Spacer(c)
				s.railButton(c, collapsed)
			}
		})
		if collapsed && beside {
			return
		}
		ui.Row(c).Height(40).Shrink(0).Gap(12).Children(func() {
			if !collapsed {
				room := inner - 26 - 12
				if !beside {
					room -= 12 + 32
				}
				s.logo(c, room)
			}
			if !beside {
				s.railButton(c, collapsed)
			}
		})
	})
	if collapsed && beside {
		return tall
	}
	return tall + 40
}

// logo is MixMusic's mark, and its name in what is left of room DIPs.
func (s *shell) logo(c *ui.Context, room float32) {
	k := &s.lk
	// The mark keeps its pink in every theme, as the app's icon does.
	ui.Icon(c, brandMark).Size(26, 26).Shrink(0).TextColor(brandPink)
	size := s.wordmarkSize(c, room)
	ui.Text(c, "mixmusic").FontSize(size).FontWeight(k.displayWeight()).Font(k.display).LetterSpacing(k.tracking(-0.03, size)).
		SingleLine().Grow(1).MinWidth(0)
}

// railButton folds or unfolds the rail.
func (s *shell) railButton(c *ui.Context, collapsed bool) {
	label := "Collapse the sidebar"
	if collapsed {
		label = "Expand the sidebar"
	}
	if s.iconButton(c, "collapse", "sidebar", label, false, 18).Clicked() {
		s.toggleRail()
	}
}

// wordmarkFit is the size of MixMusic's name in the rail, the face, weight
// and room it was measured for, and its width at 24.
type wordmarkFit struct {
	font   string
	weight int
	room   float32
	size   float32
	width  float32
}

// wordmarkSize is the size of MixMusic's name in room DIPs: 24, or less
// when the theme's face is too wide for it there, as Sakura's Georgia is,
// so that the name never shows cut. It measures the name once for each
// face, weight and room, not every frame.
func (s *shell) wordmarkSize(c *ui.Context, room float32) float32 {
	k := &s.lk
	f := wordmarkFit{font: k.display, weight: k.displayWeight(), room: room}
	if s.wordmark.size > 0 && s.wordmark.font == f.font && s.wordmark.weight == f.weight && s.wordmark.room == f.room {
		return s.wordmark.size
	}
	f.width, _ = c.MeasureText(0, ui.Span{Text: "mixmusic", Font: f.font, Size: 24, Weight: f.weight, LetterSpacing: k.tracking(-0.03, 24)})
	f.size = fitSize(24, f.width, room)
	s.wordmark = f
	return f.size
}

// fitSize is the size of a text that measures w at size, to fit in room:
// size when it fits, else smaller by halves of a DIP, but never below
// two thirds of size, where a name stops reading as a name.
func fitSize(size, w, room float32) float32 {
	if w <= room || w <= 0 {
		return size
	}
	fit := float32(math.Floor(float64(size*room/w)*2)) / 2
	return max(fit, float32(math.Round(float64(size)*2/3)))
}

// navEntry is an outlined box of an icon with its lowercase label; the
// current page floods its box with the accent. Tonal controls show it as
// an icon and its label on an indicator (tonalNav).
func (s *shell) navEntry(c *ui.Context, key, label string, svg *ui.SVG, path string, current bool) {
	k := &s.lk
	if k.tonal {
		s.tonalNav(c, key, label, svg, path, current)
		return
	}
	row := s.buttonBase(c.Key(key)).Gap(12).Justify(ui.Start).Label(label).Cursor(ui.CursorPointer)
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
			// Selawik's labels weigh 350, in a family of their own; a
			// theme's face of its own sets them at its regular weight.
			font := faceSemilight
			if k.body != face {
				font = k.body
			}
			ui.Text(c, label).FontSize(17 * k.scale).Font(font).TextColor(ink).SingleLine()
		}
	})
}

// tonalEntry is the height of a page's entry in the rail of tonal
// controls.
func (k *look) tonalEntry() float32 { return min(56, k.row-8) }

// leastEntry is the least height of a page's entry in the rail: a tonal
// one tightens to tightEntry in a short window, and an outlined one keeps
// its box.
func (s *shell) leastEntry() float32 {
	if s.lk.tonal {
		return min(s.lk.tonalEntry(), tightEntry)
	}
	return 42
}

// tonalNav is a page of the rail as tonal controls show it: its icon and
// label, the current one on an indicator in the fill of what is chosen,
// which grows from the middle on the theme's spring as it becomes current.
func (s *shell) tonalNav(c *ui.Context, key, label string, svg *ui.SVG, path string, current bool) {
	k := &s.lk
	folded := s.railFolded()
	h := k.tonalEntry()
	row := s.buttonBase(c.Key(key)).Height(h).MinHeight(s.leastEntry()).Shrink(1).Gap(14).Padding(0, 16).Justify(ui.Start).Label(label).Cursor(ui.CursorPointer)
	if folded {
		row.Tooltip(label).Padding(0).Justify(ui.Center)
	}
	if row.Clicked() {
		s.navigate(path)
	}
	switch {
	case row.Pressed() && !current:
		row.Background(k.press).Radius(k.round(h))
	case row.Hovered() && !current:
		row.Background(k.hover).Radius(k.round(h))
	}
	grown := float32(0)
	if current {
		grown = 1
	}
	if k.moves() {
		grown = row.AnimateTo("indicator", grown, k.spring(true))
	}
	if grown > 0.01 {
		fill, radius := k.selected, k.round(h)
		row.Draw(func(p *ui.Painter, r ui.Rect) {
			w := r.W * (0.5 + 0.5*grown)
			p.Fill(ui.Rect{X: r.X + (r.W-w)/2, Y: r.Y, W: w, H: r.H}, withAlpha(fill, float64(min(1, 2*grown))), min(radius, w/2))
		})
	}
	ink := k.text
	if current {
		ink = k.selectedText
	}
	row.Children(func() {
		ui.Icon(c, svg).FontSize(22).TextColor(ink)
		if !folded {
			t := ui.Text(c, label).FontSize(k.size(0.875)).TextColor(ink).SingleLine()
			k.labelWeight(t)
		}
	})
}

// mixes lists liked songs and the user's mixes, with their counts.
func (s *shell) mixes(c *ui.Context, path string) {
	k := &s.lk
	ui.Column(c).Grow(1).MinHeight(mixesLeast).Children(func() {
		ui.Row(c).Justify(ui.SpaceBetween).Margin(0, 0, 8).Children(func() {
			ui.Text(c, k.words("Your mixes")).FontSize(k.bodySize()).TextColor(k.accent)
			if s.iconButton(c, "new-mix", "plus", "New mix", false, 18).Clicked() {
				s.newMix()
			}
		})
		ui.Scroll(c).Grow(1).MinHeight(0).Children(func() {
			s.sideLink(c, "liked", k.words("Liked songs"), len(s.svc.Collection.Liked()), "/library/liked", path)
			if s.kept.Any() {
				s.sideLink(c, "downloads", k.words("Downloads"), s.kept.Status().Tracks, "/library/downloads", path)
			}
			for _, m := range s.svc.Collection.Playlists() {
				s.sideLink(c, "mix-"+m.ID, m.Title, m.TrackCount, playlistPath(m.ID), path)
			}
		})
	})
}

func (s *shell) sideLink(c *ui.Context, key, title string, n int, to, path string) {
	k := &s.lk
	link := s.buttonBase(c.Key(key)).Justify(ui.SpaceBetween).Gap(12).Padding(7, 0).Label(title).Cursor(ui.CursorPointer)
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

// liveSize is the size of the live tile's artwork, and its height: the
// line it shows while nothing plays is shorter.
func liveSize(collapsed bool) float32 {
	if collapsed {
		return 42
	}
	return 76
}

// liveTile shows what plays, and opens now playing.
func (s *shell) liveTile(c *ui.Context, collapsed bool) {
	k := &s.lk
	cur, ok := s.st.Current()
	if !ok {
		if !collapsed {
			ui.Column(c).Padding(12, 0).BorderWidth(1, 0, 0, 0).BorderColor(k.rule).Shrink(0).Children(func() {
				ui.Text(c, k.words("Nothing playing yet")).FontSize(k.smallSize()).TextColor(k.muted)
			})
		}
		return
	}
	size := liveSize(collapsed)
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
	tile := s.buttonBase(c.Key("live")).Gap(12).Justify(ui.Start).Shrink(0).Label("Open now playing").Cursor(ui.CursorPointer)
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
	// Neither face clips: each rounds its own corners. A clip turning in 3D
	// is drawn through a mask, which MyGo made anew for frames of each turn
	// and kept: a hundred megabytes and more while something played (#309).
	card := ui.Box(c.Key("live-card")).Size(size, size).Shrink(0)
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
			s.liveFace(c, t.Artwork, size)
			return
		}
		ui.Column(c).Fill().Radius(k.radiusSm).Justify(ui.SpaceBetween).Padding(6, 7).Background(k.tile(0)).Children(func() {
			ui.Text(c, k.words("Up next")).FontSize(11).LineHeight(1).TextColor(k.tileText)
			ui.Text(c, next.Title).FontSize(12).LineHeight(1.15).MaxLines(3).TextColor(k.tileText)
		})
	})
}

// liveFace is the artwork on the live card's face, as cover shows it but
// without its clip: the image rounds its own corners, and crops to fill
// the square.
func (s *shell) liveFace(c *ui.Context, art []media.Image, size float32) {
	k := &s.lk
	if b := s.art.get(c, artworkURL(art, int(size*s.zoomOf())), s.artPixels(size)); b != nil {
		ui.Image(c, b).Size(size, size).Fit(ui.Cover).Radius(k.radiusSm)
		return
	}
	ui.Box(c).Size(size, size).Radius(k.radiusSm).Background(k.surface3)
}

// noticeOf tells an error as a notice.
func noticeOf(err error) player.Notice { return player.Notice{Message: err.Error(), Error: true} }

// notice tells something that went well.
func notice(message string) player.Notice { return player.Notice{Message: message} }
