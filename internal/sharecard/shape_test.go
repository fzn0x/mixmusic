package sharecard

import (
	"image"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gotext "github.com/go-text/typesetting/font"
)

// fixture reads a font of testdata/fonts: small subsets of open fonts, so
// that the tests draw the same on every system.
func fixture(t testing.TB, name string) *Font {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "fonts", name))
	if err != nil {
		t.Fatal(err)
	}
	f, err := ParseFont(data)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// selawik is the bundled face a card sets Latin letters in.
func selawik(t testing.TB) *Font {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "fonts", "selawk.ttf"))
	if err != nil {
		t.Fatal(err)
	}
	f, err := ParseFont(data)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// inkBox is the box of the pixels of img that differ from ground.
func inkBox(img, ground *image.NRGBA) image.Rectangle {
	var box image.Rectangle
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if img.NRGBAAt(x, y) != ground.NRGBAAt(x, y) {
				box = box.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return box
}

// glyphBox is where glyph g of a row's run lands: its extents, at px, from
// the pen at x on the baseline at y, moved by the offsets the shaper gave.
func glyphBox(t *testing.T, ts *typesetter, r run, gid gotext.GID, xoff, yoff, px, x, y float64) image.Rectangle {
	t.Helper()
	e, ok := ts.sh.face(r.data, 400).GlyphExtents(gid)
	if !ok {
		t.Fatalf("glyph %d has no extents", gid)
	}
	s := px / r.data.upem
	gx, gy := x+xoff*s, y-yoff*s
	return image.Rect(int(math.Floor(gx+float64(e.XBearing)*s)), int(math.Floor(gy-float64(e.YBearing)*s)),
		int(math.Ceil(gx+float64(e.XBearing+e.Width)*s)), int(math.Ceil(gy-float64(e.YBearing+e.Height)*s)))
}

// near reports whether a and b are the same box, give or take a pixel of
// antialiasing on each side.
func near(a, b image.Rectangle) bool {
	d := func(p, q int) bool { return p-q <= 1 && q-p <= 1 }
	return d(a.Min.X, b.Min.X) && d(a.Min.Y, b.Min.Y) && d(a.Max.X, b.Max.X) && d(a.Max.Y, b.Max.Y)
}

// drawn draws text in prim, with fallback, at 120 pixels on a blank
// ground, its baseline at 300 from x 100, and gives the shaped row too.
func drawn(t *testing.T, prim *Font, fallback []*Font, text string) (img, ground *image.NRGBA, ts *typesetter, s *shaped) {
	t.Helper()
	ts = newTypesetter(Family{Faces: []Face{{400, prim}}, Fallback: fallback})
	ground = image.NewNRGBA(image.Rect(0, 0, 900, 500))
	paintGround(ground, dark)
	img = image.NewNRGBA(ground.Bounds())
	copy(img.Pix, ground.Pix)
	ts.drawRow(img, prim, 400, 120, text, isRTL(text), 100, 300, dark.Text)
	return img, ground, ts, ts.shape(prim, 400, text, isRTL(text))
}

// TestDevanagariConjunct shapes क्ष, ka, virama and ssa, as the one glyph
// of the conjunct the font has for it, and draws that glyph where the
// shaper puts it, not the three letters side by side.
func TestDevanagariConjunct(t *testing.T) {
	deva := fixture(t, "NotoSansDevanagari-subset.ttf")
	img, ground, ts, s := drawn(t, deva, nil, "क्ष")
	if len(s.runs) != 1 || len(s.runs[0].glyphs) != 1 {
		t.Fatalf("क्ष shapes as %d runs, the first of %d glyphs, not one conjunct", len(s.runs), len(s.runs[0].glyphs))
	}
	g := s.runs[0].glyphs[0]
	for _, r := range "क्ष" {
		if gid, _ := deva.cmap.Lookup(r); gid == g.GlyphID {
			t.Fatalf("the conjunct is the glyph of %q", r)
		}
	}
	want := glyphBox(t, ts, s.runs[0], g.GlyphID, float64(g.XOffset)/64, float64(g.YOffset)/64, 120, 100, 300)
	if got := inkBox(img, ground); !near(got, want) {
		t.Errorf("the conjunct's ink is at %v, the shaper put it at %v", got, want)
	}
	// Drawn letter by letter, the three would take more room.
	var apart float64
	for _, r := range "क्ष" {
		apart += newTypesetter(Family{Faces: []Face{{400, deva}}}).advance(deva, 400, 120, string(r), false)
	}
	if w := ts.advance(deva, 400, 120, "क्ष", false); w >= apart {
		t.Errorf("the conjunct is %.1f pixels wide, the letters apart %.1f", w, apart)
	}
}

// TestThaiStackedMark shapes ที่, tho thahan with sara ii and mai ek: the
// font lifts the tone mark above the vowel, and the card draws it there.
func TestThaiStackedMark(t *testing.T) {
	thai := fixture(t, "NotoSansThai-subset.ttf")
	img, ground, ts, s := drawn(t, thai, nil, "ที่")
	if len(s.runs) != 1 || len(s.runs[0].glyphs) != 3 {
		t.Fatalf("ที่ shapes as %d runs, not one of three glyphs", len(s.runs))
	}
	r := s.runs[0]
	mark := r.glyphs[2]
	// Noto Sans Thai lifts it by putting a higher glyph in its place;
	// another font might move it.
	if own, _ := thai.cmap.Lookup('่'); mark.GlyphID == own && mark.YOffset <= 0 {
		t.Fatal("the shaper left the tone mark as it was: there is nothing to test")
	}
	pen := 100.0
	for _, g := range r.glyphs[:2] {
		pen += float64(g.Advance) / 64 * 120 / r.data.upem
	}
	want := glyphBox(t, ts, r, mark.GlyphID, float64(mark.XOffset)/64, float64(mark.YOffset)/64, 120, pen, 300)
	// The mark is the topmost ink: its top is where the shaper put it.
	got := inkBox(img, ground)
	if d := got.Min.Y - want.Min.Y; d < -1 || d > 1 {
		t.Errorf("the ink's top is at %d, the shaper put the tone mark's top at %d", got.Min.Y, want.Min.Y)
	}
	// Without the vowel, the tone mark sits lower.
	low, lowGround, _, _ := drawn(t, thai, nil, "ท่")
	if lowTop := inkBox(low, lowGround).Min.Y; lowTop <= got.Min.Y {
		t.Errorf("ท่ reaches %d, as high as ที่'s %d: the mark wasn't lifted", lowTop, got.Min.Y)
	}
}

// TestArabicJoins shapes Arabic by the font's rules, which arabic.go did by
// hand before: each ba takes its initial, medial or final form, and the
// run reads right to left, its last letter drawn first, at the left.
func TestArabicJoins(t *testing.T) {
	arabic := fixture(t, "NotoSansArabic-subset.ttf")
	ts := newTypesetter(Family{Faces: []Face{{400, arabic}}})
	s := ts.shape(arabic, 400, "ببب", true)
	if len(s.runs) != 1 || len(s.runs[0].glyphs) != 3 || s.runs[0].level%2 != 1 {
		t.Fatalf("ببب shapes as %+v, not one run of three glyphs read right to left", s.runs)
	}
	isolated, _ := arabic.cmap.Lookup('ب')
	g := s.runs[0].glyphs
	if g[0].GlyphID == isolated || g[1].GlyphID == isolated || g[2].GlyphID == isolated ||
		g[0].GlyphID == g[1].GlyphID || g[1].GlyphID == g[2].GlyphID || g[0].GlyphID == g[2].GlyphID {
		t.Errorf("the three ba are glyphs %d, %d and %d, the isolated one %d: they don't join", g[0].GlyphID, g[1].GlyphID, g[2].GlyphID, isolated)
	}
	if g[0].TextIndex() != 2 || g[2].TextIndex() != 0 {
		t.Errorf("the glyphs show the letters %d, %d, %d from the left, not the last first", g[0].TextIndex(), g[1].TextIndex(), g[2].TextIndex())
	}
	// Lam and alef make one glyph.
	if s := ts.shape(arabic, 400, "لا", true); len(s.runs) != 1 || len(s.runs[0].glyphs) != 1 {
		t.Errorf("lam alef shapes as %+v, not one glyph", s.runs)
	}
}

// TestArabicCard sets a line of Arabic against the right edge of a card.
func TestArabicCard(t *testing.T) {
	fam := Family{Faces: []Face{{400, selawik(t)}}, Fallback: []*Font{fixture(t, "NotoSansArabic-subset.ttf")}}
	c := Card{Size: Square, Colors: dark, Lines: []Line{{Text: "مرحبا بلا", Size: 80, Weight: 400}}}
	img, err := c.Render(fam)
	if err != nil {
		t.Fatal(err)
	}
	ground, _ := Card{Size: Square, Colors: dark}.Render(fam)
	if box := inkBox(img, ground); box.Max.X < Square.W-Square.W*9/100-30 || box.Min.X < Square.W/2 {
		t.Errorf("the Arabic line spans %d to %d, which isn't at the right edge", box.Min.X, box.Max.X)
	}
}

// TestMixedDirections orders the runs of a line of Latin letters around
// Arabic as they show: the Arabic run between the two Latin ones, each in
// its own font.
func TestMixedDirections(t *testing.T) {
	latin, arabic := selawik(t), fixture(t, "NotoSansArabic-subset.ttf")
	ts := newTypesetter(Family{Faces: []Face{{400, latin}}, Fallback: []*Font{arabic}})
	s := ts.shape(latin, 400, "abc بلا def", false)
	var got []string
	for _, r := range s.runs {
		got = append(got, string([]rune("abc بلا def")[r.start:r.end]))
	}
	if strings.Join(got, "|") != "abc |بلا| def" {
		t.Errorf("the runs show as %q", got)
	}
	if s.runs[1].data != arabic.load() || s.runs[0].data != latin.load() || s.runs[1].level%2 != 1 {
		t.Error("the Arabic run isn't in the Arabic font, read right to left")
	}
	// A line that starts in Arabic reads right to left: its Latin run shows
	// first, at the left.
	s = ts.shape(latin, 400, "بلا abc", true)
	if first := string([]rune("بلا abc")[s.runs[0].start:s.runs[0].end]); strings.TrimSpace(first) != "abc" {
		t.Errorf("the line starting in Arabic shows %q first", first)
	}
}

// TestWrapKeepsClusters breaks a long word of conjuncts between them,
// never inside one.
func TestWrapKeepsClusters(t *testing.T) {
	deva := fixture(t, "NotoSansDevanagari-subset.ttf")
	ts := newTypesetter(Family{Faces: []Face{{400, deva}}})
	rows := ts.wrap(deva, 400, 60, strings.Repeat("क्ष", 30), false, 300, 50)
	if len(rows) < 2 {
		t.Fatalf("thirty conjuncts fit a row of 300 pixels: %+v", rows)
	}
	for _, r := range rows {
		if rs := []rune(r.text); len(rs)%3 != 0 || rs[0] != 'क' || r.width > 300.5 {
			t.Errorf("a row is %q, %.1f pixels wide", r.text, r.width)
		}
	}
}
