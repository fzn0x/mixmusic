package hct

import (
	"math"
	"testing"
)

// The expected values are those of Material Color Utilities' own tests.

func near(t *testing.T, what string, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Errorf("%s = %.4f, want %.4f", what, got, want)
	}
}

func TestViewingConditions(t *testing.T) {
	near(t, "n", vc.n, 0.184, 0.001)
	near(t, "aw", vc.aw, 29.981, 0.001)
	near(t, "nbb", vc.nbb, 1.017, 0.001)
	near(t, "c", vc.c, 0.69, 0.001)
	near(t, "nc", vc.nc, 1.0, 0.001)
	near(t, "rgbD[0]", vc.rgbD[0], 1.021, 0.001)
	near(t, "rgbD[1]", vc.rgbD[1], 0.986, 0.001)
	near(t, "rgbD[2]", vc.rgbD[2], 0.934, 0.001)
	near(t, "fl", vc.fl, 0.388, 0.001)
	near(t, "flRoot", vc.flRoot, 0.789, 0.001)
	near(t, "z", vc.z, 1.909, 0.001)
}

func TestCAM16(t *testing.T) {
	for _, c := range []struct {
		name                    string
		argb                    ARGB
		hue, chroma, j, m, s, q float64
	}{
		{"red", 0xffff0000, 27.408, 113.358, 46.445, 89.494, 91.890, 105.989},
		{"green", 0xff00ff00, 142.140, 108.410, 79.332, 85.588, 78.605, 138.520},
		{"blue", 0xff0000ff, 282.788, 87.231, 25.466, 68.867, 93.675, 78.481},
		{"white", 0xffffffff, 209.492, 2.869, 100, 2.265, 12.068, 155.521},
		{"black", 0xff000000, 0, 0, 0, 0, 0, 0},
	} {
		cam := camOf(c.argb)
		near(t, c.name+" hue", cam.hue, c.hue, 0.001)
		near(t, c.name+" chroma", cam.chroma, c.chroma, 0.001)
		near(t, c.name+" J", cam.j, c.j, 0.001)
		near(t, c.name+" M", cam.m, c.m, 0.001)
		near(t, c.name+" s", cam.s, c.s, 0.001)
		near(t, c.name+" Q", cam.q, c.q, 0.001)
	}
}

func TestFromARGB(t *testing.T) {
	g := FromARGB(0xff00ff00)
	near(t, "green hue", g.Hue, 142.139, 0.01)
	near(t, "green chroma", g.Chroma, 108.410, 0.01)
	near(t, "green tone", g.Tone, 87.737, 0.01)
	b := FromARGB(0xff0000ff)
	near(t, "blue hue", b.Hue, 282.788, 0.01)
	near(t, "blue chroma", b.Chroma, 87.230, 0.01)
	near(t, "blue tone", b.Tone, 32.302, 0.01)
	// Out of gamut, the chroma is the most sRGB has.
	b90 := From(282.788, 87.230, 90)
	near(t, "blue 90 hue", b90.Hue, 282.239, 0.01)
	near(t, "blue 90 chroma", b90.Chroma, 19.144, 0.01)
	near(t, "blue 90 tone", b90.Tone, 90.035, 0.01)
}

func onBoundary(c ARGB) bool {
	r, g, b := c.RGB()
	return r == 0 || r == 255 || g == 0 || g == 255 || b == 0 || b == 255
}

func TestSolver(t *testing.T) {
	for hue := 15.0; hue < 360; hue += 30 {
		for chroma := 0.0; chroma <= 100; chroma += 10 {
			for tone := 20.0; tone <= 80; tone += 10 {
				c := From(hue, chroma, tone)
				if chroma > 0 && math.Abs(c.Hue-hue) > 4 {
					t.Errorf("From(%v, %v, %v) has hue %.2f", hue, chroma, tone, c.Hue)
				}
				if c.Chroma < 0 || c.Chroma > chroma+2.5 {
					t.Errorf("From(%v, %v, %v) has chroma %.2f", hue, chroma, tone, c.Chroma)
				}
				if c.Chroma < chroma-2.5 && !onBoundary(c.ARGB()) {
					t.Errorf("From(%v, %v, %v) lost chroma inside the gamut: %.2f", hue, chroma, tone, c.Chroma)
				}
				near(t, "tone", c.Tone, tone, 0.5)
			}
		}
	}
}

