package main

import (
	"math"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
)

// iconButton is a 32px button of an icon, in muted ink, which turns to ink
// over a tint on hover, and to the accent when on.
func (s *shell) iconButton(c *ui.Context, key, name, label string, on bool, size float32) ui.Element {
	k := &s.lk
	b := s.buttonBase(c.Key(key)).Size(32, 32).Radius(k.round(32)).Label(label).Tooltip(label).Cursor(ui.CursorPointer)
	ink := k.muted
	switch {
	case on:
		ink = k.accent
	case b.Hovered():
		ink = k.text
	}
	if b.Pressed() {
		b.Background(k.press)
	} else if b.Hovered() {
		b.Background(k.hover)
	}
	s.pressMotion(b, 0.9)
	b.Children(func() { ui.Icon(c, icon(name)).FontSize(size).TextColor(ink) })
	return b
}

// roundButton is a transport button: a 38px circle with a 2px outline, or,
// as the main button, a 48px solid ink circle that turns to the accent on
// hover. With tonal controls, the main one is the accent's, and the
// others an icon on a tint while hovered.
func (s *shell) roundButton(c *ui.Context, key, name, label string, main, disabled bool) ui.Element {
	if main {
		return s.round(c, key, name, label, true, disabled, 48, 22)
	}
	return s.round(c, key, name, label, false, disabled, 38, 18)
}

// round is a round button of size, with a glyph of the size given.
func (s *shell) round(c *ui.Context, key, name, label string, main, disabled bool, size, glyph float32) ui.Element {
	k := &s.lk
	b := s.buttonBase(c.Key(key)).Size(size, size).Radius(size / 2).Label(label).Tooltip(label).
		Cursor(ui.CursorPointer).Disabled(disabled)
	ink := k.text
	switch {
	case k.tonal:
		ink = s.tonalRound(b, main, disabled)
	case disabled:
		b.Opacity(0.45)
		if main {
			b.Background(k.text)
			ink = k.bg
		} else {
			b.Border(2, k.outline)
		}
	case main && b.Hovered():
		b.Background(k.accent)
		ink = k.accentText
	case main:
		b.Background(k.text)
		ink = k.bg
	case b.Pressed():
		b.Background(k.text).Border(2, k.text)
		ink = k.bg
	case b.Hovered():
		b.Background(k.hover).Border(2, k.outline)
	default:
		b.Border(2, k.outline)
	}
	switch {
	case k.motion == motionExpressive:
		// The shape follows what the button does: the play button, round
		// while paused, squares off while it plays, as Material's toggles
		// do once on, and squeezes as it is pressed.
		r := size / 2
		if main && name == "pause" {
			r = 0.3 * size
		}
		if !disabled && b.Pressed() {
			r *= 0.6
		}
		b.Radius(b.AnimateTo("shape", r, k.spring(true)))
		s.pressMotion(b, 0.92)
	case k.moves():
		s.pressMotion(b, 0.94)
	case !disabled && b.Pressed():
		b.Scale(0.97, 0.97)
	}
	b.Children(func() { ui.Icon(c, icon(name)).FontSize(glyph).TextColor(ink) })
	return b
}

// tonalRound fills a transport button b as tonal controls do, and
// returns its ink.
func (s *shell) tonalRound(b ui.Element, main, disabled bool) ui.Color {
	k := &s.lk
	switch {
	case main && disabled:
		b.Background(k.accent).Opacity(0.45)
	case main && b.Pressed():
		b.Background(k.accent.Mix(k.bg, 0.14))
	case main && b.Hovered():
		b.Background(k.accent.Mix(k.text, 0.14))
	case main:
		b.Background(k.accent)
	case disabled:
		b.Opacity(0.45)
		return k.text
	case b.Pressed():
		b.Background(k.press)
	case b.Hovered():
		b.Background(k.hover)
	default:
		return k.text
	}
	if main {
		return k.accentText
	}
	return k.text
}

