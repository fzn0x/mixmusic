package main

// The head of a playlist, an album, an artist or the liked songs, in the
// top bar once the page scrolled it away: the page tells where its head
// is down it, and the frame where it scrolled.

import (
	"math"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
)

// shownPage is what the window knows of the page it shows, for the top
// bar, which builds before the page.
type shownPage struct {
	// path is the page's path.
	path string
	// scroll is where the page scrolled: its scroll container keeps it,
	// and as the view builds, it holds where the frame shows the page.
	scroll ui.ScrollState
	// content is the page's content, while the page builds, which what the
	// page measures is down from.
	content ui.Element
	// bar is what the top bar shows of the page's head once it scrolled
	// away, nil for a page without one, and barAt how far the page
	// scrolls before it does: until the head's buttons are half under
	// the top bar. It's 0 until the head was laid out.
	bar   *barHead
	barAt float32
	// built is set once the page built its scroll container in the build
	// of the view, and was in the build before; fresh while the page
	// builds in the first build that shows it.
	built, was, fresh bool
}

// startPage starts a build of the view: the page shown builds again.
func (s *shell) startPage() {
	sp := &s.here
	sp.was, sp.built, sp.content = sp.built, false, ui.Element{}
}

// showPage takes the scroll container page of the page shown as it
// builds, and returns where it keeps where it scrolled. A page new to the
// window starts without a head, until it was laid out.
func (s *shell) showPage(page ui.Element) *ui.ScrollState {
	sp := &s.here
	if !sp.was || sp.path != s.pagePath {
		*sp = shownPage{path: s.pagePath, fresh: true}
	}
	sp.built, sp.bar = true, nil
	return s.pageScrolled()
}

// takeContent takes col, the content of the page shown, as it builds in
// its scroll container page. The router keeps the pages of the history
// where they scrolled: shown again, a page goes on from where its last
// frame laid it out, and a new one from its top.
func (s *shell) takeContent(page, col ui.Element) {
	sp := &s.here
	sp.content = col
	if !sp.fresh {
		return
	}
	sp.fresh = false
	if s.capScroll != nil {
		return // capture mode scrolls it
	}
	sp.scroll.Y = 0
	if b, in := page.LayoutBounds(), col.LayoutBounds(); b != (ui.Rect{}) && in != (ui.Rect{}) {
		sp.scroll.Y = max(b.Y-in.Y, 0)
	}
}

// pageScrolled is where the page shown scrolled: capture mode scrolls it
// itself.
func (s *shell) pageScrolled() *ui.ScrollState {
	if s.capScroll != nil {
		return s.capScroll
	}
	return &s.here.scroll
}

// pageY is how far the page shown scrolled, as the frame shows it: as far
// as the frame before laid it out for the end, which math.MaxFloat32 asks
// for.
func (s *shell) pageY() float32 {
	sc := s.pageScrolled()
	if sc.Y >= math.MaxFloat32/2 {
		return sc.MaxY
	}
	return max(sc.Y, 0)
}

// downPage returns how far down the page shown e's top and bottom were, as
// the frame before laid them out, false for an element of a page that
// isn't shown, or not laid out yet.
func (s *shell) downPage(e ui.Element) (top, bottom float32, ok bool) {
	sp := &s.here
	if !sp.built || s.pagePath != sp.path {
		return 0, 0, false
	}
	b, in := e.LayoutBounds(), sp.content.LayoutBounds()
	if b == (ui.Rect{}) || in == (ui.Rect{}) {
		return 0, 0, false
	}
	return b.Y - in.Y, b.Y + b.H - in.Y, true
}

// barHead is what the top bar shows of a page's head once the page
// scrolled it away: the collection's artwork, round for an artist, or a
// glyph for what has none, its title, and the head's actions.
type barHead struct {
	title    string
	art      []media.Image
	round    bool
	glyph    *ui.SVG
	play     func()
	shuffle  func()
	download *downloadHead
	menu     func(m *ui.Menu)
}

// headed tells the top bar of the head of the page shown: row, whose
// foot holds its buttons, and what the top bar shows of it once they're
// half under the top bar. A page that isn't shown tells nothing.
func (s *shell) headed(row ui.Element, bar barHead) {
	sp := &s.here
	if !sp.built || s.pagePath != sp.path {
		return
	}
	sp.bar = &bar
	if _, bottom, ok := s.downPage(row); ok {
		sp.barAt = max(bottom-20, 1)
	}
}

// shownHead is the head the top bar shows, if the page shown has one, and
// whether the page scrolled it away.
func (s *shell) shownHead() (*barHead, bool) {
	sp := &s.here
	if !sp.was || sp.path != s.router.Path() || sp.bar == nil || sp.barAt <= 0 {
		return nil, false
	}
	return sp.bar, s.pageY() >= sp.barAt
}

// barRoom is how wide the top bar's head is at least for its download
// button: narrower, the download is in its menu.
const barRoom = 300

// topBarHead takes the room of the top bar between the search box and the
// command palette's button. Once the page shown scrolled its head away, it
// shows the head there, small: the artwork, the title, which an ellipsis
// ends where it's long, and the head's play, shuffle and download, which
// fade in, and out as the head comes back, at once while the computer
// asks for less motion. It builds nothing while it shows nothing.
func (s *shell) topBarHead(c *ui.Context) {
	k := &s.lk
	row := ui.Row(c.Key("bar-head")).Grow(1).Basis(0).MinWidth(0).AlignSelf(ui.Stretch).
		Gap(4).PaddingX(4).AlignItems(ui.Center).Clip()
	h, away := s.shownHead()
	if h == nil {
		// Another page's head never fades out over this one.
		row.Jump("fade", 0)
		return
	}
	target := float32(0)
	if away {
		target = 1
	}
	fade := row.AnimateTo("fade", target, k.spring(false))
	if fade <= 0 {
		return
	}
	row.Opacity(fade)
	wide := row.Bounds().W >= barRoom
	download := h.download != nil && s.anyDownloadable(h.download.tracks)
	menu := h.menu
	if menu == nil && download && !wide {
		// The download alone, in the menu.
		menu = func(m *ui.Menu) {
			if a := s.downloadOf(h.download); m.Item(a.item).Chosen() {
				a.run()
			}
		}
	}
	row.Children(func() {
		switch {
		case h.glyph != nil:
			ui.Box(c).Size(32, 32).Shrink(0).Radius(k.radiusSm).Background(k.tile(1)).Center().Children(func() {
				ui.Icon(c, h.glyph).FontSize(16).TextColor(k.tileText)
			})
		default:
			s.cover(c, "bar-cover", h.art, 32, h.round)
		}
		name := ui.Text(c, k.words(h.title)).FontSize(k.bodySize()).Font(k.display).SingleLine().MinWidth(0).
			Margin(0, 8, 0, 6)
		k.labelWeight(name)
		if h.play != nil && s.round(c, "bar-play", "play", "Play "+h.title, true, false, 32, 16).Shrink(0).Clicked() {
			h.play()
		}
		if h.shuffle != nil && s.iconButton(c, "bar-shuffle", "shuffle", "Shuffle "+h.title, false, 18).Shrink(0).Clicked() {
			h.shuffle()
		}
		if download && wide {
			a := s.downloadOf(h.download)
			if s.iconButton(c, "bar-download", "download", k.sentence(a.label), a.done, 18).Shrink(0).Clicked() {
				a.run()
			}
		}
		if menu != nil {
			s.iconButton(c, "bar-more", "more", "More for "+h.title, false, 18).Shrink(0).Menu(menu)
		}
	})
}
