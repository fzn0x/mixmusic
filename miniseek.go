package main

import (
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/locales"
)

// The mini player's progress (#371): a line of the accent along the foot
// of the player, 2 DIPs high at rest, which seeks. A click or a drag on it
// seeks where it lets go, as do its arrow keys once Tab reaches it; under
// the pointer, or with the keyboard's focus, it thickens, shows its thumb,
// and the time where the pointer is shows above it.

// miniSeekRoom is the height of the strip along the player's foot that
// seeks.
const miniSeekRoom = 10

// miniWaveHeight is how far the wave of the mini player's progress rises
// above its line and falls below it at rest, and miniWaveHeightActive
// under the pointer or with the keyboard's focus, in DIPs: the wave stays
// within the strip.
const (
	miniWaveHeight       = 1
	miniWaveHeightActive = 1.5
)

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
		Label(locales.PlayerSeek()).Value(locales.MiniSeekValue(clock(at), clockLength(dur))).Disabled(s.st.Loading).Cursor(ui.CursorPointer)
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
	// With Wavy progress bar on, the played part is a wave while the track
	// plays, and lies flat as it pauses, as the seek bar's does.
	wavy := s.set.WavyProgress
	rise := float32(0)
	if wavy {
		rise = seek.Animate("wave", b2f(s.st.Playing && !s.st.Loading), waveEase)
	}
	still := c.Preferences().ReduceMotion
	points := ui.Local(seek, "wave", func() [][2]float32 { return nil })
	seek.Draw(func(p *ui.Painter, r ui.Rect) {
		x, part := r.X, r.W*done
		if rtl {
			x = r.X + r.W - part
		}
		line, amp := float32(2), float32(miniWaveHeight)
		if active {
			line, amp = 4, miniWaveHeightActive
		}
		// The wave is worked out left to right, and mirrored.
		mirror := func(x float32) float32 {
			if rtl {
				return r.X + r.W - (x - r.X)
			}
			return x
		}
		waving := rise > 0 && part > line
		y := r.Y + r.H - line
		if waving {
			// The track shows only where the wave isn't.
			rx := r.X + part - line/2
			if rtl {
				rx = r.X
			}
			if active {
				p.Fill(ui.Rect{X: rx, Y: y, W: r.W - part + line/2, H: line}, thick, 2)
			}
			strokeWave(p, points, r.X+line/2, r.X+part-line/2, y+line/2-amp, line, rise*amp, still, mirror, rest)
		}
		if !active {
			if !waving {
				p.Fill(ui.Rect{X: x, Y: y, W: part, H: 2}, rest, 0)
			}
			s.paintMarks(p, ui.Rect{X: r.X, Y: y, W: r.W, H: 2}, dur, rtl)
			return
		}
		if !waving {
			p.Fill(ui.Rect{X: r.X, Y: y, W: r.W, H: 4}, thick, 2)
			p.Fill(ui.Rect{X: x, Y: y, W: part, H: 4}, rest, 2)
		}
		s.paintMarks(p, ui.Rect{X: r.X, Y: y, W: r.W, H: line}, dur, rtl)
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
	tw, th := float32(52), float32(20)
	label := clock(tip)
	// Over a part the player passes over, its kind too.
	if mark, ok := s.markAt(float64(tip)/float64(max(dur, 1)), dur, float64(6/max(w, 1))); ok {
		label, tw = mark, max(min(220, w-4), tw)
	}
	x := min(max(px-tw/2, 2), w-tw-2)
	ui.Box(c.Key("mini-seek-time")).Absolute().Left(x).Bottom(miniSeekRoom+2).Size(tw, th).Radius(k.radiusSm).
		Background(k.raised).Border(1, k.border).Center().PassThrough().AccessHidden(true).Children(func() {
		ui.Text(c, label).FontSize(k.captionSize()).FontFeatures("tnum").TextColor(k.text).SingleLine()
	})
}
