package native

// The effects of Greenroom, Glasshouse, Anime and Mochi: the cover's
// colour on the ground, speed lines and screentone, rising sparkles.

import (
	"image"
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

// coverTint returns the ground's tint: the colour of the cover of what
// plays, toned for the ground, or the fallback's without a cover, changing
// from the last over tintFade, at once while the computer asks for less
// motion.
func (s *shell) coverTint(c *ui.Context, fallback ui.Color, tone toning) tint {
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
	return *t
}

// changing tells whether the tint is still changing at now.
func (t tint) changing(now time.Time) bool {
	d := now.Sub(t.at)
	return d >= 0 && d < tintFade
}

// color is the tint's colour at now.
func (t tint) color(now time.Time) ui.Color {
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

// burstFor is how long Anime's speed lines burst once a track starts.
const burstFor = 700 * time.Millisecond

// impact is the track Anime's speed lines last saw start, and when.
type impact struct {
	track string
	at    time.Time
}

// burstAt returns when Anime's speed lines last burst, as a track started
// to play, or the zero time for none: while there's no track, and while
// the computer asks for less motion. A track the window opens on, or one
// that comes while paused, bursts them when it plays.
func (s *shell) burstAt(c *ui.Context) time.Time {
	id := ""
	if cur, ok := s.st.Current(); ok {
		id = cur.Track.ID
	}
	now := c.Now()
	switch {
	case s.st.Playing && id != s.impact.track:
		s.impact = impact{track: id, at: now}
	case now.Before(s.impact.at):
		// A clock set back would hold the burst.
		s.impact.at = now.Add(-burstFor)
	}
	if id == "" || c.Preferences().ReduceMotion {
		return time.Time{}
	}
	return s.impact.at
}

// burst is how far Anime's speed lines burst at now, after a burst at at:
// 1 at first, easing to 0 over burstFor.
func burst(at, now time.Time) float32 {
	f := float32(now.Sub(at)) / float32(burstFor)
	if at.IsZero() || f < 0 || f >= 1 {
		return 0
	}
	return 1 - ui.EaseOut(f)
}

// speedline is one of Anime's speed lines: where it crosses the window,
// from its bottom right corner (0) to its top left (1), how far from the
// focus it starts, as a share of the window's diagonal, and its half
// width at the window's far edge, in radians.
type speedline struct{ at, from, width float32 }

// seeded is a xorshift of seed, for patterns that hold still between
// frames: from 0 to 1.
func seeded(seed *uint32) float32 {
	*seed ^= *seed << 13
	*seed ^= *seed >> 17
	*seed ^= *seed << 5
	return float32(*seed%10000) / 10000
}

// speedlines are Anime's lines, the same at every frame: about 70 across
// the window, with gaps.
var speedlines = func() (lines []speedline) {
	seed := uint32(2166136261)
	const n = 96
	for i := range n {
		at, skip := (float32(i)+seeded(&seed))/n, seeded(&seed) < 0.3
		from, width := 0.34+0.36*seeded(&seed), 0.0012+0.0042*seeded(&seed)
		if !skip {
			lines = append(lines, speedline{at, from, width})
		}
	}
	return lines
}()

// paintSpeedlines draws Anime's speed lines in ink: thin wedges, widest
// at the window's far edges, each coming to a point on its way toward a
// focus past the top right corner, as the lines drawn around a manga's
// panel. A burst brings them nearer the focus, wider and darker.
func paintSpeedlines(p *ui.Painter, r ui.Rect, ink ui.Color, burst float32) {
	fx, fy := r.X+1.04*r.W, r.Y-0.12*r.H
	diag := float32(math.Hypot(float64(r.W), float64(r.H)))
	// From the bottom right corner to the top left, a little past both.
	a0 := math.Atan2(float64(r.Y+r.H-fy), float64(r.X+r.W-fx)) - 0.06
	a1 := math.Atan2(float64(r.Y-fy), float64(r.X-fx)) + 0.10
	outer := 1.6 * diag
	at := func(rad float32, a float64) (float32, float32) {
		return fx + rad*float32(math.Cos(a)), fy + rad*float32(math.Sin(a))
	}
	var path ui.Path
	for _, l := range speedlines {
		a := a0 + (a1-a0)*float64(l.at)
		half := float64(l.width * (1 + 0.8*burst))
		path.MoveTo(at(diag*(l.from-0.16*burst), a))
		path.LineTo(at(outer, a-half))
		path.LineTo(at(outer, a+half))
		path.Close()
	}
	p.Clip(r, 0, func() { p.FillPath(&path, withAlpha(ink, 0.07+0.10*float64(burst))) })
}

// tone is Anime's screentone, drawn once for a size and an ink.
type tone struct {
	w, h int
	ink  ui.Color
	bmp  *ui.Bitmap
}

// paintScreentone prints Anime's screentone in the bottom left corner of
// the ground, from a bitmap drawn once for its size.
func (s *shell) paintScreentone(p *ui.Painter, r ui.Rect, ink ui.Color) {
	side := min(0.62*r.H, 0.42*r.W, 460)
	box := ui.Rect{X: r.X, Y: r.Y + r.H - side, W: side, H: side}
	scale := p.Scale()
	w, h := int(math.Ceil(float64(box.W*scale))), int(math.Ceil(float64(box.H*scale)))
	if w <= 0 || h <= 0 {
		return
	}
	t := &s.tone
	if t.bmp == nil || t.w != w || t.h != h || t.ink != ink {
		*t = tone{w: w, h: h, ink: ink, bmp: ui.NewBitmap(screentone(w, h, scale, alpha(nrgba(ink), 0.12)))}
	}
	p.Image(t.bmp, box, ui.FillBox)
}

// screentone draws dots on a grid turned 45°, as screentone's are, of w by
// h pixels at scale pixels a DIP: largest in the bottom left corner and
// gone a width away from it, as a manga's gradated tones fade.
func screentone(w, h int, scale float32, ink color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	step := 9 * float64(scale)
	d := step / math.Sqrt2
	big, reach := 0.46*step, float64(w)
	n := int(math.Ceil(float64(w+h)/d)) + 1
	for i := -n; i <= n; i++ {
		for j := -n; j <= n; j++ {
			cx, cy := float64(i-j)*d, float64(i+j)*d
			if cx < -step || cy < -step || cx > float64(w)+step || cy > float64(h)+step {
				continue
			}
			rad := big * (1 - math.Hypot(cx, float64(h)-cy)/reach)
			if rad < 0.35 {
				continue
			}
			for py := max(int(cy-rad-1), 0); py <= min(int(cy+rad+1), h-1); py++ {
				for px := max(int(cx-rad-1), 0); px <= min(int(cx+rad+1), w-1); px++ {
					cover := clamp01(rad + 0.5 - math.Hypot(float64(px)+0.5-cx, float64(py)+0.5-cy))
					if a := uint8(math.Round(cover * float64(ink.A))); a > img.NRGBAAt(px, py).A {
						img.SetNRGBA(px, py, color.NRGBA{R: ink.R, G: ink.G, B: ink.B, A: a})
					}
				}
			}
		}
	}
	return img
}

// sparkle is one of Mochi's sparkles or hearts: where it rises across the
// window, as a share of its width, how far up it is at the start, how many
// times a minute it rises through the window, its size and sway in DIPs,
// and which of the inks it has.
type sparkle struct {
	x, start, rate, size, sway float32
	heart                      bool
	ink                        int
}

// rise is how long Mochi's sparkles have risen for: they rise only while
// something plays, and hold still where they are otherwise.
type rise struct {
	// before is how long they rose for until since, which is when they
	// last started rising, or zero while they hold still.
	before time.Duration
	since  time.Time
}

// set has the sparkles rise from now, or hold still there.
func (r *rise) set(now time.Time, moving bool) {
	switch {
	// A clock set back would hold them still.
	case moving && (r.since.IsZero() || now.Before(r.since)):
		r.since = now
	case !moving && !r.since.IsZero():
		r.before += max(now.Sub(r.since), 0)
		r.since = time.Time{}
	}
}

// at is how long the sparkles have risen for at now.
func (r rise) at(now time.Time) time.Duration {
	if r.since.IsZero() {
		return r.before
	}
	return r.before + max(now.Sub(r.since), 0)
}

// sparkles are Mochi's, the same at every frame.
var sparkles = func() (out []sparkle) {
	seed := uint32(0x9e3779b9)
	const n = 22
	for i := range n {
		out = append(out, sparkle{
			x:     (float32(i) + seeded(&seed)) / n,
			start: seeded(&seed),
			rate:  1.2 + 1.2*seeded(&seed),
			size:  3.5 + 5*seeded(&seed),
			sway:  6 + 14*seeded(&seed),
			heart: i%5 == 2,
			ink:   i % 4,
		})
	}
	return out
}()

// sparkleInks are Mochi's tiles, deepened a quarter of the way to its
// accent, to show on its ground.
func sparkleInks(k *look) (inks [4]ui.Color) {
	for i := range inks {
		inks[i] = col(mix(nrgba(k.tile(i)), nrgba(k.accent), 0.25))
	}
	return inks
}

// paintSparkles draws Mochi's sparkles and hearts rising up the ground,
// fading in at its foot and out at its head, swaying and twinkling on the
// way, as far as they rose.
func paintSparkles(p *ui.Painter, r ui.Rect, inks [4]ui.Color, rise rise) {
	t := rise.at(p.Now()).Seconds()
	if !rise.since.IsZero() {
		// They rise a pixel or so a frame, which ten frames a second would
		// show in steps: twenty will do.
		p.After(50 * time.Millisecond)
	}
	for _, sp := range sparkles {
		up := math.Mod(float64(sp.start)+t*float64(sp.rate)/60, 1)
		x := r.X + sp.x*r.W + sp.sway*float32(math.Sin(2*math.Pi*(t/9+float64(sp.start))))
		y := r.Y + r.H + 24 - float32(up)*(r.H+48)
		size := sp.size * (0.78 + 0.22*float32(math.Sin(2*math.Pi*(0.7*t+5*float64(sp.start)))))
		ink := withAlpha(inks[sp.ink], 0.9*math.Sin(math.Pi*up))
		var path ui.Path
		if sp.heart {
			s := 1.2 * size
			path.MoveTo(x, y+0.85*s).
				CubeTo(x-1.1*s, y+0.1*s, x-0.75*s, y-0.95*s, x, y-0.35*s).
				CubeTo(x+0.75*s, y-0.95*s, x+1.1*s, y+0.1*s, x, y+0.85*s).Close()
		} else {
			// Four points on concave sides, as a sparkle is drawn.
			c := 0.16 * size
			path.MoveTo(x, y-size).
				QuadTo(x+c, y-c, x+size, y).
				QuadTo(x+c, y+c, x, y+size).
				QuadTo(x-c, y+c, x-size, y).
				QuadTo(x-c, y-c, x, y-size).Close()
		}
		p.FillPath(&path, ink)
	}
}
