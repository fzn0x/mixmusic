package artwork

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

// banded is a w by h picture, blue in its centre square and red outside
// it, as a 16:9 thumbnail around a cover; see-through when alpha is below
// 255.
func banded(w, h int, alpha uint8) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	side := min(w, h)
	x0, y0 := (w-side)/2, (h-side)/2
	for y := range h {
		for x := range w {
			c := color.NRGBA{R: 220, A: alpha}
			if x >= x0 && x < x0+side && y >= y0 && y < y0+side {
				c = color.NRGBA{B: 220, A: alpha}
			}
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func encodeJPEG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// centreBlue reports whether c is the bands' blue, as a JPEG gives it back.
func centreBlue(c color.Color) bool {
	r, g, b, _ := c.RGBA()
	return r>>8 < 40 && g>>8 < 40 && b>>8 > 180
}

// TestSquare crops a cover to its centre square, as the windows show it,
// and shrinks it to the size asked: a music video's thumbnail to its
// middle, a tall picture too, a large cover to the size.
func TestSquare(t *testing.T) {
	for _, c := range []struct {
		name string
		data []byte
		px   int
		side int
	}{
		{"a 16:9 thumbnail", encodeJPEG(t, banded(320, 180, 255)), 512, 180},
		{"a tall picture", encodePNG(t, banded(200, 400, 255)), 512, 200},
		{"a large cover", encodeJPEG(t, banded(1280, 720, 255)), 512, 512},
	} {
		got, err := Square(c.data, c.px)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		img, format, err := image.Decode(bytes.NewReader(got))
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		b := img.Bounds()
		if format != "jpeg" || b.Dx() != c.side || b.Dy() != c.side {
			t.Errorf("%s: a %s of %dx%d, want a jpeg of %dx%d", c.name, format, b.Dx(), b.Dy(), c.side, c.side)
			continue
		}
		// The bands are cut off: near the edges is the centre's blue, a
		// little in, where JPEG's blocks no longer blur the edge of the
		// bands into it.
		in, out := c.side/10, c.side-1-c.side/10
		for _, p := range []image.Point{{in, in}, {out, in}, {c.side / 2, c.side / 2}, {in, out}, {out, out}} {
			if at := img.At(p.X, p.Y); !centreBlue(at) {
				t.Errorf("%s: %v at %v, not the centre's blue", c.name, at, p)
			}
		}
	}
}

// TestSquareKeeps hands a square JPEG or PNG no larger than asked back as
// it is, and a see-through one as a PNG, which keeps what shows through.
func TestSquareKeeps(t *testing.T) {
	for _, data := range [][]byte{encodeJPEG(t, banded(300, 300, 255)), encodePNG(t, banded(512, 512, 255))} {
		got, err := Square(data, 512)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, data) {
			t.Errorf("a square picture of %d bytes came back as %d other bytes", len(data), len(got))
		}
	}
	got, err := Square(encodePNG(t, banded(800, 600, 128)), 512)
	if err != nil {
		t.Fatal(err)
	}
	img, format, err := image.Decode(bytes.NewReader(got))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, a := img.At(10, 10).RGBA(); format != "png" || img.Bounds().Dx() != 512 || a>>8 > 140 || a>>8 < 110 {
		t.Errorf("a see-through picture: a %s of %v, alpha %d", format, img.Bounds(), a>>8)
	}
}

// TestSquareNotAnImage refuses what isn't a picture.
func TestSquareNotAnImage(t *testing.T) {
	if _, err := Square([]byte("<html>not found</html>"), 512); err == nil {
		t.Error("a web page squared")
	}
}