// TestCore checks the core palettes of blue against Material Color
// Utilities' own: its primary is blue's own, its secondary blue at 16.
func TestCore(t *testing.T) {
	p := Core(0xff0000ff)
	for tone, want := range map[float64]ARGB{90: 0xffe0e0ff, 40: 0xff343dff, 10: 0xff00006e} {
		if got := p.Primary.Tone(tone); got != want {
			t.Errorf("primary %v = %08x, want %08x", tone, uint32(got), uint32(want))
		}
	}
	for tone, want := range map[float64]ARGB{90: 0xffe1e0f9, 40: 0xff5c5d72, 10: 0xff191a2c} {
		if got := p.Secondary.Tone(tone); got != want {
			t.Errorf("secondary %v = %08x, want %08x", tone, uint32(got), uint32(want))
		}
	}
}

func TestPalette(t *testing.T) {
	blue := PaletteOf(0xff0000ff)
	for tone, want := range map[float64]ARGB{
		100: 0xffffffff, 95: 0xfff1efff, 90: 0xffe0e0ff, 80: 0xffbec2ff, 70: 0xff9da3ff, 60: 0xff7c84ff,
		50: 0xff5a64ff, 40: 0xff343dff, 30: 0xff0000ef, 20: 0xff0001ac, 10: 0xff00006e, 0: 0xff000000,
	} {
		if got := blue.Tone(tone); got != want {
			t.Errorf("tone %v = %08x, want %08x", tone, uint32(got), uint32(want))
		}
	}
}

// contrast is the WCAG contrast of a and b.
func contrast(a, b ARGB) float64 {
	la, lb := yFromLstar(lstarOf(a))/100, yFromLstar(lstarOf(b))/100
	return (max(la, lb) + 0.05) / (min(la, lb) + 0.05)
}

// TestSchemeContrast checks that every seed's scheme, light and dark,
// keeps its text readable: 4.5:1 for ink on its ground.
func TestSchemeContrast(t *testing.T) {
	for _, seed := range []ARGB{0xffe6007e, 0xff6750a4, 0xff0000ff, 0xffffff00, 0xff00ff00, 0xff00bde3, 0xff808080, 0xffff8000} {
		for _, dark := range []bool{false, true} {
			s := Core(seed).Scheme(dark)
			for _, p := range []struct {
				name     string
				ink, bg  ARGB
				minRatio float64
			}{
				{"on-surface", s.OnSurface, s.Surface, 4.5},
				{"on-surface-variant", s.OnSurfaceVariant, s.Surface, 4.5},
				{"on-surface-variant on highest", s.OnSurfaceVariant, s.SurfaceContainerHighest, 4.5},
				{"primary", s.Primary, s.Surface, 4.5},
				{"on-primary", s.OnPrimary, s.Primary, 4.5},
				{"on-primary-container", s.OnPrimaryContainer, s.PrimaryContainer, 4.5},
				{"on-secondary-container", s.OnSecondaryContainer, s.SecondaryContainer, 4.5},
				{"on-tertiary-container", s.OnTertiaryContainer, s.TertiaryContainer, 4.5},
				{"error", s.Error, s.Surface, 4.5},
				{"outline", s.Outline, s.Surface, 3},
			} {
				if r := contrast(p.ink, p.bg); r < p.minRatio {
					t.Errorf("seed %08x, dark %v: %s at %.2f:1", uint32(seed), dark, p.name, r)
				}
			}
		}
	}
}
