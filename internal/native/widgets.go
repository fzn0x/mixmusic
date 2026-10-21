package native

import (
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
)

// iconButton is a 32px button of an icon, in muted ink, which turns to ink
// over a tint on hover, and to the accent when on.
func (s *shell) iconButton(c *ui.Context, key, name, label string, on bool, size float32) ui.Element {
	k := &s.lk
	b := ui.ButtonBase(c.Key(key)).Size(32, 32).Radius(k.radiusSm).Label(label).Tooltip(label).KeepFocus().Cursor(ui.CursorPointer)
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
	b.Children(func() { ui.Icon(c, icon(name)).FontSize(size).TextColor(ink) })
	return b
}

// roundButton is a transport button: a 38px circle with a 2px outline, or,
// as the main button, a 48px solid ink circle that turns to the accent on
// hover.
func (s *shell) roundButton(c *ui.Context, key, name, label string, main, disabled bool) ui.Element {
	if main {
		return s.round(c, key, name, label, true, disabled, 48, 22)
	}
	return s.round(c, key, name, label, false, disabled, 38, 18)
}

// round is a round button of size, with a glyph of the size given.
func (s *shell) round(c *ui.Context, key, name, label string, main, disabled bool, size, glyph float32) ui.Element {
	k := &s.lk
	b := ui.ButtonBase(c.Key(key)).Size(size, size).Radius(size / 2).Label(label).Tooltip(label).KeepFocus().
		Cursor(ui.CursorPointer).Disabled(disabled)
	ink := k.text
	switch {
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
	if !disabled && b.Pressed() {
		b.Scale(0.97, 0.97)
	}
	b.Children(func() { ui.Icon(c, icon(name)).FontSize(glyph).TextColor(ink) })
	return b
}

// button is a 40px button with a label: a filled accent block when
// primary, else a 2px outline that inverts to solid ink when pressed.
func (s *shell) button(c *ui.Context, key, label string, primary bool) ui.Element {
	k := &s.lk
	b := ui.ButtonBase(c.Key(key)).Height(40).PaddingX(24).Radius(k.radiusSm).Label(label).Cursor(ui.CursorPointer).AlignSelf(ui.Start)
	ink := k.text
	switch {
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
	if b.Pressed() {
		b.Scale(0.97, 0.97)
	}
	b.Children(func() { ui.Text(c, label).FontSize(k.bodySize()).TextColor(ink).SingleLine() })
	return b
}

// pageHeader names the page in one giant light lowercase word, which
// shrinks to fit the pane on one line, and wraps under 45% of its size,
// never cut. Pivots under it switch the page's sections; selected is the
// current one's index.
func (s *shell) pageHeader(c *ui.Context, title string, pivots []string, selected *int) {
	s.pageHeaderCounts(c, title, pivots, nil, selected)
}

// pageHeaderCounts is a page's header whose pivots count what they show:
// counts[i], or nothing where it's below 0.
func (s *shell) pageHeaderCounts(c *ui.Context, title string, pivots []string, counts []int, selected *int) {
	k := &s.lk
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
					b := ui.ButtonBase(c.Key("pivot-" + p)).Label(p).Cursor(ui.CursorPointer)
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

// sectionTitle titles a section over its content, on a 1px accent rule.
func (s *shell) sectionTitle(c *ui.Context, title string) { s.sectionHead(c, title, nil) }

// sectionHead is a section's title, with links to what relates to it at
// its end.
func (s *shell) sectionHead(c *ui.Context, title string, links func()) {
	k := &s.lk
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
	return ui.Link(c.Key(key), label, "").FontSize(s.lk.smallSize()).Clicked()
}

// alert tells of a problem in a coral panel, with what to do about it.
func (s *shell) alert(c *ui.Context, title, message string, actions func()) ui.Element {
	k := &s.lk
	return ui.Column(c).Padding(16).Gap(12).Background(k.danger.Alpha(0.12).Over(k.bg)).Border(1, k.danger.Alpha(0.45)).
		Radius(k.radiusSm).Children(func() {
		ui.Text(c, title).FontSize(k.subtitleSize()).FontWeight(300)
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
		b = s.art.get(artworkURL(images, int(size)), artPixels(size))
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

// artPixels is how many pixels artwork of size DIPs takes, at the scale of
// most displays.
func artPixels(size float32) int { return int(size * 1.5) }

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
			b := ui.ButtonBase(c.Key(key+"-"+count(i))).Gap(14).Padding(10, 12).Radius(k.radiusSm).Justify(ui.Start).
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
	seg.Track.Height(34).Border(2, k.outline).Radius(k.radiusSm).Label(label).Shrink(0).Children(func() {
		for i, l := range labels {
			part := seg.Segment(i).PaddingX(14).FillHeight().Label(l).Cursor(ui.CursorPointer)
			ink := k.text
			switch {
			case i == *selected:
				part.Background(k.accent)
				ink = k.accentText
			case part.Hovered():
				part.Background(k.hover)
			}
			if part.Clicked() {
				clicked = true
			}
			part.Children(func() { ui.Text(c, l).FontSize(k.smallSize()).TextColor(ink).SingleLine() })
		}
	})
	// With none chosen, MyGo's segmented control takes the first of its
	// own: only a click chooses then.
	if before < 0 && !clicked {
		*selected = before
	}
	return *selected != before
}
