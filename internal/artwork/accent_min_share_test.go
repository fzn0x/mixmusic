package artwork

import (
	"image"
	"image/color"
	"testing"
)

// TestPickNeedsAShareOfTheCover: a small red mark on black sets no accent,
// a 2% one does, at the sizes covers come in.
func TestPickNeedsAShareOfTheCover(t *testing.T) {
	red := color.RGBA{0xe0, 0x10, 0x10, 0xff}
	for _, size := range []int{120, 300, 544} {
		for _, share := range []float64{0.0004, 0.0025, 0.005} {
			if got := Pick(markCover(size, black, red, share)); !isNeutral(got) {
				t.Errorf("%d px, mark of %.2f%%: Pick = %v, want a neutral", size, share*100, got)
			}
		}
		if got := Pick(markCover(size, black, red, 0.02)); got != red {
			t.Errorf("%d px, mark of 2%%: Pick = %v, want %v", size, got, red)
		}
	}
}

// TestPickAddsUpSmallMarks: several small marks of one hue count together.
func TestPickAddsUpSmallMarks(t *testing.T) {
	red := color.RGBA{0xe0, 0x10, 0x10, 0xff}
	const size = 300
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := range size {
		for x := range size {
			img.Set(x, y, black)
		}
	}
	// Four squares of 0.64% of the cover each: only their sum passes 1%.
	for i := range 4 {
		for y := range 24 {
			for x := range 24 {
				img.Set(20+i*60+x, 20+y, red)
			}
		}
	}
	if got := Pick(img); got != red {
		t.Errorf("Pick = %v, want the red the marks add up to", got)
	}
}

// TestPickGreyWithRealColour: a mostly grey cover with a real colour area
// takes that colour.
func TestPickGreyWithRealColour(t *testing.T) {
	grey := color.RGBA{0x80, 0x80, 0x80, 0xff}
	teal := color.RGBA{0x10, 0xa0, 0xb0, 0xff}
	if got := Pick(markCover(300, grey, teal, 0.15)); got != teal {
		t.Errorf("Pick = %v, want %v", got, teal)
	}
}

// TestPickKeepsGoodAccents: covers that gave a good accent keep it, from a
// big block of one hue to two hues side by side.
func TestPickKeepsGoodAccents(t *testing.T) {
	orange := color.RGBA{0xe0, 0x80, 0x20, 0xff}
	cases := []struct {
		name string
		img  image.Image
		want color.RGBA
	}{
		{"half orange on dark", markCover(300, dark, orange, 0.5), orange},
		{"all orange", markCover(300, orange, orange, 0.5), orange},
		{"5% orange on dark", markCover(300, dark, orange, 0.05), orange},
	}
	for _, c := range cases {
		if got := Pick(c.img); got != c.want {
			t.Errorf("%s: Pick = %v, want %v", c.name, got, c.want)
		}
	}
}
