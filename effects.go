package main

import (
	"image"
	"image/color"
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
	// (Greenroom), cover lights the ground with it under translucent
	// panels, and white makes the dialogs white glass, frosted by blur,
	// whatever shows under them (Glasshouse).
	wash, cover, white bool
	// speedlines draws an anime's speed lines toward a corner, which burst
	// when the track changes, and screentone prints manga's dots in the
	// corner across (Anime).
	speedlines, screentone bool
	// sparkles drift up the ground, twinkling (Mochi).
	sparkles bool
	// material makes the colours of Material's scheme of a seed: the
	// artwork's, as the accent follows it, or the accent (Material).
	material bool
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
		fx.cover, fx.blur, fx.white = opt("cover") == "true", num("blur"), true
	case "anime":
		fx.speedlines, fx.screentone = opt("speedlines") == "true", opt("screentone") == "true"
	case "mochi":
		fx.sparkles = opt("sparkles") == "true"
	case "material", "material-dark":
		fx.material = true
	}
	return fx
}

// ground paints the theme's ground on root, under the window's content:
// the lights of Aurora, which keep still while the computer asks for less
// motion, or the petals of Sakura. What moves is computed as it is
// painted: the frames an animation asks for paint again without building
// the view.
func (s *shell) ground(c *ui.Context, root ui.Element) {
	if paint := s.groundPaint(c); paint != nil {
		root.Draw(paint)
	}
}

// groundOn fills e with the ground as the window shows it where e is, over
// what scrolls under e: a row pinned over a page shows as the ground
// around it does.
func (s *shell) groundOn(c *ui.Context, e ui.Element) {
	e.Background(s.lk.bg)
	paint := s.groundPaint(c)
	if paint == nil {
		return
	}
	w, h := c.Size()
	window := ui.Rect{W: w, H: h}
	e.Draw(func(p *ui.Painter, r ui.Rect) { p.Clip(r, 0, func() { paint(p, window) }) })
}

// groundPaint paints the theme's ground on the window's box, or is nil
// for a theme of none.
func (s *shell) groundPaint(c *ui.Context) func(p *ui.Painter, r ui.Rect) {
	k := &s.lk
	switch {
	case k.fx.aurora:
		bg, tint, accent, drift := k.bg, k.fx.tint, k.accent, k.fx.drift && !c.Preferences().ReduceMotion
		return func(p *ui.Painter, r ui.Rect) { s.paintAurora(p, r, bg, tint, accent, drift) }
	case k.fx.petals:
		return paintPetals
	case k.fx.wash:
		t := s.coverTint(c, k.accent, groundTone(k.dark))
		return func(p *ui.Painter, r ui.Rect) {
			paintWash(p, r, t.color(p.Now()))
			if t.changing(p.Now()) {
				p.AnimationFrame()
			}
		}
	case k.fx.cover:
		t := s.coverTint(c, k.accent, groundTone(k.dark))
		// Without a cover, the accent's light is half as strong: the
		// glass stays white.
		bg, accent, strength := k.bg, k.accent, 1.0
		if s.artColor == "" {
			strength = 0.5
		}
		return func(p *ui.Painter, r ui.Rect) {
			s.paintCover(p, r, bg, t.color(p.Now()), accent, strength)
			if t.changing(p.Now()) {
				p.AnimationFrame()
			}
		}
	case k.fx.speedlines || k.fx.screentone:
		lines, tone, ink, at := k.fx.speedlines, k.fx.screentone, k.text, s.burstAt(c)
		return func(p *ui.Painter, r ui.Rect) {
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
		}
	case k.fx.sparkles:
		// They rise while something plays, and hold still where they are
		// otherwise, as while the computer asks for less motion.
		s.rise.set(c.Now(), s.st.Playing && !c.Preferences().ReduceMotion)
		inks, rise := sparkleInks(k), s.rise
		return func(p *ui.Painter, r ui.Rect) { paintSparkles(p, r, inks, rise) }
	}
	return nil
}