// pressMotion squeezes b to scale while pressed, on the theme's spring,
// and lets it go back: themes whose motion is standard press at once,
// where the button does.
func (s *shell) pressMotion(b ui.Element, scale float32) {
	k := &s.lk
	if !k.moves() {
		return
	}
	target := float32(1)
	if b.Pressed() {
		target = scale
	}
	// A button at rest, and one that never moved, keeps no spring going.
	if target != 1 || !b.Settled("press") {
		v := b.AnimateTo("press", target, k.spring(true))
		b.Scale(v, v)
	}
}

// pointerKeys leaves the keys without modifiers that run commands, as
// Space plays or pauses, to those commands while e has the focus and shows
// no ring for it, as after a click: MyGo gives a focused control Space and
// Enter before the window's shortcuts, and the control would act again. A
// focus that shows its ring keeps them, as one that Tab gave, for Space and
// Enter to press what Tab reached; so does one whose ring a press of Space
// or Enter showed. It does nothing while a shortcut is being recorded.
func (s *shell) pointerKeys(e ui.Element) ui.Element {
	if len(s.plainKeys) == 0 || !e.Focused() || e.FocusVisible() {
		return e
	}
	for _, k := range s.plainKeys {
		if e.Shortcut(0, k.key) {
			k.run()
		}
	}
	return e
}

// buttonBase, switchOf, selectOf, linkOf and colorWellOf are MyGo's
// controls, whose focus after a click leaves Space to play or pause
// (pointerKeys).
func (s *shell) buttonBase(c *ui.Context) ui.Element         { return s.pointerKeys(ui.ButtonBase(c)) }
func (s *shell) switchOf(c *ui.Context, on *bool) ui.Element { return s.pointerKeys(ui.Switch(c, on)) }
func (s *shell) selectOf(c *ui.Context, chosen *string, options []string) ui.Element {
	return s.pointerKeys(ui.Select(c, chosen, options))
}
func (s *shell) linkOf(c *ui.Context, label, url string) ui.Element {
	return s.pointerKeys(ui.Link(c, label, url))
}
func (s *shell) colorWellOf(c *ui.Context, color *ui.Color) ui.Element {
	return s.pointerKeys(ui.ColorWell(c, color))
}

// dialogButton is button for a dialog, where the window's shortcuts wait:
// Space and Enter press it after a click too, as they press the buttons of
// MyGo's own dialogs.
func (s *shell) dialogButton(c *ui.Context, key, label string, primary bool) ui.Element {
	return s.styledButton(ui.ButtonBase(c.Key(key)), c, label, primary)
}

// button is a 40px button with a label: a filled accent block when
// primary, else a 2px outline that inverts to solid ink when pressed, or,
// with tonal controls, a tonal fill.
func (s *shell) button(c *ui.Context, key, label string, primary bool) ui.Element {
	return s.styledButton(s.buttonBase(c.Key(key)), c, label, primary)
}

// styledButton gives b, a button's base, the look of button.
func (s *shell) styledButton(b ui.Element, c *ui.Context, label string, primary bool) ui.Element {
	k := &s.lk
	b.Height(40).PaddingX(24).Radius(k.round(40)).Label(label).Cursor(ui.CursorPointer).AlignSelf(ui.Start)
	ink := k.text
	switch {
	case k.tonal && !primary && b.Pressed():
		b.Background(k.tonalFill.Mix(k.tonalInk, 0.18))
		ink = k.tonalInk
	case k.tonal && !primary && b.Hovered():
		b.Background(k.tonalFill.Mix(k.tonalInk, 0.1))
		ink = k.tonalInk
	case k.tonal && !primary:
		b.Background(k.tonalFill)
		ink = k.tonalInk
	case primary && b.Hovered():
		b.Background(k.accent.Mix(k.text, 0.14))
		ink = k.accentText
	case primary:
		b.Background(k.accent)
		ink = k.accentText
	case b.Pressed():
		b.Background(k.text).Border(2, k.text)
		ink = k.bg
	case b.Hovered():
		b.Background(k.hover).Border(2, k.outline)
	default:
		b.Border(2, k.outline)
	}
	if k.moves() {
		s.pressMotion(b, 0.94)
	} else if b.Pressed() {
		b.Scale(0.97, 0.97)
	}
	// The label keeps its width: the button's, made of it, may round a
	// fraction of a pixel short of it, and cut its last letter.
	b.Children(func() {
		t := ui.Text(c, k.sentence(label)).FontSize(k.bodySize()).TextColor(ink).SingleLine().Shrink(0)
		k.labelWeight(t)
	})
	return b
}

