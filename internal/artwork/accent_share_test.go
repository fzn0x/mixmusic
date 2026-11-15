package artwork

import (
	"image"
	"image/color"
	"math"
	"testing"
)

// markCover makes a size x size cover of bg with a square mark of the given
// share of its area, in the top left corner.
func markCover(size int, bg, mark color.RGBA, share float64) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	side := int(float64(size)*math.Sqrt(share) + 0.5)
	for y := range size {
		for x := range size {
			if x < side && y < side {
				img.Set(x, y, mark)
			} else {
				img.Set(x, y, bg)
			}
		}
	}
	return img
}

func isNeutral(c color.RGBA) bool {
	d := func(a, b uint8) int {
		if a > b {
			return int(a - b)
		}
		return int(b - a)
	}
	return d(c.R, c.G) < 24 && d(c.G, c.B) < 24 && d(c.R, c.B) < 24
}

var (
	black   = color.RGBA{0, 0, 0, 0xff}
	dark    = color.RGBA{0x30, 0x30, 0x30, 0xff}
	pureRed = color.RGBA{0xff, 0, 0, 0xff}
)

// TestPickKeepsPureColours: 40% pure red on dark grey gives the red, not
// the muddy average; pure green and blue too.
func TestPickKeepsPureColours(t *testing.T) {
	for _, c := range []color.RGBA{pureRed, {0, 0xff, 0, 0xff}, {0, 0, 0xff, 0xff}} {
		if got := Pick(markCover(300, dark, c, 0.4)); got != c {
			t.Errorf("Pick of 40%% %v on grey = %v", c, got)
		}
	}
}

// TestPickStillSkipsNearWhite: a pale pink, nearly white, makes no accent.
func TestPickStillSkipsNearWhite(t *testing.T) {
	pale := color.RGBA{0xff, 0xf0, 0xf0, 0xff}
	if got := Pick(markCover(300, dark, pale, 0.4)); !isNeutral(got) {
		t.Errorf("Pick took a near white %v as the accent", got)
	}
}