// frosted fills a dialog's panel with the surface, laid over the ground
// where the theme's would show the page through, or, in Aurora and
// Glasshouse, with frosted glass that what is under it shows through:
// Glasshouse's white, its raised surface over the blur.
func (s *shell) frosted(panel ui.Element) ui.Element {
	k := &s.lk
	switch {
	case k.fx.blur > 0 && k.fx.white:
		return panel.Material(frost{glass.Blur{Radius: k.fx.blur}, k.surface2})
	case k.fx.blur > 0:
		return panel.Material(glass.Blur{Radius: k.fx.blur})
	}
	return panel.Background(col(over(nrgba(k.surface), nrgba(k.bg))))
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

// auroraFrame is how often Aurora's lights drift a step. They are soft:
// at their fastest, halfway across, a fifth of a second moves them far
// enough to change a pixel by about a level of colour, which more frames,
// each painting the whole window again, would show no more smoothly.
const auroraFrame = 200 * time.Millisecond

// paintAurora paints the three lights of Aurora's ground over bg, its
// stylesheet's radial gradients, on a ground 140% of the window's size,
// which drifts across it and back when drift is set.
func (s *shell) paintAurora(p *ui.Painter, r ui.Rect, bg, tint, accent ui.Color, drift bool) {
	var x, y float32
	if drift {
		phase := float32(math.Mod(float64(p.Now().Sub(auroraStart))/float64(auroraPeriod), 2))
		if phase > 1 {
			phase = 2 - phase
		}
		phase = ui.EaseInOut(phase)
		x, y = phase, 0.6*phase
		p.After(auroraFrame)
	}
	w, h := 1.4*r.W, 1.4*r.H
	ground := s.lights.of(bg, [3]groundLight{
		{0.15, 0.10, 0.60, 0.50, withAlpha(tint, 0.45)},
		{0.85, 0.25, 0.50, 0.45, withAlpha(accent, 0.30)},
		{0.50, 1, 0.70, 0.60, ui.Color{R: 40, G: 90, B: 255, A: 64}},
	})
	p.Clip(r, 0, func() { p.Image(ground, ui.Rect{X: r.X - (w-r.W)*x, Y: r.Y - (h-r.H)*y, W: w, H: h}, ui.FillBox) })
}

// groundLight is one of the lights of a ground: a glow of c, at its
// opacity, centered at (x, y), of radii rx and ry, in shares of the
// ground's width and height.
type groundLight struct {
	x, y, rx, ry float64
	c            ui.Color
}

// groundLights are a ground's colour under its three lights, drawn into a
// bitmap once for their colours: the same bitmap, stretched, is the
// ground of a window of any size, one image, whose pixels the GPU shades
// once a frame. Painted as stacks of rings, the lights had it shade each
// pixel under them dozens of times, more than a laptop's GPU had time for
// (#325).
type groundLights struct {
	bg  ui.Color
	all [3]groundLight
	bmp *ui.Bitmap
}

// lightsW and lightsH are the size of a ground's bitmap: its pixels, a
// few DIPs apart on the ground, are many to a light, and the GPU blends
// between them as smoothly as the gradients would.
const lightsW, lightsH = 256, 160

// of returns the bitmap of bg under all, drawn again only when they
// change.
func (l *groundLights) of(bg ui.Color, all [3]groundLight) *ui.Bitmap {
	if l.bmp == nil || l.bg != bg || l.all != all {
		*l = groundLights{bg: bg, all: all, bmp: ui.NewBitmap(drawLights(bg, all[:]))}
	}
	return l.bmp
}

// drawLights draws lights over bg, each as its stylesheet's
// radial-gradient(c, transparent 70%) of its radii: c at its opacity at
// the center, fading evenly to nothing 70% of the way out.
func drawLights(bg ui.Color, lights []groundLight) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, lightsW, lightsH))
	for y := range lightsH {
		v := (float64(y) + 0.5) / lightsH
		for x := range lightsW {
			u := (float64(x) + 0.5) / lightsW
			r, g, b := float64(bg.R)/255, float64(bg.G)/255, float64(bg.B)/255
			for _, l := range lights {
				a := float64(l.c.A) / 255 * max(1-math.Hypot((u-l.x)/(0.7*l.rx), (v-l.y)/(0.7*l.ry)), 0)
				r += (float64(l.c.R)/255 - r) * a
				g += (float64(l.c.G)/255 - g) * a
				b += (float64(l.c.B)/255 - b) * a
			}
			img.SetRGBA(x, y, color.RGBA{R: unit(r), G: unit(g), B: unit(b), A: 255})
		}
	}
	return img
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
