package artwork

import (
	"image"
	"image/color"
	"testing"

	"github.com/fzn0x/mixmusic/internal/hct"
)

// TestGreyCoverInMaterial: a cover with no vivid colour gives Pick's
// average, a black or grey; the Material scheme of it stays grey, and
// covers one step apart give the same accent (#573).
func TestGreyCoverInMaterial(t *testing.T) {
	accent := func(g color.RGBA) (hct.ARGB, float64) {
		seed := Pick(solid(g))
		s := hct.Core(hct.RGB(seed.R, seed.G, seed.B)).Scheme(true)
		r, gg, b := s.Primary.RGB()
		return s.Primary, hct.FromARGB(hct.RGB(r, gg, b)).Chroma
	}
	for _, g := range []color.RGBA{{0x0a, 0x0a, 0x0a, 0xff}, {0x10, 0x10, 0x12, 0xff}, {0x80, 0x80, 0x80, 0xff}, {0x12, 0x10, 0x10, 0xff}} {
		if _, c := accent(g); c >= 8 {
			t.Errorf("a plain %s cover gives an accent of HCT chroma %.1f, want under 8", Hex(g), c)
		}
	}
	a, b := color.RGBA{0x0d, 0x0c, 0x0c, 0xff}, color.RGBA{0x0d, 0x0d, 0x0d, 0xff}
	if pa, _ := accent(a); pa != func() hct.ARGB { p, _ := accent(b); return p }() {
		t.Errorf("the covers %s and %s look alike but give different accents", Hex(a), Hex(b))
	}
}

func solid(c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 120, 120))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, c.A
	}
	return img
}
