package main

// Pull to refresh: the page shown, pulled down past its top by a touchpad,
// a touchscreen or a mouse wheel turned up there, refreshes as Refresh
// this page does once let go past pullFar (refresh.go).
//
// MyGo tells a touchpad's scrolling, which goes by pixels (Precise), from a
// wheel's notches, and on macOS the momentum that goes on once the fingers
// left (Momentum). Windows and Linux send their momentum, where there is
// some, as any other scrolling. So a pull starts only on a page at rest at
// its top: scrolling that came up to the top, a fling's momentum among
// it, goes on to its gesture's end without pulling. A gesture ends once no
// scrolling came for pullRest, which lets go of its pull; or as the
// fingers leave, which macOS and Linux tell with an event that scrolls
// nothing, and macOS with the momentum that starts.

import (
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/locales"
)

const (
	// pullFar is how far a pull goes, in DIPs, for letting go to refresh
	// the page.
	pullFar = 120
	// pullNotch is how far a notch of a mouse wheel pulls, however far it
	// scrolls: three in a row go far enough, one never does.
	pullNotch = 40
	// pullRest is how long scrolling stops for its gesture to end.
	pullRest = 250 * time.Millisecond
	// pullBar is how tall the strip above the page opens at pullFar, and
	// pullStretch how much taller it opens past it, at most.
	pullBar, pullStretch = 56, 16
)

// pullState is a pull of the page shown, and the gesture it's of.
type pullState struct {
	// by is how far the page at is pulled, 0 while it isn't, and was how
	// far it was in the last frame it was, which the strip shows as it
	// closes: 0 for a pull that went back without refreshing.
	by, was float32
	at      string
	// last is when the gesture's last scrolling came, and momentum whether
	// it was momentum.
	last     time.Time
	momentum bool
	// held is set once the gesture can pull no more, until it ends: it
	// scrolled the page, sideways, or what is inside the page, it's
	// momentum, its pull was let go, or the page had nothing to refresh.
	held bool
	// inside is set by a scroller inside the page, as the lyrics, for the
	// scrolling over it, which the page takes next (scrollsInside).
	inside bool
	// shown is how tall the strip opened in the frame before.
	shown float32
}

// take takes ev, scrolling over the page, at rest at its top or not, which
// can't refresh while off, and reports whether it took ev, which the page
// doesn't scroll by, and whether letting go refreshes the page.
func (p *pullState) take(ev ui.InputEvent, top, off bool) (taken, refresh bool) {
	inside := p.inside
	p.inside = false
	if p.by > 0 && ev.Time.Sub(p.last) >= pullRest {
		// Its gesture ended before the frame after came.
		refresh = p.letGo()
	}
	if ev.DX == 0 && ev.DY == 0 {
		// The fingers left the touchpad.
		if p.by > 0 {
			refresh, taken = p.letGo(), true
		}
		return taken, refresh && !off
	}
	if ev.Time.Sub(p.last) >= pullRest || p.momentum && !ev.Momentum {
		// A gesture starts, as the fingers come back on macOS.
		p.held = false
	}
	p.last, p.momentum = ev.Time, ev.Momentum
	pulled := p.by > 0
	switch {
	case ev.Momentum && pulled:
		// The fingers left the touchpad, with a fling.
		refresh, taken = p.letGo(), true
	case ev.Momentum, inside, off, ev.Mods != 0, !pulled && abs32(ev.DX) > abs32(ev.DY):
		// No pull, as Shift+wheel scrolls sideways; one before goes back
		// without refreshing.
		p.by, p.was, p.held = 0, 0, true
	case pulled:
		// Scrolled down, a pull goes back first.
		p.by, taken = max(0, p.by+pullOf(ev)), true
	case ev.DY < 0 && top && !p.held:
		p.by, taken = pullOf(ev), true
	default:
		// It scrolls the page.
		p.held = true
	}
	return taken, refresh && !off
}

// rest lets go of a pull no scrolling came for since pullRest, by now, and
// reports whether it refreshes the page.
func (p *pullState) rest(now time.Time) bool {
	return p.by > 0 && now.Sub(p.last) >= pullRest && p.letGo()
}

// letGo lets go of the pull, which its gesture pulls no more, and reports
// whether it went far enough to refresh the page.
func (p *pullState) letGo() bool {
	far := p.by >= pullFar
	p.by, p.held = 0, true
	return far
}

// pullOf is how far ev pulls the page, back as it scrolls down: a
// touchpad's or a touchscreen's distance, a wheel's notch pullNotch.
func pullOf(ev ui.InputEvent) float32 {
	switch {
	case ev.Precise:
		return -ev.DY
	case ev.DY < 0:
		return pullNotch
	case ev.DY > 0:
		return -pullNotch
	}
	return 0
}

func abs32(v float32) float32 { return max(v, -v) }

// pullHeight is how tall the strip opens for a pull of by: in step with
// it to pullBar at pullFar, then less and less, to pullStretch more.
func pullHeight(by float32) float32 {
	if by <= pullFar {
		return by * pullBar / pullFar
	}
	over := by - pullFar
	return pullBar + pullStretch*over/(over+pullFar)
}

