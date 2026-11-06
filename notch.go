package main

import (
	"context"
	"errors"
	"math"
	"sort"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/lyrics"
)

// The panel's motion: it grows from the housing, and shrinks back, its
// contents fading in once it grows.
var (
	notchGrow   = ui.Animation{Duration: 240 * time.Millisecond, Ease: ui.EaseOut}
	notchShrink = ui.Animation{Duration: 200 * time.Millisecond, Ease: ui.EaseInOut}
	notchFadeIn = ui.Animation{Duration: 160 * time.Millisecond, Delay: 60 * time.Millisecond}
	notchFade   = ui.Animation{Duration: 100 * time.Millisecond}
)

// The colours of the player around the notch: black with white text,
// whatever the theme, as it extends the camera housing.
var (
	notchBlack = ui.RGB(0, 0, 0)
	notchWhite = ui.RGB(255, 255, 255)
	notchMuted = ui.RGBA(255, 255, 255, 0.6)
	// notchClear is the white of the contents faded out: its hue stays
	// as AnimateColor fades it in.
	notchClear = ui.RGBA(255, 255, 255, 0)
	notchTheme = func() *ui.Theme {
		t := *ui.DarkTheme()
		t.Background = ui.Transparent
		t.Text, t.TextMuted = notchWhite, notchMuted
		t.Accent, t.AccentHover, t.AccentPressed, t.AccentText = notchWhite, notchWhite, notchMuted, notchBlack
		// White while another app has the keyboard, as it mostly does.
		t.InactiveAccent = notchWhite
		t.Focus = notchWhite
		t.Font, t.FontSize = "", 13
		return &t
	}()
	// notchCollapse is the arrow that closes the panel, in the small
	// cover's place.
	notchCollapse = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="square" stroke-linejoin="miter"><path d="M5.5 14.5 12 8l6.5 6.5"/></svg>`))
)

// notchHost is what the system does for the window around the notch
// beside MyGo: AppKit's part on macOS (notch_darwin.go).
type notchHost interface {
	// keyboard gives the window the keyboard, as the panel opens, or gives
	// it back to the app the user was in, as it closes. It returns at once.
	keyboard(take bool)
	// close undoes what the host set up, as the window closes.
	close()
}

// notch is the player around the notch (#46): a window of its own, with a
// shell of its own, as the mini player has. Compact, it widens the camera
// housing with the cover and bars that move with the music; open, a panel
// grows down from it with the player and the lyrics.
type notch struct {
	s       *shell
	housing housing
	host    notchHost
	// place moves and sizes the window, after the frame.
	place func(mygo.Rectangle)
	// open is set while the panel is open, or opening; big while the
	// window has the open size, from the panel's opening until it shrank
	// back.
	open, big bool
	// away closes the open panel once the pointer stayed away.
	away countdown
	// keyFocus is set when a control of the panel showed the keyboard's
	// focus in the frame before, which keeps the panel open.
	keyFocus bool
	// seeking and seekTo are the progress bar while the user drags it.
	seeking bool
	seekTo  float64
	// artOf is the track whose artwork artURL is.
	artOf, artURL string
}

// notchArtPixels is the size the covers load at: the large cover's, at
// twice its points, as the screens with a notch have.
const notchArtPixels = 128

// OpenNotch opens the player around the notch of a MacBook's screen, or
// returns nil where no screen has a camera housing: other Macs, external
// displays, a closed lid, Windows and Linux.
func OpenNotch(a *app.App) *mygo.Window {
	h, ok := findHousing()
	if !ok || a.GoPlayer() == nil {
		return nil
	}
	registerFonts()
	n := &notch{s: newShell(a, a.GoPlayer()), housing: h}
	r := h.compact()
	win := mygo.NewWindow(mygo.WindowOptions{
		Title:  "MixMusic around the notch",
		X:      r.X,
		Y:      r.Y,
		Width:  r.Width,
		Height: r.Height,
		// Shown once AppKit has it over the menu bar.
		Hidden:        true,
		Panel:         true,
		Frameless:     true,
		Transparent:   true,
		DisableResize: true,
		DisableMove:   true,
		DisableShadow: true,
		Content:       ui.View(n.view),
	})
	n.s.attach(win)
	n.host = newNotchHost(win)
	// After the frame, as closing a window is (#37), and in the order
	// asked: a panel opened as it shrinks back gets the open size last.
	n.place = func(r mygo.Rectangle) { mygo.App.WhenReady(func() { win.SetBounds(r) }) }
	win.SetBounds(r)
	win.SetVisibleOnAllWorkspaces(true)
	n.s.follow(n.host.close)
	win.ShowInactive()
	return win
}

// toggle opens the panel, or closes it.
func (n *notch) toggle() {
	if n.open {
		n.close()
		return
	}
	n.open, n.big = true, true
	n.away.hold()
	n.place(n.housing.open())
	n.host.keyboard(true)
}

// close closes the panel, which shrinks back into the housing, and gives
// the keyboard back.
func (n *notch) close() {
	if !n.open {
		return
	}
	n.open = false
	n.away.hold()
	n.host.keyboard(false)
}

// openMixMusic shows MixMusic's main window, at the page of path unless
// it's "", and closes the panel.
func (n *notch) openMixMusic(path string) {
	n.s.svc.System.OpenMainWindow(path)
	n.close()
}

// view draws the player around the notch: the black around the housing,
// and the panel growing from it. The window keeps the open size while the
// panel moves, and only the black drawn in it grows or shrinks; once it
// shrank back, the window takes the compact size again.
func (n *notch) view(c *ui.Context) {
	s := n.s
	s.st = s.p.State()
	s.notes = s.notes[:0]
	// Whatever the theme, the covers aren't tinted.
	s.art.phosphor = false
	c.SetTheme(notchTheme)
	cur, ok := s.st.Current()
	reduce := c.Preferences().ReduceMotion
	root := ui.Box(c.Key("notch")).Fill()
	if !ok {
		// Nothing to play: the app closes the window.
		n.close()
		return
	}
	n.closing(c, root)
	open := n.open
	h := n.housing
	w, _ := c.Size()
	cx, hh := w/2, float32(h.H)
	cw := float32(h.compact().Width)
	o := h.open()
	ow, oh := float32(o.Width), float32(o.Height)

	grow := root.AnimateTo("grow", b2f(open), pick(open, notchGrow, notchShrink))
	ink := root.AnimateColor("ink", pick(open, notchWhite, notchClear), pick(open, notchFadeIn, notchFade))
	// Under Reduce Motion the panel fades in place, at its open size.
	fill := notchBlack
	if reduce {
		fill = root.AnimateColor("fill", pick(open, notchBlack, ui.Transparent), pick(open, notchFadeIn, notchFade))
	}
	alpha := float32(ink.A) / 255
	pw, ph := lerp(cw, ow, grow), lerp(hh, oh, grow)
	pr := lerp(h.compactRadius(), h.openRadius(), grow)
	if reduce {
		pw, ph, pr = ow, oh, h.openRadius()
	}
	// The black, under everything the view builds: under Reduce Motion,
	// the compact black and the panel fading in over it.
	cr := h.compactRadius()
	root.Draw(func(p *ui.Painter, r ui.Rect) {
		if reduce {
			p.Clip(ui.Rect{X: r.X + cx - cw/2, Y: r.Y - cr, W: cw, H: hh + cr}, cr, func() {
				p.Fill(ui.Rect{X: r.X + cx - cw/2, Y: r.Y, W: cw, H: hh}, notchBlack, 0)
			})
		}
		if fill.A > 0 {
			// Square at the top, against the screen's edge: the rounded
			// rectangle starts above the window.
			p.Clip(ui.Rect{X: r.X + cx - pw/2, Y: r.Y - pr, W: pw, H: ph + pr}, pr, func() {
				p.Fill(ui.Rect{X: r.X + cx - pw/2, Y: r.Y, W: pw, H: ph}, fill, 0)
			})
		}
	})
	n.keyFocus = false
	root.Children(func() {
		// The top row first, as Tab goes, then the panel's body below it,
		// cut to the black as it grows.
		n.topRow(c, cx, alpha)
		if n.big && (open || alpha > 0) {
			ui.Box(c.Key("notch-panel")).Absolute().Left(cx-pw/2).Top(hh).Size(pw, max(ph-hh, 0)).Radius(0, 0, pr, pr).Clip().
				Children(func() {
					body := ui.Row(c.Key("notch-body")).Absolute().Left((pw-ow)/2).Top(0).Size(ow, notchBody).
						Padding(6, 18, 16, 18).Gap(16).Opacity(alpha).Disabled(!open)
					body.Children(func() { n.body(c, cur.Track) })
				})
		}
	})
	switch {
	case n.open != open:
		// A control opened or closed the panel as the frame was built:
		// the next frame moves it.
		c.AnimationFrame()
	case n.big && !open && root.Settled("grow") && root.Settled("ink") && root.Settled("fill"):
		// The panel shrank back, and wasn't opened since.
		n.big = false
		n.place(h.compact())
	}
}

// closing closes the open panel on Escape, and once the pointer stayed
// away from it for two seconds while no drag or keyboard focus holds it.
func (n *notch) closing(c *ui.Context, root ui.Element) {
	if !n.open {
		return
	}
	if c.Shortcut(0, ui.KeyEscape) {
		n.close()
		return
	}
	if root.Hovered() || n.seeking || n.keyFocus {
		n.away.hold()
		return
	}
	if out, left := n.away.run(c.Now()); out {
		n.close()
	} else {
		c.After(left)
	}
}

// topRow draws what sits beside the housing: on the left the small cover,
// or the arrow that closes the panel in its place, with the button that
// opens MixMusic left of it; on the right the bars that move with the
// music, or a spinner while the track loads. Nothing is drawn under the
// camera. Clicking either side opens the panel, or closes it.
func (n *notch) topRow(c *ui.Context, cx, alpha float32) {
	s := n.s
	h := n.housing
	hw, hh := float32(h.W), float32(h.H)
	wing, art := float32(h.wing()), float32(h.art())
	top := (hh - art) / 2
	if alpha > 0 {
		open := n.s.buttonBase(c.Key("notch-main")).Absolute().Left(cx-hw/2-wing-notchOpenRoom+6).Top((hh-28)/2).
			Size(28, 28).Radius(14).Label("Open MixMusic").Cursor(ui.CursorPointer).Opacity(alpha).Disabled(!n.open)
		n.buttonLook(open, false, false)
		open.Children(func() { ui.Icon(c, icon("mini")).FontSize(15).TextColor(notchWhite) })
		if open.Clicked() {
			n.openMixMusic("")
		}
	}
	label := "Open the player"
	if n.open {
		label = "Close the player"
	}
	left := n.s.buttonBase(c.Key("notch-left")).Absolute().Left(cx-hw/2-wing).Top(0).Size(wing, hh).
		Label(label).Cursor(ui.CursorPointer)
	if left.FocusVisible() {
		n.keyFocus = true
	}
	left.Children(func() {
		if alpha < 1 {
			n.cover(c, "notch-small", art).Absolute().Left(notchInset).Top(top).Radius(art / 5).Opacity(1 - alpha)
		}
		if alpha > 0 {
			ui.Box(c).Absolute().Left(notchInset).Top(top).Size(art, art).Center().Opacity(alpha).Children(func() {
				ui.Icon(c, notchCollapse).FontSize(art * 0.8).TextColor(notchWhite)
			})
		}
	})
	if left.Clicked() {
		n.toggle()
	}
	// The right side opens and closes the panel too, for the pointer: the
	// left is the keyboard's.
	right := ui.Box(c.Key("notch-right")).Absolute().Left(cx+hw/2).Top(0).Size(wing, hh).Cursor(ui.CursorPointer)
	right.Children(func() {
		box := ui.Box(c).Absolute().Left(wing-notchInset-art).Top(top).Size(art, art).Center()
		if s.st.Loading || s.st.Waiting && s.st.WillPlay {
			box.Children(func() { ui.Spinner(c).FontSize(art * 0.7).TextColor(notchWhite).Label("Loading") })
			return
		}
		playing := s.st.Playing
		moving := playing && !c.Preferences().ReduceMotion
		box.Label(pick(playing, "Playing", "Paused")).Draw(func(p *ui.Painter, r ui.Rect) {
			drawBars(p, r, playing, moving)
		})
	})
	if right.Clicked() {
		n.toggle()
	}
}

// barsStill are the heights of the bars while the music plays under
// Reduce Motion, where they don't move.
var barsStill = [...]float32{0.45, 0.8, 0.6, 0.9, 0.5}

// drawBars draws five bars in r that move with the music while it plays,
// from p.Now() alone, and rest while it doesn't.
func drawBars(p *ui.Painter, r ui.Rect, playing, moving bool) {
	const bars = 5
	gap := r.W / 12
	bw := (r.W - (bars-1)*gap) / bars
	t := float64(p.Now().UnixNano()%int64(time.Hour)) / float64(time.Second)
	for i := range bars {
		f := float32(0.18)
		switch {
		case moving:
			// Each bar at a pace of its own, so that they never move as one.
			v := 0.5 + 0.5*math.Sin(2*math.Pi*(t*(1.1+0.37*float64(i)))+float64(i)*1.7)
			f = 0.25 + 0.75*float32(v)
		case playing:
			f = barsStill[i]
		}
		bh := max(r.H*f, bw)
		p.Fill(ui.Rect{X: r.X + float32(i)*(bw+gap), Y: r.Y + (r.H-bh)/2, W: bw, H: bh}, notchWhite, bw/2)
	}
	if moving {
		p.AnimationFrameRate(30)
	}
}

// body is the open panel below the housing: the cover, what plays, the
// progress and the transport on the left, and the lyrics on the right.
func (n *notch) body(c *ui.Context, t media.Track) {
	s := n.s
	ui.Column(c).Width(232).Shrink(0).Gap(8).Children(func() {
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			cover := n.s.buttonBase(c.Key("notch-cover")).Size(64, 64).Radius(10).Clip().Shrink(0).
				Label("Show what plays in MixMusic").Cursor(ui.CursorPointer)
			if cover.FocusVisible() {
				n.keyFocus = true
			}
			cover.Children(func() { n.cover(c, "notch-large", 64) })
			if cover.Clicked() {
				n.openMixMusic("/now-playing")
			}
			ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
				ui.Text(c, t.Title).FontSize(14).FontWeight(600).TextColor(notchWhite).SingleLine().MinWidth(0)
				ui.Text(c, names(t.Artists)).FontSize(12).TextColor(notchMuted).SingleLine().MinWidth(0)
			})
		})
		n.progress(c, t)
		ui.Row(c).Gap(14).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
			if n.transport(c, "notch-previous", "previous", "Previous", false, false).Clicked() {
				s.p.Previous()
			}
			name, label := toggleOf(s.st)
			toggle := n.transport(c, "notch-toggle", name, label, true, s.st.Loading)
			if toggle.Clicked() {
				s.p.Toggle()
			}
			if n.transport(c, "notch-next", "next", "Next", false, false).Clicked() {
				s.p.Next()
			}
		})
	})
	ui.Column(c).Grow(1).MinWidth(0).Justify(ui.Center).Gap(4).Children(func() { n.lyrics(c, t) })
}

// cover is the artwork of what plays, side points wide, or a grey square
// while it loads.
func (n *notch) cover(c *ui.Context, key string, side float32) ui.Element {
	s := n.s
	if cur, ok := s.st.Current(); ok && cur.Track.ID != n.artOf {
		// Asked for once a track, not each frame: artworkURL sorts.
		n.artOf, n.artURL = cur.Track.ID, artworkURL(cur.Track.Artwork, notchArtPixels)
	}
	b := s.art.get(c, n.artURL, notchArtPixels)
	box := ui.Box(c.Key(key)).Size(side, side).Shrink(0).Clip()
	if b == nil {
		return box.Background(ui.RGB(40, 40, 40))
	}
	return box.Children(func() { ui.Image(c, b).Size(side, side).Fit(ui.Cover) })
}

// transport is a button of the panel's transport: the main one a white
// circle, the others a glyph.
func (n *notch) transport(c *ui.Context, key, name, label string, main, disabled bool) ui.Element {
	size, ink := float32(32), notchWhite
	if main {
		size, ink = 38, notchBlack
	}
	b := n.s.buttonBase(c.Key(key)).Size(size, size).Radius(size / 2).Label(label).Cursor(ui.CursorPointer).Disabled(disabled)
	n.buttonLook(b, main, disabled)
	b.Children(func() { ui.Icon(c, icon(name)).FontSize(18).TextColor(ink) })
	return b
}

// buttonLook styles a button of the panel: a white circle when main, a
// tint while hovered or pressed otherwise, dimmed when disabled. A focus
// that came from the keyboard keeps the panel open.
func (n *notch) buttonLook(b ui.Element, main, disabled bool) {
	switch {
	case main:
		b.Background(notchWhite)
	case b.Pressed():
		b.Background(ui.RGBA(255, 255, 255, 0.24))
	case b.Hovered():
		b.Background(ui.RGBA(255, 255, 255, 0.12))
	}
	if disabled {
		b.Opacity(0.45)
	}
	if b.FocusVisible() {
		n.keyFocus = true
	}
}

// progress is the thin bar of how far the track played, between the time
// played and its length: dragging it, or its arrow keys, seek, once the
// drag ends. Dragging it keeps the panel open.
func (n *notch) progress(c *ui.Context, t media.Track) {
	s := n.s
	dur := s.st.Duration
	if dur <= 0 {
		dur = ms(t.DurationMs)
	}
	if !n.seeking {
		n.seekTo = s.st.Position.Seconds()
	}
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		caption := func(text string) {
			ui.Text(c, text).FontSize(11).TextColor(notchMuted).FontFeatures("tnum").Width(34).TextAlign(ui.Center).Shrink(0)
		}
		if s.st.Live {
			caption("live")
		} else {
			caption(clock(time.Duration(n.seekTo * float64(time.Second))))
		}
		seek := ui.SliderBase(c.Key("notch-seek"), &n.seekTo, 0, max(dur.Seconds(), 1)).Grow(1).MinWidth(0).
			Height(14).PaddingX(4).Label("Seek").Disabled(s.st.Live || s.st.Loading)
		_, _, held := seek.Dragged()
		if !held && (n.seeking || seek.Changed()) {
			s.p.Seek(time.Duration(n.seekTo * float64(time.Second)))
		}
		n.seeking = held
		keys := seek.FocusVisible()
		if keys {
			n.keyFocus = true
		}
		done := float32(0)
		if d := max(dur.Seconds(), 1); !s.st.Live {
			done = float32(min(max(n.seekTo/d, 0), 1))
		}
		thumb := held || keys || seek.Hovered()
		seek.Draw(func(p *ui.Painter, r ui.Rect) {
			x, y, w := r.X+4, r.Y+r.H/2, r.W-8
			p.Fill(ui.Rect{X: x, Y: y - 1.5, W: w, H: 3}, ui.RGBA(255, 255, 255, 0.25), 1.5)
			p.Fill(ui.Rect{X: x, Y: y - 1.5, W: w * done, H: 3}, notchWhite, 1.5)
			if thumb {
				p.Fill(ui.Rect{X: x + w*done - 4.5, Y: y - 4.5, W: 9, H: 9}, notchWhite, 4.5)
			}
		})
		if s.st.Live {
			caption("")
		} else {
			caption(clock(dur))
		}
	})
}

// lyrics shows the line of the track's timed lyrics sung now, with the
// lines before and after it, following the music; plain lyrics or none
// say why, and a lookup that failed can be tried again. They come from
// Catalog.Lyrics, which keeps them: opening the panel doesn't ask again.
func (n *notch) lyrics(c *ui.Context, t media.Track) {
	if t.IsEpisode() {
		// An episode has its chapters in their place (episodedetails.go).
		n.chapters(c, t)
		return
	}
	s := n.s
	note := func(text string) {
		ui.Text(c, text).FontSize(12).TextColor(notchMuted).MaxLines(3)
	}
	l := fetch(s, "lyrics:"+t.ID, func(ctx context.Context) (lyrics.Lyrics, error) { return s.svc.Catalog.Lyrics(ctx, t) })
	switch {
	case !l.done:
		note("Looking for the lyrics…")
		return
	case l.err != nil:
		note(l.err.Error())
		// A track without lyrics stays so; a failure may pass.
		if !errors.Is(l.err, lyrics.ErrNotFound) {
			again := n.s.buttonBase(c.Key("notch-lyrics-retry")).Padding(4, 10).Radius(12).Border(1, notchMuted).
				AlignSelf(ui.Start).Label("try again").Cursor(ui.CursorPointer)
			n.buttonLook(again, false, false)
			again.Children(func() { ui.Text(c, "try again").FontSize(12).TextColor(notchWhite) })
			if again.Clicked() {
				s.forget("lyrics:" + t.ID)
			}
		}
		return
	case l.value.Instrumental:
		note("This track is instrumental.")
		return
	case !l.value.Synced || len(l.value.Lines) == 0:
		note("No timed lyrics.")
		return
	}
	lines := l.value.Lines
	pos := s.st.Position + lyricsLead(s.st.Speed)
	at := sort.Search(len(lines), func(i int) bool { return ms(lines[i].TimeMs) > pos }) - 1
	// In Latin letters, as the user shows the lyrics of their language
	// (lyricsromanize.go).
	latin, how := s.latinLines(t, l.value)
	line := func(i int) string { return notchLine(lines, latin, how, i) }
	if at > 0 {
		ui.Text(c, line(at-1)).FontSize(12).TextColor(notchMuted).SingleLine().MinWidth(0)
	}
	ui.Text(c, line(at)).FontSize(15).FontWeight(600).TextColor(notchWhite).MaxLines(2).MinWidth(0)
	if under := notchUnder(latin, how, at); under != "" {
		ui.Text(c, under).FontSize(12).TextColor(notchMuted).SingleLine().MinWidth(0)
	}
	if at+1 < len(lines) {
		ui.Text(c, line(at+1)).FontSize(12).TextColor(notchMuted).SingleLine().MinWidth(0)
	}
}

// lerp is the value t of the way from a to b.
func lerp(a, b, t float32) float32 { return a + (b-a)*t }

// b2f is 1 for true, 0 for false.
func b2f(v bool) float32 {
	if v {
		return 1
	}
	return 0
}

// pick is a when v, else b.
func pick[T any](v bool, a, b T) T {
	if v {
		return a
	}
	return b
}
