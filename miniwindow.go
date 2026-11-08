package main

import (
	"math"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// The mini player's window as its panel opens and closes (#371): it grows
// by the panel's height on the side of the player with room on the screen,
// below unless the screen's work area has more room above, over a moment,
// and shrinks back as the panel closes, the player staying where it is.
// What the window does happens after the frame, as closing it does (#37),
// and in the order the frames ask it.

// miniPanelHeight is the height of the mini player's panel, at 100%.
const miniPanelHeight = 220

// miniGrow is how long the window takes to grow for the panel, or to
// shrink back.
const miniGrow = 200 * time.Millisecond

// miniWindow is what the mini player's panel does with its window:
// nativeMini, or a test's. Each method but later is called in a function
// later runs.
type miniWindow interface {
	// later runs fn on the main thread once the frame is drawn, in the
	// order asked.
	later(fn func())
	// content returns the window's content, in the screen's DIPs.
	content() mygo.Rectangle
	// workArea returns the work area of the display that shows the most
	// of r: the screen but the taskbar, the Dock and the menu bar.
	workArea(r mygo.Rectangle) mygo.Rectangle
	// limit sets the least width and height, and the largest height, 0
	// for none, of the window's content.
	limit(minW, minH, maxH int)
	// place moves and sizes the window's content to r.
	place(r mygo.Rectangle)
	// invalidate asks for a frame.
	invalidate()
}

// nativeMini is the mini player's own window.
type nativeMini struct{ win *mygo.Window }

func (n nativeMini) later(fn func())         { mygo.App.WhenReady(fn) }
func (n nativeMini) content() mygo.Rectangle { return n.win.ContentBounds() }
func (n nativeMini) workArea(r mygo.Rectangle) mygo.Rectangle {
	return mygo.Screen.DisplayMatching(r).WorkArea
}
func (n nativeMini) limit(minW, minH, maxH int) {
	n.win.SetMinimumSize(minW, minH)
	n.win.SetMaximumSize(0, maxH)
}
func (n nativeMini) place(r mygo.Rectangle) { n.win.SetContentBounds(r) }
func (n nativeMini) invalidate()            { n.win.Invalidate() }

// miniMove is the window's move as the panel opens or closes: from where
// it was, to where it goes, from start on.
type miniMove struct {
	from, to mygo.Rectangle
	start    time.Time
	opening  bool
	// placed is where the frame before asked the window to be.
	placed mygo.Rectangle
}

// panelBelow reports whether the panel opens below the window's content
// r, panel DIPs high, in the work area: where there's room for it there,
// or else where there's the most room.
func panelBelow(r, work mygo.Rectangle, panel int) bool {
	below := work.Y + work.Height - (r.Y + r.Height)
	above := r.Y - work.Y
	return below >= panel || below >= above
}

// grownFor is the window's content r grown by panel DIPs below it, or
// above it, kept in the work area where it can be, moving the player as
// little as it takes.
func grownFor(r, work mygo.Rectangle, panel int, below bool) mygo.Rectangle {
	g := r
	g.Height += panel
	if !below {
		g.Y -= panel
	}
	if work.Height > 0 {
		g.Y = max(min(g.Y, work.Y+work.Height-g.Height), work.Y)
	}
	return g
}

// shrunkFrom is the window's content r without the panel, the player
// part DIPs high: the player stays where it is.
func shrunkFrom(r mygo.Rectangle, part int, below bool) mygo.Rectangle {
	s := r
	s.Height = part
	if !below {
		s.Y = r.Y + r.Height - part
	}
	return s
}

// between is the rectangle t of the way from a to b.
func between(a, b mygo.Rectangle, t float64) mygo.Rectangle {
	at := func(x, y int) int { return x + int(math.Round(float64(y-x)*t)) }
	return mygo.Rectangle{X: at(a.X, b.X), Y: at(a.Y, b.Y), Width: at(a.Width, b.Width), Height: at(a.Height, b.Height)}
}

// miniLimit is the least and the largest height of the mini player's
// window, at 100%: the largest 0 for none.
type miniLimit struct{ min, max float32 }

// miniLimits are the mini player's window's heights, at 100%: those of the
// player alone, those of the player as tall as it opened beside the panel
// while it shows, and none while the window moves.
func (s *shell) miniLimits() miniLimit {
	m := &s.mini
	switch {
	case m.move != nil:
		return miniLimit{min: miniMinHeight}
	case m.shown != "":
		return miniLimit{min: m.part + miniPanelHeight, max: miniArt + miniPanelHeight}
	}
	return miniLimit{min: miniMinHeight, max: miniArt}
}

// limitMini sets the limits of miniLimits on the window, at the zoom.
func (s *shell) limitMini() {
	lim, z := s.miniLimits(), s.zoomOf()
	scale := func(n float32) int { return int(math.Ceil(float64(n * z))) }
	if lim.max == 0 {
		s.mini.win.limit(scale(miniMinWidth), scale(lim.min), 0)
		return
	}
	s.mini.win.limit(scale(miniMinWidth), scale(lim.min), scale(lim.max))
}

// growMini has the window grow for the panel, once this frame is drawn:
// on the side with room, from where it is. part is the player's height, at
// 100%, which it keeps beside the panel.
func (s *shell) growMini(part float32) {
	m := &s.mini
	reverse := m.move != nil && !m.move.opening
	if !reverse {
		m.part = part
	}
	if m.win == nil {
		return
	}
	m.win.later(func() {
		z := float64(s.zoomOf())
		from := m.win.content()
		if m.move != nil {
			// It shrank back part of the way: it grows from there.
			from = m.move.placed
		}
		panel := int(math.Round(miniPanelHeight * z))
		work := m.win.workArea(from)
		if !reverse {
			m.below = panelBelow(from, work, panel)
		}
		to := grownFor(shrunkFrom(from, int(math.Round(float64(m.part)*z)), m.below), work, panel, m.below)
		m.move = &miniMove{from: from, to: to, opening: true, placed: from}
		s.limitMini()
		m.win.invalidate()
	})
}

// shrinkMini has the window shrink back to the player, once this frame is
// drawn.
func (s *shell) shrinkMini() {
	m := &s.mini
	if m.win == nil {
		m.shown = ""
		return
	}
	m.win.later(func() {
		from := m.win.content()
		if m.move != nil {
			from = m.move.placed
		}
		to := shrunkFrom(from, int(math.Round(float64(m.part*s.zoomOf()))), m.below)
		m.move = &miniMove{from: from, to: to, placed: from}
		s.limitMini()
		m.win.invalidate()
	})
}

// moveMini moves the window on its way, a frame at a time, and ends the
// move once it's there: an opened panel's limits are set, and a closed
// panel goes. Under Reduce Motion, the window takes its size at once.
func (s *shell) moveMini(c *ui.Context) {
	m := &s.mini
	mv := m.move
	if mv == nil {
		return
	}
	if mv.start.IsZero() {
		mv.start = c.Now()
	}
	t := 1.0
	if !c.Preferences().ReduceMotion {
		t = min(float64(c.Now().Sub(mv.start))/float64(miniGrow), 1)
	}
	r := between(mv.from, mv.to, float64(ui.EaseOut(float32(t))))
	if r != mv.placed {
		mv.placed = r
		m.win.later(func() { m.win.place(r) })
	}
	if t < 1 {
		c.AnimationFrame()
		return
	}
	m.move = nil
	if !mv.opening && m.panel == "" {
		// Unless a panel opened as it shrank: it grows again next.
		m.shown = ""
	}
	m.win.later(s.limitMini)
	c.Invalidate()
}

// compactMini takes the panel out of the window at once, as it closes: the
// window keeps the player's size and place for the next time it opens.
// It runs on the main thread, outside a frame.
func (s *shell) compactMini() {
	m := &s.mini
	if m.shown == "" || m.win == nil {
		return
	}
	r := m.win.content()
	if m.move != nil {
		r = m.move.placed
	}
	m.shown, m.move = "", nil
	s.limitMini()
	m.win.place(shrunkFrom(r, int(math.Round(float64(m.part*s.zoomOf()))), m.below))
}

// miniHeights are the heights of the player and of the panel in a window
// h high: the player as tall as it was as the panel opened, while the
// panel shows, and the panel the rest.
func (s *shell) miniHeights(h float32) (part, panel float32) {
	m := &s.mini
	if m.shown == "" || m.part <= 0 {
		return h, 0
	}
	part = min(m.part, h)
	return part, h - part
}
