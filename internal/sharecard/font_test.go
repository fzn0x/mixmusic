package sharecard

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

// TestSystemFontReadWhenNeeded opens a font as a system one: it knows its
// letters at once, reads the rest only when a card draws one of them,
// keeps it for the next card a while, then lets it go, and reads it again
// for the card after.
func TestSystemFontReadWhenNeeded(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "fonts", "NotoSansThai-subset.ttf"))
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "NotoSansThai-Regular.ttf")
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatal(err)
	}
	fonts := loadFonts([]string{file})
	if len(fonts) != 1 {
		t.Fatalf("opened %d fonts", len(fonts))
	}
	f := fonts[0]
	if !f.covers('ท') || f.covers('A') {
		t.Error("the font's letters aren't known")
	}
	if f.weak.Value() != nil {
		t.Fatal("the font was read before a card needed it")
	}
	ink := drawThai(t, f)
	if ink == 0 {
		t.Fatal("the font's letters draw nothing")
	}
	runtime.GC()
	if d := f.weak.Value(); d == nil || !slices.Contains(kept.fonts, d) {
		t.Error("the font wasn't kept for the next card")
	}
	letGo()
	runtime.GC()
	runtime.GC()
	if f.weak.Value() != nil {
		t.Error("the font stayed in memory after it was let go")
	}
	if again := drawThai(t, f); again != ink {
		t.Errorf("read again, the font draws %d pixels, not %d", again, ink)
	}
}

// drawThai draws a card of Thai letters in f, and counts its ink.
func drawThai(t *testing.T, f *Font) int {
	t.Helper()
	fam := Family{Faces: []Face{{400, selawik(t)}}, Fallback: []*Font{f}}
	c := Card{Size: Square, Colors: dark, Lines: []Line{{Text: "สวัสดี", Size: 80, Weight: 400}}}
	img, err := c.Render(fam)
	if err != nil {
		t.Fatal(err)
	}
	ground, _ := Card{Size: Square, Colors: dark}.Render(fam)
	return ink(img, ground)
}