// pageHeader names the page in one giant light lowercase word, which
// shrinks to fit the pane on one line, and wraps under 45% of its size,
// never cut. Pivots under it switch the page's sections; selected is the
// current one's index. Themes of other headings name it as written, at
// their sizes, over chips (plainHeader).
func (s *shell) pageHeader(c *ui.Context, title string, pivots []string, selected *int) {
	s.pageHeaderCounts(c, title, pivots, nil, selected)
}

// pageHeaderCounts is a page's header whose pivots count what they show:
// counts[i], or nothing where it's below 0.
func (s *shell) pageHeaderCounts(c *ui.Context, title string, pivots []string, counts []int, selected *int) {
	k := &s.lk
	if k.headings != headingsLight {
		s.plainHeader(c, title, pivots, counts, selected)
		return
	}
	word := strings.ToLower(title)
	// Terminal's titles are prompts.
	prompt := ""
	if k.fx.prompt {
		prompt = "> "
	}
	pane := s.paneWidth(c) - 2*k.gutter
	size := min(max(56, 0.14*(pane+2*k.gutter)), 144) * k.scale
	// Selawik Light at the page word's tracking is about 0.47em a letter.
	natural := 0.47 * size * float32(utf8.RuneCountInString(prompt+word))
	fit := float32(1)
	if natural > pane && natural > 0 {
		fit = pane / natural
	}
	wrap := fit < 0.45
	if wrap {
		fit = 0.45
	}
	// heading sets the title, or, for its glow, its blur's ink.
	heading := func(glow bool) ui.Element {
		// The space after the prompt takes no room at the end of its text:
		// the gap stands for it.
		row := ui.Row(c).AlignItems(ui.Start).Gap(0.5 * size * fit)
		row.Children(func() {
			text := func(s string, ink ui.Color) ui.Element {
				t := ui.Text(c, s).FontSize(size * fit).FontWeight(300).Font(k.display).LetterSpacing(-0.045 * size * fit).
					LineHeight(0.86).TextColor(ink)
				if glow {
					t.TextColor(withAlpha(k.accent, 0.45))
				}
				if k.fx.italic {
					t.Italic()
				}
				return t
			}
			if prompt != "" {
				text(strings.TrimSpace(prompt), k.accent).SingleLine().Shrink(0)
			}
			t := text(word, k.text).MinWidth(0)
			if wrap {
				t.LineHeight(0.95)
			} else {
				t.SingleLine()
			}
		})
		return row
	}
	ui.Column(c).Padding(8, 0, 32).Gap(12).Children(func() {
		if k.fx.glow > 0 {
			// The phosphor's glow: text-shadow 0 0 glow, whose blur is
			// twice its Gaussian's sigma.
			ui.Box(c).Children(func() {
				heading(true).Absolute().Top(0).Right(0).Left(0).Blur(k.fx.glow / 2).PassThrough()
				heading(false)
			})
		} else {
			heading(false)
		}
		if len(pivots) > 0 && selected != nil {
			ui.Row(c).Gap(0.66 * k.pivotSize()).Wrap().Children(func() {
				for i, p := range pivots {
					p := k.words(p)
					b := s.buttonBase(c.Key("pivot-" + p)).Label(p).Cursor(ui.CursorPointer)
					ink := k.text.Alpha(0.38)
					switch {
					case i == *selected:
						ink = k.text
					case b.Hovered():
						ink = k.text.Alpha(0.7)
					}
					if b.Clicked() {
						*selected = i
					}
					b.Children(func() {
						ui.Text(c, p).FontSize(k.pivotSize()).FontWeight(300).Font(k.display).
							LetterSpacing(-0.02 * k.pivotSize()).TextColor(ink).SingleLine()
						if i < len(counts) && counts[i] >= 0 {
							ui.Text(c, count(counts[i])).FontSize(k.captionSize()).TextColor(ink).FontFeatures("tnum").
								AlignSelf(ui.Start).Margin(6, 0, 0, 3)
						}
					})
				}
			})
		}
	})
}

