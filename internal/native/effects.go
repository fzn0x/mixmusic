package native

import (
	"image"
	"image/draw"
	"math"
	"strconv"
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/addons"
)

// effects are what the built-in themes' stylesheets add to their tokens in
// the web interface, which the native interface draws itself, as the
// themes' options say.
type effects struct {
	// texture is printed in a corner of the tiles (Mix): "dots",
	// "stripes", "slashes", or "" for none.
	texture string
	// aurora lights the ground with three glows, one of tint, which drift
	// unless still, and blur frosts the dialogs over them (Aurora).
	aurora, drift bool
	tint          ui.Color
	blur          float32
	// petals prints a faint pattern of petals on the ground, and italic
	// sets the pages' titles in italic (Sakura).
	petals, italic bool
	// scanlines draws a CRT's lines over the window, glow is how far the
	// pages' titles glow, prompt starts them with "> ", and phosphor
	// tints the artwork green (Terminal).
	scanlines, prompt, phosphor bool
	glow                        float32
	// wash washes the top of the ground down in the colour of the cover
	// (Greenroom), and cover lights the ground with it, under translucent
	// panels and dialogs frosted by blur (Glasshouse).
	wash, cover bool
	// speedlines draws an anime's speed lines toward a corner, which burst
	// when the track changes, and screentone prints manga's dots in the
	// corner across (Anime).
	speedlines, screentone bool
	// sparkles drift up the ground, twinkling (Mochi).
	sparkles bool
}

// optionValue is the value of a theme's option: the user's, or its
// default.
func optionValue(th addons.Theme, chosen map[string]string, id string) string {
	if v, ok := chosen[id]; ok {
		return v
	}
	for _, o := range th.Options {
		if o.ID == id {
			return o.Default
		}
	}
	return ""
}

// themeEffects are the effects of a built-in theme, with the values of
// its options the user chose.
func themeEffects(th addons.Theme, chosen map[string]string) effects {
	opt := func(id string) string { return optionValue(th, chosen, id) }
	num := func(id string) float32 {
		v, _ := strconv.ParseFloat(opt(id), 32)
		return float32(max(v, 0))
	}
	var fx effects
	switch th.ID {
	case "mix":
		switch t := opt("texture"); t {
		case "dots", "stripes", "slashes":
			fx.texture = t
		}
	case "aurora":
		fx.aurora, fx.drift, fx.blur = true, opt("motion") == "true", num("blur")
		tint, ok := parseColor(opt("tint"))
		if !ok {
			tint, _ = parseColor("#7b5cff")
		}
		fx.tint = col(tint)
	case "sakura":
		fx.petals, fx.italic = opt("petals") == "true", true
	case "terminal":
		fx.scanlines, fx.glow = opt("scanlines") == "true", num("glow")
		fx.prompt, fx.phosphor = true, true
	case "greenroom":
		fx.wash = opt("wash") == "true"
	case "glasshouse":
		fx.cover, fx.blur = opt("cover") == "true", num("blur")
	case "anime":
		fx.speedlines, fx.screentone = opt("speedlines") == "true", opt("screentone") == "true"
	case "mochi":
		fx.sparkles = opt("sparkles") == "true"
	}
	return fx
}