// pullInput is the input handler of the page shown: the scrolling over it
// may pull it. What a pull takes, the page doesn't scroll by, which it
// couldn't anyway, at its top.
func (s *shell) pullInput(ev ui.InputEvent) bool {
	if ev.Kind != ui.InputScroll {
		return false
	}
	p := &s.pull
	s.pullAt()
	sc := s.pageScrolled()
	taken, refresh := p.take(ev, sc.Y <= 0 && !sc.Scrolling(), s.nothingToRefresh() || s.refreshing())
	if refresh {
		s.refreshPage()
	}
	return taken
}

// scrollsInside is the input handler of a scroller inside a page, as the
// lyrics of the now playing page: the scrolling over it goes on to the
// page's handler, and never pulls the page.
func (s *shell) scrollsInside(ev ui.InputEvent) bool {
	if ev.Kind == ui.InputScroll {
		s.pull.inside = true
	}
	return false
}

// pullAt drops a pull of a page the window no longer shows.
func (s *shell) pullAt() {
	p := &s.pull
	if loc := s.router.Location(); p.at != loc {
		p.by, p.was, p.shown, p.at = 0, 0, 0, loc
	}
}

// pullStrip is the strip above the page shown that a pull opens, as far as
// pullHeight says, and lets go of a pull no scrolling came for since
// pullRest, which refreshes the page past pullFar. It shows a ring that
// fills as the pull goes and an arrow that turns with it, and past pullFar
// the ring filled with the accent. Let go, it closes on a spring, saying
// the page refreshes when it does. While the computer asks for less
// motion, it opens whole and closes at once, and its arrow stays still.
// It builds nothing while it's closed.
func (s *shell) pullStrip(c *ui.Context) {
	p := &s.pull
	s.pullAt()
	p.inside = false
	if p.rest(c.Now()) && !s.nothingToRefresh() && !s.refreshing() {
		s.refreshPage()
	}
	if p.by > 0 {
		p.was = p.by
		c.After(pullRest - c.Now().Sub(p.last))
	} else if p.shown == 0 {
		return
	}
	k := &s.lk
	still := c.Preferences().ReduceMotion
	strip := ui.Column(c.Key("pull-strip")).Shrink(0).Clip().AlignItems(ui.Center).Justify(ui.Center)
	var h float32
	switch {
	case p.by > 0 && still:
		h = strip.Jump("open", pullBar)
	case p.by > 0:
		h = strip.Jump("open", pullHeight(p.by))
	default:
		h = strip.AnimateTo("open", 0, k.spring(true))
	}
	if h < 0.5 {
		h = 0
	}
	p.shown = h
	strip.Height(h).Opacity(min(1, h/(pullBar*0.75)))
	far, gone := p.was >= pullFar, min(p.was/pullFar, 1)
	label, ink := locales.PullRefreshPull(), k.muted
	switch {
	case far && p.by > 0:
		label, ink = locales.PullRefreshRelease(), k.accentText
	case far:
		label, ink = locales.PullRefreshRefreshing(), k.accentText
	}
	strip.Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			badge := ui.Box(c).Size(28, 28).Shrink(0).Radius(14).Center()
			if far {
				badge.Background(k.accent)
			} else {
				track, fill := k.border, k.accent
				badge.Draw(func(pt *ui.Painter, r ui.Rect) { pullRing(pt, r, gone, track, fill) })
			}
			badge.Children(func() {
				arrow := ui.Icon(c, icon("refresh")).FontSize(16).TextColor(ink)
				if !still {
					arrow.Rotate(gone * 270)
				}
			})
			ui.Text(c, k.sentence(label)).FontSize(k.smallSize()).TextColor(k.muted)
		})
	})
}

// pullRing paints the ring of a pull in r: its track, and from the top
// round to gone of it, the part filled.
func pullRing(p *ui.Painter, r ui.Rect, gone float32, track, fill ui.Color) {
	const width = 2
	cx, cy, rad := r.X+r.W/2, r.Y+r.H/2, min(r.W, r.H)/2-width/2
	var ring ui.Path
	p.StrokePath(ring.Circle(cx, cy, rad), width, track)
	if gone <= 0 {
		return
	}
	// Round from the top, as a clock's hand.
	const k = 0.5522847498
	ring.Reset().MoveTo(cx, cy-rad).
		CubeTo(cx+rad*k, cy-rad, cx+rad, cy-rad*k, cx+rad, cy).
		CubeTo(cx+rad, cy+rad*k, cx+rad*k, cy+rad, cx, cy+rad).
		CubeTo(cx-rad*k, cy+rad, cx-rad, cy+rad*k, cx-rad, cy).
		CubeTo(cx-rad, cy-rad*k, cx-rad*k, cy-rad, cx, cy-rad).
		Trim(0, gone)
	p.StrokePath(&ring, width, fill)
}