// plainHeader names the page as written, at the sizes of the theme's
// headings: Material's display sizes, or larger and heavier for bold
// ones, as the pane widens. It shrinks to fit the pane on one line, down
// to 60% of its size, and wraps under that. Chips under it switch the
// page's sections.
func (s *shell) plainHeader(c *ui.Context, title string, pivots []string, counts []int, selected *int) {
	k := &s.lk
	pane := s.paneWidth(c) - 2*k.gutter
	window := pane + 2*k.gutter
	size, letter, leading := min(max(36, 0.045*window), 57), float32(0.5), float32(1.12)
	if k.headings == headingsBold {
		size, letter, leading = min(max(40, 0.06*window), 76), 0.52, 1.0
	}
	size *= k.scale
	natural := letter * size * float32(utf8.RuneCountInString(title))
	fit := float32(1)
	if natural > pane && natural > 0 {
		fit = pane / natural
	}
	wrap := fit < 0.6
	if wrap {
		fit = 0.6
	}
	ui.Column(c).Padding(16, 0, 28).Gap(20).Children(func() {
		t := ui.Text(c, title).FontSize(size * fit).FontWeight(k.displayWeight()).Font(k.display).
			LetterSpacing(k.tracking(-0.02, size*fit)).LineHeight(leading).TextColor(k.text).MinWidth(0)
		if !wrap {
			t.SingleLine()
		}
		if len(pivots) > 0 && selected != nil {
			s.chips(c, pivots, counts, selected)
		}
	})
}

// chips switch a page's sections: filled pills, or boxes, the chosen one
// in the fill of what is chosen; counts[i] counts what one shows, unless
// it's below 0.
func (s *shell) chips(c *ui.Context, pivots []string, counts []int, selected *int) {
	k := &s.lk
	ui.Row(c).Gap(8).Wrap().Children(func() {
		for i, p := range pivots {
			b := s.buttonBase(c.Key("pivot-" + p)).Height(32).PaddingX(16).Gap(6).Radius(k.round(32)).Label(p).Cursor(ui.CursorPointer)
			fill, ink := k.surface2, k.text
			switch {
			case i == *selected:
				fill, ink = k.selected, k.selectedText
			case b.Pressed():
				fill = k.surface3
			case b.Hovered():
				fill = k.surface2.Mix(k.text, 0.08)
			}
			b.Background(fill)
			if b.Clicked() {
				*selected = i
			}
			s.pressMotion(b, 0.95)
			b.Children(func() {
				t := ui.Text(c, k.sentence(p)).FontSize(k.smallSize()).TextColor(ink).SingleLine()
				k.labelWeight(t)
				if i < len(counts) && counts[i] >= 0 {
					ui.Text(c, count(counts[i])).FontSize(k.captionSize()).TextColor(ink.Alpha(0.8)).FontFeatures("tnum")
				}
			})
		}
	})
}

// sectionTitle titles a section over its content, on a 1px accent rule.
func (s *shell) sectionTitle(c *ui.Context, title string) { s.sectionHead(c, title, nil) }