// ground paints the theme's ground on root, under the window's content:
// the lights of Aurora, which keep still while the computer asks for less
// motion, or the petals of Sakura. What moves is computed as it is
// painted: the frames an animation asks for paint again without building
// the view.
func (s *shell) ground(c *ui.Context, root ui.Element) {
	k := &s.lk
	switch {
	case k.fx.aurora:
		tint, accent, drift := k.fx.tint, k.accent, k.fx.drift && !c.Preferences().ReduceMotion
		root.Draw(func(p *ui.Painter, r ui.Rect) { paintAurora(p, r, tint, accent, drift) })
	case k.fx.petals:
		root.Draw(paintPetals)
	case k.fx.wash:
		t := s.coverTint(c, k.accent, washTone)
		root.Draw(func(p *ui.Painter, r ui.Rect) {
			paintWash(p, r, t.color(p.Now()))
			if t.changing(p.Now()) {
				p.AnimationFrame()
			}
		})
	case k.fx.cover:
		t := s.coverTint(c, k.accent, coverTone)
		// Without a cover, the accent's light is half as strong: the
		// glass stays white.
		accent, strength := k.accent, 1.0
		if s.artColor == "" {
			strength = 0.5
		}
		root.Draw(func(p *ui.Painter, r ui.Rect) {
			paintCover(p, r, t.color(p.Now()), accent, strength)
			if t.changing(p.Now()) {
				p.AnimationFrame()
			}
		})
	case k.fx.speedlines || k.fx.screentone:
		lines, tone, ink, at := k.fx.speedlines, k.fx.screentone, k.text, s.burstAt(c)
		root.Draw(func(p *ui.Painter, r ui.Rect) {
			if tone {
				s.paintScreentone(p, r, ink)
			}
			if lines {
				b := burst(at, p.Now())
				paintSpeedlines(p, r, ink, b)
				if b > 0 {
					p.AnimationFrame()
				}
			}
		})
	case k.fx.sparkles:
		// They rise while something plays, and hold still where they are
		// otherwise, as while the computer asks for less motion.
		s.rise.set(c.Now(), s.st.Playing && !c.Preferences().ReduceMotion)
		inks, rise := sparkleInks(k), s.rise
		root.Draw(func(p *ui.Painter, r ui.Rect) { paintSparkles(p, r, inks, rise) })
	}
}

// frosted fills a dialog's panel with the surface, or, in Aurora and
// Glasshouse, with frosted glass that what is under it shows through:
// Glasshouse's white, its raised surface over the blur.
func (s *shell) frosted(panel ui.Element) ui.Element {
	k := &s.lk
	switch {
	case k.fx.blur > 0 && k.fx.cover:
		return panel.Material(frost{glass.Blur{Radius: k.fx.blur}, k.surface2})
	case k.fx.blur > 0:
		return panel.Material(glass.Blur{Radius: k.fx.blur})
	}
	return panel.Background(k.surface)
}

// frost is frosted glass of a colour: what is under it blurred, under its
// tint.
type frost struct {
	blur glass.Blur
	tint ui.Color
}

func (f frost) PaintMaterial(p *ui.Painter, box ui.Rect, radii [4]float32) {
	f.blur.PaintMaterial(p, box, radii)
	p.Fill(box, f.tint, radii[0])
}

// overlay draws the theme's effects over the window's content, which the
// pointer goes through: Terminal's scanlines.
func (s *shell) overlay(c *ui.Context) {
	if !s.lk.fx.scanlines {
		return
	}
	ui.Box(c).Absolute().Top(0).Right(0).Bottom(0).Left(0).PassThrough().Draw(paintScanlines)
}

// auroraPeriod is how long Aurora's lights take to drift across the
// window, and as long to drift back.
const auroraPeriod = 40 * time.Second

// auroraStart is when Aurora's lights started drifting.
var auroraStart = time.Now()

// paintAurora paints the three lights of Aurora's ground, its stylesheet's
// radial gradients, on a ground 140% of the window's size, which drifts
// across it and back when drift is set.
func paintAurora(p *ui.Painter, r ui.Rect, tint, accent ui.Color, drift bool) {
	var x, y float32
	if drift {
		phase := float32(math.Mod(float64(p.Now().Sub(auroraStart))/float64(auroraPeriod), 2))
		if phase > 1 {
			phase = 2 - phase
		}
		phase = ui.EaseInOut(phase)
		x, y = phase, 0.6*phase
		// The lights move a pixel or so a frame: ten frames a second will do.
		p.After(100 * time.Millisecond)
	}
	w, h := 1.4*r.W, 1.4*r.H
	ox, oy := r.X-(w-r.W)*x, r.Y-(h-r.H)*y
	p.Clip(r, 0, func() {
		glow(p, ox+0.15*w, oy+0.10*h, 0.60*w, 0.50*h, withAlpha(tint, 0.45))
		glow(p, ox+0.85*w, oy+0.25*h, 0.50*w, 0.45*h, withAlpha(accent, 0.30))
		glow(p, ox+0.50*w, oy+h, 0.70*w, 0.60*h, ui.Color{R: 40, G: 90, B: 255, A: 64})
	})
}

