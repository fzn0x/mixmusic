package app

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"os"
	"path/filepath"
	"testing"
)

// testGIF is a GIF w by h of frames frames, each a shade of blue in its
// centre square and red outside it.
func testGIF(t *testing.T, w, h, frames int) []byte {
	t.Helper()
	pal := color.Palette{color.RGBA{R: 220, A: 255}}
	for i := range frames {
		pal = append(pal, color.RGBA{B: uint8(200 + i), A: 255})
	}
	g := &gif.GIF{Config: image.Config{Width: w, Height: h, ColorModel: pal}}
	side := min(w, h)
	x0, y0 := (w-side)/2, (h-side)/2
	for i := range frames {
		f := image.NewPaletted(image.Rect(0, 0, w, h), pal)
		for y := y0; y < y0+side; y++ {
			for x := x0; x < x0+side; x++ {
				f.SetColorIndex(x, y, uint8(1+i))
			}
		}
		g.Image = append(g.Image, f)
		g.Delay = append(g.Delay, 8)
	}
	var b bytes.Buffer
	if err := gif.EncodeAll(&b, g); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// TestMixCoverGIF gives a mix the centre square of a GIF that moves: its
// first frame is the still that its tiles show, and the GIF beside it, as
// large and of every frame, moves on its page (#569). A GIF of one frame
// is a still alone, and a cover in its place, or the songs' covers, leave
// no file of it behind.
func TestMixCoverGIF(t *testing.T) {
	c, p, _ := coverMix(t)
	if err := c.SetMixCover(p.ID, writeImage(t, "wide.gif", testGIF(t, 1200, 900, 3))); err != nil {
		t.Fatal(err)
	}
	still, img := mixCover(t, c, p.ID)
	if b := img.Bounds(); filepath.Ext(still) != ".jpg" || b.Dx() != 600 || b.Dy() != 600 || !blue(img.At(2, 2)) {
		t.Errorf("the still is %s of %v", still, b)
	}
	url := c.Playlists()[0].Artwork[0].URL
	moving, ok := c.MovingMixCover(url)
	if !ok {
		t.Fatal("no GIF beside the still")
	}
	data, err := os.ReadFile(moving)
	if err != nil {
		t.Fatal(err)
	}
	g, err := gif.DecodeAll(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if g.Config.Width != 600 || g.Config.Height != 600 || len(g.Image) != 3 || g.Delay[2] != 8 {
		t.Errorf("the GIF is %d × %d of %d frames, %v", g.Config.Width, g.Config.Height, len(g.Image), g.Delay)
	}
	if _, ok := c.MovingMixCover("mixcover:../" + filepath.Base(moving)); ok {
		t.Error("an address outside the folder finds a GIF")
	}

	if err := c.SetMixCover(p.ID, writeImage(t, "one.gif", testGIF(t, 300, 300, 1))); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(moving); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the GIF replaced stays: %v", err)
	}
	if _, ok := c.MovingMixCover(c.Playlists()[0].Artwork[0].URL); ok {
		t.Error("a GIF of one frame moves")
	}
	if err := c.SetMixCover(p.ID, writeImage(t, "again.gif", testGIF(t, 200, 200, 2))); err != nil {
		t.Fatal(err)
	}
	moving, ok = c.MovingMixCover(c.Playlists()[0].Artwork[0].URL)
	if !ok {
		t.Fatal("no GIF beside the still")
	}
	if err := c.UseSongsCovers(p.ID); err != nil {
		t.Fatal(err)
	}
	if entries, _ := os.ReadDir(c.a.mixCovers()); len(entries) != 0 || moving == "" {
		t.Errorf("back to the songs' covers, the folder holds %d files", len(entries))
	}
}
