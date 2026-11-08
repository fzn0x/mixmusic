package main

// The wavy progress bar (#382): with Settings › Appearance › Wavy progress
// bar on, the played part of the seek bar, in the player bar and on the now
// playing page, is a gentle wave that moves along while the track plays
// and eases flat as it pauses. It is painted from the frame's time, so it
// moves in frames that only paint again, without building the view.

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

const (
	// waveLength is the length of one wave of the bar, and waveHeight how
	// far it rises above the middle of the track and falls below it, in
	// DIPs: within the knob's height, so that it goes in under the knob.
	waveLength = 28
	waveHeight = 2.5
	// wavePeriod is how long the wave takes to move its length along.
	wavePeriod = 2 * time.Second
	// waveRate is how many frames a second the wave moves at: it moves
	// half a DIP a frame, which more frames, each painting the window again,
	// would show no more smoothly.
	waveRate = 30
	// waveEase is how long the wave takes to lie flat as the track pauses,
	// and to rise again as it plays.
	waveEase = 400 * time.Millisecond
	// waveStep is how far apart the points of the wave's line are, in DIPs.
	waveStep = 2
)

// seekSlider is the seek bar's slider of *at, from 0 to most seconds: the
// theme's, or the wavy one while the settings ask for it, which waves
// while the track plays.
func (s *shell) seekSlider(c *ui.Context, at *float64, most float64) ui.Element {
	if !s.set.WavyProgress {
		return ui.Slider(c.Key("seek"), at, 0, most)
	}
	return wavySlider(c.Key("seek"), at, 0, most, s.st.Playing && !s.st.Loading)
}

// wavySlider is a slider of *v from lo to hi, which looks as the theme's
// sliders do, its track and its knob, but for the part of the track the
// accent fills: while moving is set, that is a wave, which moves along
// unless the computer asks for less motion, and lies flat once it isn't.
// It drags and takes the keys as the theme's does.
func wavySlider(c *ui.Context, v *float64, lo, hi float64, moving bool) ui.Element {
	t := c.Theme()
	kw := t.Space(5)
	// The knob moves across the content box, half of it inside the
	// padding on each side, as the theme's slider has it.
	sl := ui.SliderBase(c, v, lo, hi).Height(t.Space(5)).MinWidth(t.Space(20)).PaddingX(kw / 2).FocusRing(false)
	frac := float32(0)
	if hi > lo {
		frac = float32(min(max((*v-lo)/(hi-lo), 0), 1))
	}
	rtl := c.Direction() == ui.RightToLeft
	still := c.Preferences().ReduceMotion
	// Under Reduce Motion, both go to their ends at once.
	rise := sl.Animate("wave", b2f(moving), waveEase)
	held := sl.Animate("held", b2f(sl.Pressed()), 150*time.Millisecond)
	focus := sl.FocusVisible()
	points := ui.Local(sl, "wave", func() [][2]float32 { return nil })
	sl.Draw(func(p *ui.Painter, r ui.Rect) {
		h := t.Space(1.5)
		cy := r.Y + t.Space(2.5)
		track := ui.Rect{X: r.X, Y: cy - h/2, W: r.W, H: h}
		// Right to left, the bar fills from the right: it is worked out
		// left to right, and mirrored.
		mirror := func(x float32) float32 {
			if rtl {
				return r.X + r.W - (x - r.X)
			}
			return x
		}
		x := r.X + kw/2 + (r.W-kw)*frac
		k := knobBox(t, mirror(x), cy, held)
		if held > 0 {
			// The lens lightens what is behind it, under the track.
			p.Fill(k, ui.RGBA(255, 255, 255, 0.1*held), k.H/2)
		}
		played := x - track.X
		if rise <= 0 || played <= h {
			p.Fill(track, t.Border, h/2)
			p.Fill(ui.Rect{X: min(mirror(track.X), mirror(x)), Y: track.Y, W: played, H: h}, t.Accent, h/2)
		} else {
			// The track shows only where the wave isn't, from under the knob
			// on, not between the wave's crests.
			rest := ui.Rect{X: x - h/2, Y: track.Y, W: track.X + track.W - (x - h/2), H: h}
			if rtl {
				rest.X = mirror(track.X + track.W)
			}
			p.Fill(rest, t.Border, h/2)
			phase := float32(0)
			if !still {
				phase = wavePhase(p.Now())
				p.AnimationFrameRate(waveRate)
			}
			*points = wavePoints((*points)[:0], track.X+h/2, x-h/2, cy, rise*waveHeight, phase)
			if rtl {
				for i := range *points {
					(*points)[i][0] = mirror((*points)[i][0])
				}
			}
			p.StrokePolyline(*points, h, t.Accent)
		}
		paintKnob(p, t, k, held, focus)
	})
	return sl
}