// glowRings is how many rings a glow is painted with: enough that their
// steps don't show.
const glowRings = 48

// glow paints a radial gradient centered on (cx, cy), of radii rx and ry:
// c at the center, fading evenly to nothing 70% of the way out, as
// radial-gradient(c, transparent 70%) does. It paints rings, each over the
// ones around it, of the opacity that brings what they cover to the
// gradient's, and keeps what the opacities lost to rounding for the rings
// after.
func glow(p *ui.Painter, cx, cy, rx, ry float32, c ui.Color) {
	if rx <= 0 || ry <= 0 || c.A == 0 {
		return
	}
	a := float64(c.A) / 255
	p.Transform(ui.Scaling(1, ry/rx), cx, cy, func() {
		covered := 0.0
		for i := 1; i <= glowRings; i++ {
			want := a * float64(i) / glowRings
			ring := math.Round(255 * (1 - (1-want)/(1-covered)))
			if ring <= 0 {
				continue
			}
			covered = 1 - (1-covered)*(1-ring/255)
			rad := 0.7 * rx * float32(glowRings-i+1) / glowRings
			p.Fill(ui.Rect{X: cx - rad, Y: cy - rad, W: 2 * rad, H: 2 * rad}, ui.Color{R: c.R, G: c.G, B: c.B, A: uint8(ring)}, rad)
		}
	})
}

// withAlpha is c at an opacity of a.
func withAlpha(c ui.Color, a float64) ui.Color {
	c.A = uint8(math.Round(255 * clamp01(a)))
	return c
}

// paintPetals prints Sakura's faint petals on the ground: its stylesheet's
// three in every 260px square.
func paintPetals(p *ui.Painter, r ui.Rect) {
	const size = 260
	petals := []struct {
		x, y, r float32
		a       uint8
	}{{0.12, 0.18, 6.5, 18}, {0.78, 0.64, 9.5, 15}, {0.46, 0.88, 5.5, 13}}
	for y := r.Y; y < r.Y+r.H; y += size {
		for x := r.X; x < r.X+r.W; x += size {
			for _, pt := range petals {
				cx, cy := x+pt.x*size, y+pt.y*size
				p.Fill(ui.Rect{X: cx - pt.r, Y: cy - pt.r, W: 2 * pt.r, H: 2 * pt.r}, ui.Color{R: 224, G: 69, B: 123, A: pt.a}, pt.r)
			}
		}
	}
}

// paintScanlines draws Terminal's scanlines: a dark line every third DIP.
func paintScanlines(p *ui.Painter, r ui.Rect) {
	line := ui.Color{A: 46} // black at 18%
	for y := r.Y; y < r.Y+r.H; y += 3 {
		p.Fill(ui.Rect{X: r.X, Y: y, W: r.W, H: 1}, line, 0)
	}
}

