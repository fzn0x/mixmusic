package main

import (
	"image/color"
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
)

// lightAt is the colour the stylesheets' radial-gradient(c, transparent
// 70%) of each light gives at (u, v) of a ground of bg, in shares of its
// width and height.
func lightAt(bg ui.Color, lights []groundLight, u, v float64) color.RGBA {
	c := [3]float64{float64(bg.R), float64(bg.G), float64(bg.B)}
	for _, l := range lights {
		a := float64(l.c.A) / 255 * max(1-math.Hypot((u-l.x)/(0.7*l.rx), (v-l.y)/(0.7*l.ry)), 0)
		for i, to := range [3]uint8{l.c.R, l.c.G, l.c.B} {
			c[i] += (float64(to) - c[i]) * a
		}
	}
	return color.RGBA{R: uint8(math.Round(c[0])), G: uint8(math.Round(c[1])), B: uint8(math.Round(c[2])), A: 255}
}

// levelsApart is how many levels of a channel a and b are apart, at most.
func levelsApart(a, b color.RGBA) int {
	d := func(x, y uint8) int { return int(max(x, y) - min(x, y)) }
	return max(d(a.R, b.R), d(a.G, b.G), d(a.B, b.B))
}

// TestLightsFade draws each light of a ground as its stylesheet's
// radial-gradient(c, transparent 70%): c at its opacity at the center,
// fading evenly across and down the ellipse, nothing past 70% of its
// radii, and the lights over each other.
func TestLightsFade(t *testing.T) {
	bg := ui.Color{R: 7, G: 10, B: 22, A: 255}
	lights := []groundLight{
		{0.3, 0.4, 0.5, 0.6, ui.Color{R: 200, G: 100, B: 50, A: 128}},
		{0.9, 1.1, 0.4, 0.3, ui.Color{R: 40, G: 90, B: 255, A: 64}},
	}
	img := drawLights(bg, lights)
	for y := range lightsH {
		for x := range lightsW {
			want := lightAt(bg, lights, (float64(x)+0.5)/lightsW, (float64(y)+0.5)/lightsH)
			if got := img.RGBAAt(x, y); levelsApart(got, want) > 0 {
				t.Fatalf("at %d,%d the lights are %v, want %v", x, y, got, want)
			}
		}
	}
	// The center of the first light, and past its reach.
	if got, want := img.RGBAAt(77, 64), lightAt(bg, lights[:1], 0.3, 0.4); levelsApart(got, want) > 1 {
		t.Errorf("at the first light's center, %v, want %v", got, want)
	}
	if got := img.RGBAAt(0, lightsH-1); got != (color.RGBA{R: 7, G: 10, B: 22, A: 255}) {
		t.Errorf("past the lights, %v, want the ground", got)
	}
}

// TestAuroraGround paints Aurora's lights still as its stylesheet's
// gradients over its ground: the bitmap they are drawn into stretches over
// a ground 140% of the window's size.
func TestAuroraGround(t *testing.T) {
	s := newTestShell(t)
	wear(s, "aurora", map[string]string{"motion": "false"})
	const w, h = 640, 400
	tt := ui.NewTester(func(ctx *ui.Context) {
		s.ground(ctx, ui.Column(ctx).Fill().Background(s.lk.bg))
	}, w, h)
	tt.Frame()
	img := tt.Image()
	k := &s.lk
	lights := []groundLight{
		{0.15, 0.10, 0.60, 0.50, withAlpha(k.fx.tint, 0.45)},
		{0.85, 0.25, 0.50, 0.45, withAlpha(k.accent, 0.30)},
		{0.50, 1, 0.70, 0.60, ui.Color{R: 40, G: 90, B: 255, A: 64}},
	}
	for _, pt := range [][2]int{{0, 0}, {134, 56}, {200, 100}, {320, 200}, {500, 90}, {639, 140}, {320, 399}, {600, 380}} {
		u, v := (float64(pt[0])+0.5)/(1.4*w), (float64(pt[1])+0.5)/(1.4*h)
		want := lightAt(k.bg, lights, u, v)
		if got := img.RGBAAt(pt[0], pt[1]); levelsApart(got, want) > 2 {
			t.Errorf("at %v Aurora's ground is %v, want %v", pt, got, want)
		}
	}
}