// wavePhase is how far the wave has moved along at now, in waves, from 0
// to 1.
func wavePhase(now time.Time) float32 {
	return float32(now.UnixNano()%int64(wavePeriod)) / float32(wavePeriod)
}

// wavePoints appends to pts the points of a wave along y from x0 to x1,
// rising height above y and falling as far below it, moved along by phase
// of a wave: it rises from y over its first half wave, so that the end of
// the bar keeps to the middle of the track.
func wavePoints(pts [][2]float32, x0, x1, y, height, phase float32) [][2]float32 {
	if x1 <= x0 {
		return append(pts, [2]float32{x0, y})
	}
	n := int(math.Ceil(float64((x1 - x0) / waveStep)))
	for i := 0; i <= n; i++ {
		x := min(x0+float32(i)*waveStep, x1)
		d := x - x0
		a := height * ui.EaseInOut(min(d/(waveLength/2), 1))
		angle := 2 * math.Pi * float64(d/waveLength-phase)
		pts = append(pts, [2]float32{x, y - a*float32(math.Sin(angle))})
	}
	return pts
}

// knobBox is the box of a slider's knob centered at (x, y), held from 0 to
// 1, as the theme's: a capsule, which grows into a lens as it is held.
func knobBox(t *ui.Theme, x, y, held float32) ui.Rect {
	kw, kh := t.Space(5), t.Space(4)
	g := 1 + 0.35*held
	return ui.Rect{X: x - kw*g/2, Y: y - kh*g/2, W: kw * g, H: kh * g}
}

// paintKnob paints a slider's knob k, held from 0 to 1, as the theme
// paints its sliders' knobs: a capsule of white, lifted by its shadows,
// which grows into a lens of glass as it is held, and wears the focus ring
// while the keyboard's focus shows on it.
func paintKnob(p *ui.Painter, t *ui.Theme, k ui.Rect, held float32, focus bool) {
	face, rim, drop := ui.RGB(255, 255, 255), ui.RGBA(0, 0, 0, 0.22), ui.RGBA(0, 0, 0, 0.14)
	if t.Dark {
		face, rim, drop = ui.RGB(224, 225, 225), ui.RGBA(0, 0, 0, 0.7), ui.RGBA(0, 0, 0, 0.3)
	}
	rad := k.H / 2
	if held > 0 {
		p.Shadow(k, rad, 0, 8, 14, -3, drop.Alpha(held))
	}
	if held < 1 {
		p.Shadow(k, rad, 0, 0.5, 1, 0, ui.RGBA(0, 0, 0, 0.08*(1-held)))
		p.Shadow(k, rad, 0, 1.5, 7, 0, ui.RGBA(0, 0, 0, 0.1*(1-held)))
		p.Fill(k, face.Alpha(1-held), rad)
	}
	if held > 0 {
		// The lens's rim, lit on the inside.
		p.Stroke(k, rim.Alpha(held), rad, 1)
		p.Stroke(ui.Rect{X: k.X + 1, Y: k.Y + 1, W: k.W - 2, H: k.H - 2}, ui.RGBA(255, 255, 255, 0.25*held), rad-1, 1)
	}
	if focus {
		p.FocusRing(k, [4]float32{rad, rad, rad, rad})
	}
}