// sectionHead is a section's title, with links to what relates to it at
// its end: on a rule in the light headings, as written and heavier,
// without one, in the others.
func (s *shell) sectionHead(c *ui.Context, title string, links func()) {
	k := &s.lk
	if k.headings != headingsLight {
		size := 22 * k.scale
		if k.headings == headingsBold {
			size = 24 * k.scale
		}
		ui.Row(c).Margin(0, 0, 12).Gap(16).Children(func() {
			ui.Text(c, title).FontSize(size).FontWeight(k.titleWeight()).Font(k.display).
				LetterSpacing(k.tracking(-0.02, size)).TextColor(k.text).SingleLine().Grow(1).MinWidth(0)
			if links != nil {
				ui.Row(c).Gap(16).Shrink(0).Children(links)
			}
		})
		return
	}
	ui.Row(c).Margin(0, 0, 16).Padding(0, 0, 8).Gap(16).AlignItems(ui.End).BorderWidth(0, 0, 1, 0).BorderColor(k.rule).Children(func() {
		ui.Text(c, strings.ToLower(title)).FontSize(k.titleSize()).FontWeight(300).Font(k.display).
			LetterSpacing(-0.02 * k.titleSize()).TextColor(k.text).SingleLine().Grow(1).MinWidth(0)
		if links != nil {
			ui.Row(c).Gap(16).Shrink(0).Padding(0, 0, 4).Children(links)
		}
	})
}

// link is a small text in the accent that does something when clicked.
func (s *shell) link(c *ui.Context, key, label string) bool {
	return s.linkOf(c.Key(key), s.lk.sentence(label), "").FontSize(s.lk.smallSize()).Clicked()
}

// alert tells of a problem in a coral panel, with what to do about it.
func (s *shell) alert(c *ui.Context, title, message string, actions func()) ui.Element {
	k := &s.lk
	return ui.Column(c).Padding(16).Gap(12).Background(k.danger.Alpha(0.12).Over(k.bg)).Border(1, k.danger.Alpha(0.45)).
		Radius(k.radiusSm).Children(func() {
		ui.Text(c, title).FontSize(k.subtitleSize()).FontWeight(k.leadWeight())
		if message != "" {
			ui.Text(c, message).FontSize(k.bodySize()).MaxWidth(720)
		}
		if actions != nil {
			ui.Row(c).Gap(8).Children(actions)
		}
	})
}

// sourceMark tells where a track comes from by a shape: a disc for
// Spotify, a square for YouTube Music, two rings for MixMusic's own mixes.
func (s *shell) sourceMark(c *ui.Context, source media.Source, on bool) {
	k := &s.lk
	ink := k.muted
	switch {
	case on && k.tonal:
		ink = k.selectedSub
	case on:
		ink = k.accentText
	case source == media.SourceSpotify:
		ink = k.spotify
	case source == media.SourceYouTube:
		ink = k.youtube
	}
	ui.Box(c).Size(10, 10).Shrink(0).Label(sourceName(source)).Draw(func(p *ui.Painter, r ui.Rect) {
		switch source {
		case media.SourceSpotify:
			p.Fill(ui.Rect{X: r.X + 0.8, Y: r.Y + 0.8, W: 8.4, H: 8.4}, ink, 4.2)
		case media.SourceYouTube:
			p.Fill(ui.Rect{X: r.X + 1, Y: r.Y + 1, W: 8, H: 8}, ink, 0)
		default:
			var a, b ui.Path
			a.Circle(r.X+3.6, r.Y+5, 2.8)
			b.Circle(r.X+6.4, r.Y+5, 2.8)
			p.StrokePath(&a, 1.3, ink)
			p.StrokePath(&b, 1.3, ink)
		}
	})
}

// cover is artwork at size, or a raised grey frame until it comes; round
// for artists. Only what shows in the window loads, as the web
// interface's lazy images do: a frame new to the window asks for one more
// frame, which tells whether it shows.
func (s *shell) cover(c *ui.Context, key string, images []media.Image, size float32, round bool) ui.Element {
	k := &s.lk
	r := k.radiusSm
	if round {
		r = size / 2
	}
	frame := ui.Box(c.Key(key)).Size(size, size).Radius(r).Shrink(0).Clip()
	var b *ui.Bitmap
	if s.inView(c, frame) {
		b = s.art.get(c, artworkURL(images, int(size*s.zoomOf())), s.artPixels(size))
	}
	if b == nil {
		frame.Background(k.surface3)
	}
	frame.Children(func() {
		if b != nil {
			ui.Image(c, b).Size(size, size).Fit(ui.Cover).Radius(r)
		}
	})
	return frame
}

