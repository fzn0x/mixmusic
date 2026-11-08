package main

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// TestBadges keeps a badge in brand/discord for each the tool draws, of
// its size, whose middle is its mark and whose corners its ground: the
// app names them in internal/app, and Discord reads them from GitHub.
func TestBadges(t *testing.T) {
	for name, b := range badges {
		f, err := os.Open(filepath.Join("..", "..", "brand", "discord", name+".png"))
		if err != nil {
			t.Fatalf("%s: %v; run go run ./tools/discordbadges", name, err)
		}
		img, err := png.Decode(f)
		f.Close()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if r := img.Bounds(); r.Dx() != size || r.Dy() != size {
			t.Errorf("%s is %v, not %d pixels square", name, r, size)
		}
		drawn := draw(b.mark, b.ink)
		for _, at := range [][2]int{{0, 0}, {size - 1, size - 1}, {size / 2, size / 2}, {size * 3 / 8, size / 2}} {
			r, g, bl, _ := img.At(at[0], at[1]).RGBA()
			wr, wg, wb, _ := drawn.At(at[0], at[1]).RGBA()
			if r != wr || g != wg || bl != wb {
				t.Errorf("%s at %v is %x %x %x, not %x %x %x: run go run ./tools/discordbadges", name, at, r, g, bl, wr, wg, wb)
			}
		}
	}
}
