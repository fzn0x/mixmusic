package main

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// The mini player's progress (#371): a line of the accent along the foot
// of the player, 2 DIPs high at rest, which seeks. A click or a drag on it
// seeks where it lets go, as do its arrow keys once Tab reaches it; under
// the pointer, or with the keyboard's focus, it thickens, shows its thumb,
// and the time where the pointer is shows above it.

// miniSeekRoom is the height of the strip along the player's foot that
// seeks.
const miniSeekRoom = 10

// miniSeek is the progress along the foot of the player, w wide, which
// seeks.
func (s *shell) miniSeek(c *ui.Context, w float32) {
	k := &s.lk
	dur := s.miniDuration()
	if !s.seeking {
		s.seekTo = s.st.Position.Seconds()
	}
	hi := max(dur.Seconds(), 1)
	at := time.Duration(s.seekTo * float64(time.Second))
	seek := ui.SliderBase(c.Key("mini-seek"), &s.seekTo, 0, hi).Absolute().Left(0).Bottom(0).Size(w, miniSeekRoom).
		Label("Seek").Value(clock(at) + " of " + clock(dur)).Disabled(s.st.Loading).Cursor(ui.CursorPointer)
	_, _, held := seek.Dragged()
	if !held && (s.seeking || seek.Changed()) {
		s.p.Seek(time.Duration(s.seekTo * float64(time.Second)))
	}
	s.seeking = held
	px, _, over := seek.PointerPosition()
	over = over && !s.st.Loading
	active := held || over || seek.FocusVisible()
	done := float32(min(max(s.seekTo/hi, 0), 1))
	rtl := c.Direction() == ui.RightToLeft
	rest, thick := k.accent, k.border
	seek.Draw(func(p *ui.Painter, r ui.Rect) {
		x, part := r.X, r.W*done
		if rtl {
			x = r.X + r.W - part
		}
		if !active {
			p.Fill(ui.Rect{X: x, Y: r.Y + r.H - 2, W: part, H: 2}, rest, 0)
			return
		}
		y := r.Y + r.H - 4
		p.Fill(ui.Rect{X: r.X, Y: y, W: r.W, H: 4}, thick, 2)
		p.Fill(ui.Rect{X: x, Y: y, W: part, H: 4}, rest, 2)
		knob := r.X + part
		if rtl {
			knob = r.X + r.W - part
		}
		knob = min(max(knob, r.X+5), r.X+r.W-5)
		p.Fill(ui.Rect{X: knob - 5, Y: y - 3, W: 10, H: 10}, rest, 5)
	})
	if !over && !held {
		return
	}
	// The time where the pointer is, or where the drag would seek.
	tip := at
	if !held {
		f := min(max(px/max(w, 1), 0), 1)
		if rtl {
			f = 1 - f
		}
		tip = time.Duration(float64(f) * float64(dur))
	}
	const tw, th = 52, 20
	x := min(max(px-tw/2, 2), w-tw-2)
	ui.Box(c.Key("mini-seek-time")).Absolute().Left(x).Bottom(miniSeekRoom+2).Size(tw, th).Radius(k.radiusSm).
		Background(k.raised).Border(1, k.border).Center().PassThrough().AccessHidden(true).Children(func() {
		ui.Text(c, clock(tip)).FontSize(k.captionSize()).FontFeatures("tnum").TextColor(k.text).SingleLine()
	})
}