// inView reports whether e showed in the window in the frame before; for
// an element new to the window, it asks for another frame to tell.
func (s *shell) inView(c *ui.Context, e ui.Element) bool {
	if e.InView() {
		return true
	}
	if e.Bounds() == (ui.Rect{}) {
		c.Invalidate()
	}
	return false
}

// nearRows returns the rows, of the n rows of height row of the list e
// keyed key, that showed in the window in the frame before, or within a
// window's height of it: a long list builds only those, and takes their
// room with the others, so that a frame of a list of thousands takes about
// as long as of a few. A list new to the window starts at its top.
//
// The rows near are told from where the list was, as followMoves says.
func (s *shell) nearRows(c *ui.Context, key string, e ui.Element, n int, row float32) (from, to int) {
	_, h := c.Size()
	top := s.followMoves(c, key, e)
	from = min(max(int((-h-top)/row), 0), n)
	to = min(max(int(math.Ceil(float64((2*h-top)/row))), from), n)
	return from, to
}

// farFromView reports whether e, keyed key, lay more than a window's
// height from the window in the frame before, and its height then: a
// shelf far from the view keeps its room, and builds nothing. It is told
// from where e was, as followMoves says.
func (s *shell) farFromView(c *ui.Context, key string, e ui.Element) (height float32, far bool) {
	_, h := c.Size()
	s.followMoves(c, key, e)
	b := e.Bounds()
	if b == (ui.Rect{}) {
		return 0, false
	}
	return e.LayoutBounds().H, b.Y+b.H < -h || b.Y > 2*h
}

// followMoves returns the top of e, keyed key, in the frame before. What
// is near the view is told from there: when e moved since the frame
// before, as when its page scrolls, followMoves asks for another frame,
// for what a jump, as to the page's end, brought into view.
func (s *shell) followMoves(c *ui.Context, key string, e ui.Element) float32 {
	top := e.Bounds().Y
	s.moved(c, key, top)
	return top
}

// moved keeps where what is keyed key is, at, and asks for another frame
// when it moved since the frame before.
func (s *shell) moved(c *ui.Context, key string, at float32) {
	if was, ok := s.listTops[key]; ok && was != at {
		c.Invalidate()
	}
	s.listTops[key] = at
}

// artPixels is how many pixels artwork of size DIPs takes, at the scale of
// most displays and the window's zoom.
func (s *shell) artPixels(size float32) int { return int(size * 1.5 * s.zoomOf()) }

// zoomOf is the zoom the window shows, 1 before its first frame.
func (s *shell) zoomOf() float32 {
	if s.zoom <= 0 {
		return 1
	}
	return float32(s.zoom)
}

// loading shows that a page waits for what it shows.
func (s *shell) loading(c *ui.Context) {
	ui.Row(c).PaddingY(24).Children(func() { ui.Spinner(c).Label("Loading") })
}

// problem tells why a page can't show what it should, in coral, with a way
// to try again.
func (s *shell) problem(c *ui.Context, err error, retry func()) {
	k := &s.lk
	ui.Column(c).Padding(16).Gap(12).Background(k.danger.Alpha(0.12).Over(k.bg)).Border(1, k.danger.Alpha(0.45)).
		Radius(k.radiusSm).AlignSelf(ui.Stretch).Children(func() {
		ui.Text(c, err.Error()).FontSize(k.bodySize()).TextColor(k.text)
		if retry != nil && s.button(c, "retry", "try again", false).Clicked() {
			retry()
		}
	})
}

// emptyLine is a muted line that tells a list has nothing.
func (s *shell) emptyLine(c *ui.Context, text string) {
	ui.Text(c, text).FontSize(s.lk.bodySize()).TextColor(s.lk.muted).PaddingY(8)
}