// paintTexture prints Mix's texture in the corner of a tile of box r: a
// patch of 30% of it, 12px in from its bottom right corner, in ink at 55%.
func paintTexture(p *ui.Painter, r ui.Rect, texture string, ink ui.Color) {
	box := ui.Rect{W: 0.3 * r.W, H: 0.3 * r.H}
	box.X, box.Y = r.X+r.W-12-box.W, r.Y+r.H-12-box.H
	ink = withAlpha(ink, 0.55)
	p.Clip(box, 0, func() {
		switch texture {
		case "dots":
			// A dot of 1.25px in the middle of every 8px square.
			for y := box.Y + 4; y < box.Y+box.H+4; y += 8 {
				for x := box.X + 4; x < box.X+box.W+4; x += 8 {
					p.Fill(ui.Rect{X: x - 1.25, Y: y - 1.25, W: 2.5, H: 2.5}, ink, 1.25)
				}
			}
		case "stripes":
			for x := box.X; x < box.X+box.W; x += 7 {
				p.Fill(ui.Rect{X: x, Y: box.Y, W: 1, H: box.H}, ink, 0)
			}
		case "slashes":
			// Lines 8px apart across them, rising to the right.
			var path ui.Path
			step := float32(8 * math.Sqrt2)
			for o := -box.H; o < box.W; o += step {
				path.MoveTo(box.X+o, box.Y+box.H).LineTo(box.X+o+box.H, box.Y)
			}
			p.StrokePath(&path, 1, ink)
		}
	})
}

// phosphor is what Terminal's stylesheet does to artwork, by the luminance
// of a pixel: grayscale(1) sepia(1) hue-rotate(60deg) saturate(3)
// brightness(0.85), each step clamped, as CSS's filters are.
var phosphor = func() (lut [256][3]uint8) {
	mul := func(m [3][3]float64, v [3]float64) [3]float64 {
		var out [3]float64
		for i := range out {
			out[i] = clamp01(m[i][0]*v[0] + m[i][1]*v[1] + m[i][2]*v[2])
		}
		return out
	}
	sepia := [3][3]float64{{0.393, 0.769, 0.189}, {0.349, 0.686, 0.168}, {0.272, 0.534, 0.131}}
	cos, sin := math.Cos(math.Pi/3), math.Sin(math.Pi/3)
	hue := [3][3]float64{
		{0.213 + cos*0.787 - sin*0.213, 0.715 - cos*0.715 - sin*0.715, 0.072 - cos*0.072 + sin*0.928},
		{0.213 - cos*0.213 + sin*0.143, 0.715 + cos*0.285 + sin*0.140, 0.072 - cos*0.072 - sin*0.283},
		{0.213 - cos*0.213 - sin*0.787, 0.715 - cos*0.715 + sin*0.715, 0.072 + cos*0.928 + sin*0.072},
	}
	const sat = 3.0
	saturate := [3][3]float64{
		{0.213 + 0.787*sat, 0.715 - 0.715*sat, 0.072 - 0.072*sat},
		{0.213 - 0.213*sat, 0.715 + 0.285*sat, 0.072 - 0.072*sat},
		{0.213 - 0.213*sat, 0.715 - 0.715*sat, 0.072 + 0.928*sat},
	}
	for l := range lut {
		v := float64(l) / 255
		c := mul(saturate, mul(hue, mul(sepia, [3]float64{v, v, v})))
		for i := range c {
			lut[l][i] = unit(0.85 * c[i])
		}
	}
	return lut
}()

// phosphorize tints img as Terminal's stylesheet tints artwork.
func phosphorize(img image.Image) *image.RGBA {
	b := img.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(out, out.Bounds(), img, b.Min, draw.Src)
	px := out.Pix
	for i := 0; i+3 < len(px); i += 4 {
		a := float64(px[i+3])
		if a == 0 {
			continue
		}
		// The pixels are premultiplied by their alpha, as the tint must
		// be; grayscale(1) weighs sRGB's values as Rec. 709 does.
		l := int(math.Round((0.2126*float64(px[i]) + 0.7152*float64(px[i+1]) + 0.0722*float64(px[i+2])) * 255 / a))
		c := phosphor[min(l, 255)]
		for j := range 3 {
			px[i+j] = uint8(math.Round(float64(c[j]) * a / 255))
		}
	}
	return out
}
