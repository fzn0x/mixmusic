package native

// The effects of Greenroom and Glasshouse: the cover's colour on the
// ground.

import (
	"image/color"
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

// nrgba is c as image/color has it.
func nrgba(c ui.Color) color.NRGBA { return color.NRGBA{R: c.R, G: c.G, B: c.B, A: c.A} }

// tintFade is how long the ground takes to change from one cover's
// colour to the next: the motion scale's longest step.
const tintFade = 520 * time.Millisecond

// tint is the colour of the cover the ground takes, changing from from to
// to since at.
type tint struct {
	from, to ui.Color
	at       time.Time
}

// toning brings a cover's colour to a ground: its lightness between lo
// and hi, its saturation up to sat.
type toning struct{ lo, hi, sat float64 }

var (
	// washTone keeps Greenroom's charcoal dark under the cover's colour,
	// and coverTone keeps Glasshouse's white light.
	washTone  = toning{0.28, 0.46, 0.45}
	coverTone = toning{0.68, 0.82, 0.55}
)

// coverTint returns the colour of the cover of what plays, toned for the
// ground, or the fallback's without a cover, and whether it is still
// changing from the last: over tintFade, at once while the computer asks
// for less motion.
func (s *shell) coverTint(c *ui.Context, fallback ui.Color, tone toning) (ui.Color, bool) {
	from := nrgba(fallback)
	if art, ok := parseColor(s.artColor); ok && s.artColor != "" {
		from = art
	}
	want := col(toned(from, tone.lo, tone.hi, tone.sat))
	now, t := c.Now(), &s.tint
	switch {
	// A clock set back would hold the change halfway.
	case t.at.IsZero(), c.Preferences().ReduceMotion, now.Before(t.at):
		t.from, t.to, t.at = want, want, now.Add(-tintFade)
	case want != t.to:
		t.from, t.to, t.at = t.color(now), want, now
	}
	return t.color(now), now.Sub(t.at) < tintFade
}

// color is the tint's colour at now.
func (t *tint) color(now time.Time) ui.Color {
	f := float32(now.Sub(t.at)) / float32(tintFade)
	if f >= 1 {
		return t.to
	}
	e := float64(ui.EaseOut(max(f, 0)))
	m := func(a, b uint8) uint8 { return uint8(math.Round(float64(a) + (float64(b)-float64(a))*e)) }
	return ui.Color{R: m(t.from.R, t.to.R), G: m(t.from.G, t.to.G), B: m(t.from.B, t.to.B), A: m(t.from.A, t.to.A)}
}

// paintWash washes the top of Greenroom's ground in c, strongest at the
// top and gone 60% of the way down.
func paintWash(p *ui.Painter, r ui.Rect, c ui.Color) {
	p.FillGradient(ui.Rect{X: r.X, Y: r.Y, W: r.W, H: min(0.6*r.H, 560)},
		ui.LinearGradient{From: withAlpha(c, 0.55), To: withAlpha(c, 0), Angle: 180, Oklab: true}, 0)
}

// paintCover lights Glasshouse's ground with the cover's colour c in two
// soft lights, and its accent in a third, at a strength from 0 to 1, which
// the translucent panels over them frost.
func paintCover(p *ui.Painter, r ui.Rect, c, accent ui.Color, strength float64) {
	p.Clip(r, 0, func() {
		glow(p, r.X+0.12*r.W, r.Y+0.04*r.H, 0.85*r.W, 0.80*r.H, withAlpha(c, 0.5*strength))
		glow(p, r.X+0.94*r.W, r.Y+0.32*r.H, 0.60*r.W, 0.70*r.H, withAlpha(accent, 0.14*strength))
		glow(p, r.X+0.55*r.W, r.Y+1.04*r.H, 0.95*r.W, 0.55*r.H, withAlpha(c, 0.32*strength))
	})
}