// paneWidth is the width of the main pane: the window's, but the rail's
// and the side panel's.
func (s *shell) paneWidth(c *ui.Context) float32 {
	w, _ := c.Size()
	w -= s.railWidth()
	if s.panel != "" {
		w -= s.panelWidth(c)
	}
	return max(w, 200)
}

// panelWidth is the side panel's width, narrower in narrow windows.
func (s *shell) panelWidth(c *ui.Context) float32 {
	if w, _ := c.Size(); w < 1100 {
		return 300
	}
	return s.lk.panel
}

// choice is an option of choices: its title, what it does, and whether
// it can't be chosen.
type choice struct {
	title, description string
	disabled           bool
}

// choices chooses one of a few ways, each with its title and what it does:
// a radio button each, the chosen one filled with the accent; -1 chooses
// none. It tells whether the user changed the choice.
func (s *shell) choices(c *ui.Context, key, label string, selected *int, options []choice) bool {
	k := &s.lk
	changed := false
	ui.Column(c.Key(key)).Gap(4).Padding(14, 0).Label(label).Children(func() {
		for i, o := range options {
			b := s.buttonBase(c.Key(key+"-"+count(i))).Gap(14).Padding(10, 12).Radius(k.radiusSm).Justify(ui.Start).
				AlignItems(ui.Start).Label(o.title).Disabled(o.disabled).Cursor(ui.CursorPointer)
			on := i == *selected
			if o.disabled {
				b.Opacity(0.5)
			} else if b.Hovered() {
				b.Background(k.hover)
			}
			if b.Clicked() && !o.disabled && !on {
				*selected, changed = i, true
			}
			b.Children(func() {
				ui.Box(c).Size(18, 18).Shrink(0).Margin(2, 0, 0).Radius(9).Border(2, k.outline).Center().Children(func() {
					if on {
						ui.Box(c).Size(10, 10).Radius(5).Background(k.accent)
					}
				})
				ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
					ui.Text(c, o.title).FontSize(k.bodySize())
					ui.Text(c, o.description).FontSize(k.smallSize()).TextColor(k.muted)
				})
			})
		}
	})
	return changed
}

// segmented chooses one of a few options: segments in a 2px outline, the
// chosen one flooded with the accent; -1 chooses none. It tells whether
// the user changed the choice.
func (s *shell) segmented(c *ui.Context, key, label string, selected *int, labels ...string) bool {
	k := &s.lk
	before := *selected
	clicked := false
	seg := ui.SegmentedBase(c.Key(key), selected, len(labels))
	// The track clips its segments, so that the chosen one's flood keeps
	// to its rounded corners.
	seg.Track.Height(34).Radius(k.round(34)).Clip().Label(label).Shrink(0)
	if k.tonal {
		seg.Track.Background(k.surface2)
	} else {
		seg.Track.Border(2, k.outline)
	}
	seg.Track.Children(func() {
		for i, l := range labels {
			l := k.words(l)
			part := s.pointerKeys(seg.Segment(i)).PaddingX(14).FillHeight().Label(l).Cursor(ui.CursorPointer)
			ink := k.text
			switch {
			case i == *selected:
				part.Background(k.selected)
				ink = k.selectedText
			case part.Hovered():
				part.Background(k.hover)
			}
			if k.pill {
				part.Radius(17)
			}
			if part.Clicked() {
				clicked = true
			}
			// A label stays whole, without an ellipsis: a single line of
			// some faces lays out a hair wider than the box measured for
			// it, as Quicksand cut "best" to "be…".
			part.Children(func() { ui.Text(c, k.sentence(l)).FontSize(k.smallSize()).TextColor(ink).NoWrap() })
		}
	})
	// With none chosen, MyGo's segmented control takes the first of its
	// own: only a click chooses then.
	if before < 0 && !clicked {
		*selected = before
	}
	return *selected != before
}
