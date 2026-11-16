package artwork

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"testing"
)

// gifPalette is red, blue, green and see-through.
var gifPalette = color.Palette{color.RGBA{R: 220, A: 255}, color.RGBA{B: 220, A: 255}, color.RGBA{G: 220, A: 255}, color.RGBA{}}

// movingGIF is a GIF w by h of frames frames, each blue in its centre
// square and red outside it, but for a green band that moves along: a
// frame each tenth of a second, the last undone to what was under it.
func movingGIF(t testing.TB, w, h, frames int) []byte {
	t.Helper()
	g := &gif.GIF{Config: image.Config{Width: w, Height: h, ColorModel: gifPalette}}
	side := min(w, h)
	x0, y0 := (w-side)/2, (h-side)/2
	for i := range frames {
		f := image.NewPaletted(image.Rect(0, 0, w, h), gifPalette)
		for y := range h {
			for x := range w {
				switch {
				case x >= x0+i*side/frames && x < x0+i*side/frames+4:
					f.SetColorIndex(x, y, 2)
				case x >= x0 && x < x0+side && y >= y0 && y < y0+side:
					f.SetColorIndex(x, y, 1)
				}
			}
		}
		g.Image = append(g.Image, f)
		g.Delay = append(g.Delay, 10)
		g.Disposal = append(g.Disposal, gif.DisposalNone)
	}
	g.Disposal[frames-1] = gif.DisposalPrevious
	var b bytes.Buffer
	if err := gif.EncodeAll(&b, g); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// TestSizeOfGIF reads what a GIF takes decoded from its blocks: its
// frames, their pixels and its picture, without decoding it. What isn't
// a GIF, or stops short, doesn't read.
func TestSizeOfGIF(t *testing.T) {
	data := movingGIF(t, 30, 20, 4)
	s, err := SizeOfGIF(data)
	if err != nil {
		t.Fatal(err)
	}
	if s.Frames != 4 || s.Pixels != 4*30*20 || s.Width != 30 || s.Height != 20 || !s.Previous {
		t.Errorf("read %+v", s)
	}
	if got, want := s.Memory(), int64(4*30*20+2*4*30*20); got != want {
		t.Errorf("it takes %d bytes, not %d", got, want)
	}
	if !IsGIF(data) || IsGIF([]byte("\x89PNG")) {
		t.Error("GIFs aren't told from the rest")
	}
	for name, bad := range map[string][]byte{"png": encodePNG(t, banded(4, 4, 255)), "short": data[:len(data)/2], "header": data[:12]} {
		if _, err := SizeOfGIF(bad); err == nil {
			t.Errorf("%s reads", name)
		}
	}
}

// TestSquareGIF keeps the centre square of a GIF that moves, at most px
// across, with its frames' delays and how each is undone, and its first
// frame as a still; a small square one stays as it is, and one of a
// single frame doesn't move.
func TestSquareGIF(t *testing.T) {
	data := movingGIF(t, 300, 200, 3)
	moving, still, err := SquareGIF(data, 100)
	if err != nil {
		t.Fatal(err)
	}
	g, err := gif.DecodeAll(bytes.NewReader(moving))
	if err != nil {
		t.Fatal(err)
	}
	if g.Config.Width != 100 || g.Config.Height != 100 || len(g.Image) != 3 {
		t.Fatalf("the square is %d × %d of %d frames", g.Config.Width, g.Config.Height, len(g.Image))
	}
	for i, f := range g.Image {
		if g.Delay[i] != 10 {
			t.Errorf("frame %d shows for %d", i, g.Delay[i])
		}
		// The green band moves along the square, a third at a time.
		if c := f.At(i*100/3, 50); c != gifPalette[2] {
			t.Errorf("frame %d at its band is %v", i, c)
		}
		if c := f.At(99-i, 2); c != gifPalette[1] {
			t.Errorf("frame %d near its corner is %v, not the GIF's centre", i, c)
		}
	}
	if g.Disposal[2] != gif.DisposalPrevious {
		t.Errorf("the frames are undone as %v", g.Disposal)
	}
	img, format, err := image.Decode(bytes.NewReader(still))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); format != "jpeg" || b.Dx() != 100 || b.Dy() != 100 {
		t.Errorf("the still is a %s of %v", format, b)
	}

	small := movingGIF(t, 64, 64, 2)
	if moving, _, err := SquareGIF(small, 100); err != nil || !bytes.Equal(moving, small) {
		t.Errorf("a small square GIF became %d bytes: %v", len(moving), err)
	}
	if moving, still, err := SquareGIF(movingGIF(t, 64, 40, 1), 100); err != nil || moving != nil || len(still) == 0 {
		t.Errorf("a GIF of one frame: %d bytes moving, %d still, %v", len(moving), len(still), err)
	}
}
