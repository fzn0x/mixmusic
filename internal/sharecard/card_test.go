package sharecard

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/image/font"
)

// family is Selawik, as the app sets its cards in it, read from the
// fonts the app bundles.
func family(t *testing.T) Family {
	t.Helper()
	var fam Family
	for _, f := range []struct {
		file   string
		weight int
	}{{"selawkl.ttf", 300}, {"selawk.ttf", 400}, {"selawksb.ttf", 600}, {"selawkb.ttf", 700}} {
		data, err := os.ReadFile(filepath.Join("..", "..", "fonts", f.file))
		if err != nil {
			t.Fatal(err)
		}
		font, err := ParseFont(data)
		if err != nil {
			t.Fatal(err)
		}
		fam.Faces = append(fam.Faces, Face{f.weight, font})
	}
	fam.Fallback = SystemFallback()
	return fam
}

var (
	dark  = Colors{Background: color.NRGBA{18, 18, 22, 255}, Text: color.NRGBA{245, 245, 247, 255}, Muted: color.NRGBA{160, 160, 170, 255}, Accent: color.NRGBA{0, 189, 227, 255}}
	light = Colors{Background: color.NRGBA{250, 247, 240, 255}, Text: color.NRGBA{28, 26, 24, 255}, Muted: color.NRGBA{110, 104, 98, 255}, Accent: color.NRGBA{200, 60, 20, 255}}
)

// cover is a square of a red top and a blue foot, as a cover to find.
func cover(side int) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, side, side))
	for y := range side {
		c := color.NRGBA{220, 30, 40, 255}
		if y >= side/2 {
			c = color.NRGBA{30, 60, 230, 255}
		}
		for x := range side {
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

// outDir is where the renders are saved: the folder SHARECARD_OUT names,
// to look at them, else a folder the test removes.
func outDir(t *testing.T) string {
	if d := os.Getenv("SHARECARD_OUT"); d != "" {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		return d
	}
	return t.TempDir()
}

// song is the card of a song: its cover, title and artist.
func song(size Size, colors Colors, title, artist string, withCover bool) Card {
	c := Card{
		Size:   size,
		Colors: colors,
		Lines: []Line{
			{Text: title, Size: 88, Weight: 700, MaxLines: 3},
			{Text: artist, Size: 52, Weight: 400, Ink: colors.Muted, MaxLines: 2, Gap: 20},
		},
		Foot: []Line{{Text: "MixMusic", Size: 34, Weight: 600, Ink: colors.Accent}},
	}
	if withCover {
		c.Cover = cover(600)
	}
	return c
}

// fallbackFor reports whether the family has a glyph for every letter of
// text.
func fallbackFor(fam Family, text string) bool {
	ts := &typesetter{fam: fam}
	for _, r := range text {
		if r == ' ' || r == '🎉' {
			continue
		}
		if ts.fontFor(fam.Faces[0].Font, r) == fam.Faces[0].Font {
			if i, err := fam.Faces[0].Font.GlyphIndex(&ts.buf, r); err != nil || i == 0 {
				return false
			}
		}
	}
	return true
}

// inkOutside finds a pixel of img outside the box of margin pixels from
// each edge that differs from the ground the card paints there.
func inkOutside(t *testing.T, img *image.NRGBA, ground *image.NRGBA, margin int) (image.Point, bool) {
	t.Helper()
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if x >= margin && x < b.Max.X-margin && y >= margin && y < b.Max.Y-margin {
				continue
			}
			if img.NRGBAAt(x, y) != ground.NRGBAAt(x, y) {
				return image.Pt(x, y), true
			}
		}
	}
	return image.Point{}, false
}

// ink counts the pixels of img that differ from ground.
func ink(img, ground *image.NRGBA) int {
	n := 0
	for i := 0; i < len(img.Pix); i += 4 {
		if img.Pix[i] != ground.Pix[i] || img.Pix[i+1] != ground.Pix[i+1] || img.Pix[i+2] != ground.Pix[i+2] {
			n++
		}
	}
	return n
}

func save(t *testing.T, dir, name string, img image.Image) {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".png"), buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestRender draws cards of both sizes in both themes with titles of every
// kind, and checks the ground, that the text stays inside the card, and
// that the text and the cover show.
func TestRender(t *testing.T) {
	fam := family(t)
	dir := outDir(t)
	long := strings.Repeat("A very long title that goes on and on ", 8)
	titles := []struct{ name, title, artist string }{
		{"plain", "Glass Harbor", "North Pier"},
		{"long", long, "An artist with a name long enough to need two lines of its own, as some do"},
		{"unbroken", strings.Repeat("W", 120), "x"},
		{"hebrew", "שיר האהבה הגדול של כל הזמנים", "אריק איינשטיין"},
		{"arabic", "أغنية عن الليل والمدينة", "فيروز"},
		{"mixed", "Hello שלום World", "Mixed"},
		{"cjk", "星空の下で歌う 星空の下で歌う 星空の下で歌う 星空の下で歌う 星空の下で歌う 星空の下で歌う 星空の下で歌う", "Aster Vale 星河 별빛"},
		{"emoji", "Party \U0001F389 time", "DJ"},
	}
	sizes := map[string]Size{"story": Story, "square": Square, "wide": Wide}
	themes := map[string]Colors{"dark": dark, "light": light}
	for sizeName, size := range sizes {
		for themeName, colors := range themes {
			for _, tc := range titles {
				for _, withCover := range []bool{true, false} {
					name := sizeName + "-" + themeName + "-" + tc.name
					if !withCover {
						name += "-nocover"
					}
					t.Run(name, func(t *testing.T) {
						card := song(size, colors, tc.title, tc.artist, withCover)
						img, err := card.Render(fam)
						if err != nil {
							t.Fatal(err)
						}
						if got := img.Bounds().Size(); got != image.Pt(size.W, size.H) {
							t.Fatalf("the card is %v, not %dx%d", got, size.W, size.H)
						}
						blank := Card{Size: size, Colors: colors}
						ground, err := blank.Render(fam)
						if err != nil {
							t.Fatal(err)
						}
						// The ground: plain at the foot, tinted at the top.
						for _, p := range []image.Point{{0, size.H - 1}, {size.W - 1, size.H - 1}, {size.W / 2, size.H - 1}} {
							if got := img.NRGBAAt(p.X, p.Y); got != colors.Background {
								t.Errorf("the ground at %v is %v, not %v", p, got, colors.Background)
							}
						}
						if top := img.NRGBAAt(0, 0); top == colors.Background {
							t.Errorf("the top has no tint of the accent: %v", top)
						}
						if top := ground.NRGBAAt(0, 0); top.A != 255 {
							t.Errorf("the ground is see-through: %v", top)
						}
						// The text and the cover stay inside the margin.
						margin := size.W * 9 / 100
						if size.H < size.W {
							margin = size.H * 9 / 100
						}
						if p, bad := inkOutside(t, img, ground, margin-2); bad {
							t.Errorf("ink at %v is outside the margin of %d", p, margin)
						}
						// Where the system has no font for a script, its letters
						// stay blank, and only the foot shows.
						if min := 2000; (ink(img, ground) < min) && (fallbackFor(fam, tc.title) || tc.name == "plain" || tc.name == "long") {
							t.Errorf("only %d pixels of text and cover", ink(img, ground))
						}
						if withCover {
							found := false
							for y := margin; y < size.H && !found; y += 7 {
								for x := margin; x < size.W-margin; x += 7 {
									if img.NRGBAAt(x, y) == (color.NRGBA{220, 30, 40, 255}) {
										found = true
										break
									}
								}
							}
							if !found {
								t.Error("the cover doesn't show")
							}
						}
						if withCover && sizeName == "wide" {
							// The foot's words don't run over the cover.
							side := min(int(float64(size.W-2*margin)*0.5), size.H-2*margin)
							if got := img.NRGBAAt(margin+40, margin+side-40); got != (color.NRGBA{30, 60, 230, 255}) {
								t.Errorf("the cover's corner is %v, ink ran over it", got)
							}
						}
						if sizeName != "wide" && themeName == "dark" && withCover {
							save(t, dir, name, img)
						}
						if tc.name == "plain" || tc.name == "long" {
							save(t, dir, name, img)
						}
					})
				}
			}
		}
	}
}

// TestLong keeps a long title to its lines: it ends with an ellipsis, and
// takes no more room than the lines it may have.
func TestLong(t *testing.T) {
	fam := family(t)
	long := song(Square, dark, strings.Repeat("Words ", 200), "x", false)
	short := song(Square, dark, "Words Words Words", "x", false)
	a, _ := long.Render(fam)
	b, _ := short.Render(fam)
	ga, _ := Card{Size: Square, Colors: dark}.Render(fam)
	// The long title covers no more than three lines of the short one's
	// size, which is under four times its ink, plus a little for the
	// ellipsis.
	if ink(a, ga) > 4*ink(b, ga) {
		t.Errorf("the long title has %d pixels of ink, the short one %d", ink(a, ga), ink(b, ga))
	}
}

// TestRightToLeft sets a line of Hebrew against the right edge, and a line
// of Latin letters against the left.
func TestRightToLeft(t *testing.T) {
	fam := family(t)
	if !fallbackFor(fam, "שלום") {
		t.Skip("the system has no font with Hebrew letters")
	}
	extent := func(text string) (min, max int) {
		c := Card{Size: Square, Colors: dark, Lines: []Line{{Text: text, Size: 80, Weight: 400}}}
		img, err := c.Render(fam)
		if err != nil {
			t.Fatal(err)
		}
		g, _ := Card{Size: Square, Colors: dark}.Render(fam)
		min, max = Square.W, 0
		for y := range Square.H {
			for x := range Square.W {
				if img.NRGBAAt(x, y) != g.NRGBAAt(x, y) {
					min, max = minInt(min, x), maxInt(max, x)
				}
			}
		}
		return min, max
	}
	lo, hi := extent("שלום")
	if hi < Square.W-Square.W*9/100-30 || lo < Square.W/2 {
		t.Errorf("Hebrew spans %d to %d, which isn't at the right edge", lo, hi)
	}
	lo, hi = extent("Shalom")
	if lo > Square.W*9/100+10 || hi > Square.W/2 {
		t.Errorf("Latin letters span %d to %d, which isn't at the left edge", lo, hi)
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// TestLyrics draws a card of lines of lyrics, with the cover at its foot.
func TestLyrics(t *testing.T) {
	fam := family(t)
	dir := outDir(t)
	for name, size := range map[string]Size{"story": Story, "square": Square} {
		c := Card{
			Size: size, Colors: light, Cover: cover(300), CoverAt: CoverFoot,
			Lines: []Line{
				{Text: "Lanterns on the water", Size: 76, Weight: 700, MaxLines: 3},
				{Text: "Counting the boats until morning comes in", Size: 76, Weight: 700, MaxLines: 3, Gap: 28},
				{Text: "The tide keeps the time", Size: 76, Weight: 700, MaxLines: 3, Gap: 28},
			},
			Foot: []Line{
				{Text: "Glass Harbor", Size: 44, Weight: 700},
				{Text: "North Pier", Size: 34, Weight: 400, Ink: light.Muted},
			},
		}
		img, err := c.Render(fam)
		if err != nil {
			t.Fatal(err)
		}
		save(t, dir, "lyrics-"+name, img)
		g, _ := Card{Size: size, Colors: light}.Render(fam)
		if p, bad := inkOutside(t, img, g, size.W*9/100-2); bad {
			t.Errorf("%s: ink at %v is outside the margin", name, p)
		}
		// The cover sits at the foot, at the left.
		x, y := size.W*9/100+20, size.H-size.W*9/100-20
		if got := img.NRGBAAt(x, y); got != (color.NRGBA{30, 60, 230, 255}) {
			t.Errorf("%s: the foot of the cover is %v", name, got)
		}
	}
}

// TestMark sets a mark at the bottom right of a lyrics card: its logo shows
// there, inside the margin, and the foot keeps clear of it, however long.
func TestMark(t *testing.T) {
	fam := family(t)
	dir := outDir(t)
	pink := color.NRGBA{230, 0, 126, 255}
	logo := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	draw.Draw(logo, logo.Bounds(), &image.Uniform{pink}, image.Point{}, draw.Src)
	mark := &Mark{Logo: logo, Name: Line{Text: "mixmusic", Size: 30, Weight: 700}}
	for name, size := range map[string]Size{"story": Story, "square": Square} {
		c := Card{
			Size: size, Colors: dark, Cover: cover(300), CoverAt: CoverFoot,
			Lines: []Line{{Text: "Lanterns on the water", Size: 76, Weight: 700, MaxLines: 3}},
			Foot: []Line{
				{Text: "Glass Harbor and the Long Tide of the Northern Lighthouse Keepers", Size: 44, Weight: 700, MaxLines: 2},
				{Text: "North Pier, the Saltwick Choir and the Morning Ferry Band", Size: 34, Weight: 400, Ink: dark.Muted},
			},
			Mark: mark,
		}
		img, err := c.Render(fam)
		if err != nil {
			t.Fatal(err)
		}
		save(t, dir, "mark-"+name, img)
		margin := size.W * 9 / 100
		g, _ := Card{Size: size, Colors: dark}.Render(fam)
		if p, bad := inkOutside(t, img, g, margin-2); bad {
			t.Errorf("%s: ink at %v is outside the margin", name, p)
		}
		ts := &typesetter{fam: fam, faces: map[faceKey]font.Face{}}
		set := ts.setMark(c, float64(size.W)/design, float64(size.W-2*margin))
		ts.close()
		if set.width <= set.logoW || set.logoW <= 0 {
			t.Fatalf("%s: the mark is set as %+v", name, set)
		}
		// The logo shows at the mark's left, at the bottom right of the card.
		left, top := size.W-margin-int(set.width), size.H-margin-int(set.height)
		if got := img.NRGBAAt(left+int(set.logoW)/2, top+int(set.height)/2); got != pink {
			t.Errorf("%s: the middle of the logo is %v", name, got)
		}
		// The mark's corner is as on a card of the mark alone: the foot
		// stays out of it.
		only, err := Card{Size: size, Colors: dark, Mark: mark}.Render(fam)
		if err != nil {
			t.Fatal(err)
		}
		if ink(only, g) == 0 {
			t.Fatalf("%s: the mark alone draws nothing", name)
		}
		for y := top - 8; y < size.H-margin; y++ {
			for x := left - 8; x < size.W-margin; x++ {
				if img.NRGBAAt(x, y) != only.NRGBAAt(x, y) {
					t.Fatalf("%s: the foot reaches the mark at %v", name, image.Pt(x, y))
				}
			}
		}
	}
}

// TestCentered centres a line across the card.
func TestCentered(t *testing.T) {
	fam := family(t)
	c := Card{Size: Square, Colors: dark, Align: AlignCenter, Lines: []Line{{Text: "Recap", Size: 100, Weight: 700}}}
	img, err := c.Render(fam)
	if err != nil {
		t.Fatal(err)
	}
	g, _ := Card{Size: Square, Colors: dark}.Render(fam)
	lo, hi := Square.W, 0
	for y := range Square.H {
		for x := range Square.W {
			if img.NRGBAAt(x, y) != g.NRGBAAt(x, y) {
				lo, hi = minInt(lo, x), maxInt(hi, x)
			}
		}
	}
	if d := (lo + hi) / 2; d < Square.W/2-6 || d > Square.W/2+6 {
		t.Errorf("the line spans %d to %d, its middle %d isn't the card's %d", lo, hi, d, Square.W/2)
	}
}

// TestErrors refuses a card of no size and a family without faces.
func TestErrors(t *testing.T) {
	if _, err := (Card{Colors: dark}).Render(family(t)); err == nil {
		t.Error("a card of no size rendered")
	}
	if _, err := (Card{Size: Square, Colors: dark}).Render(Family{}); err == nil {
		t.Error("a family without faces rendered")
	}
	if _, err := (Card{Size: Size{99999, 10}, Colors: dark}).Render(family(t)); err == nil {
		t.Error("a card too big rendered")
	}
}

// TestPNG writes a PNG that decodes to the card's size.
func TestPNG(t *testing.T) {
	var buf bytes.Buffer
	if err := song(Story, dark, "Glass Harbor", "North Pier", true).PNG(&buf, family(t)); err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 1080 || img.Bounds().Dy() != 1920 {
		t.Errorf("the PNG is %v", img.Bounds())
	}
}

// TestVisual reorders what reads right to left, and leaves the rest.
func TestVisual(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"Glass Harbor", "Glass Harbor"},
		{"שלום", "םולש"},
		{"abc שלום def", "abc םולש def"},
		{"", ""},
	} {
		if got := visual(tc.in); got != tc.want {
			t.Errorf("visual(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	if !isRTL("\"שלום") || isRTL("Hello שלום") || isRTL("123") || !isRTL("مرحبا") {
		t.Error("isRTL misjudges where a line starts")
	}
}

// TestShapeArabic puts the letters of a word in the shapes they take.
func TestShapeArabic(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		// Initial, medial and final: ba, ba, ba.
		{"ببب", "ﺑﺒﺐ"},
		// A right-joining letter ends the join: ba, alef, ba.
		{"باب", "ﺑﺎﺏ"},
		// Lam and alef make one letter.
		{"لا", "ﻻ"},
		{"بلا", "ﺑﻼ"},
		// Hello, and text that isn't Arabic stays.
		{"abc", "abc"},
		{"ب a", "ﺏ a"},
	} {
		if got := shapeArabic(tc.in); got != tc.want {
			t.Errorf("shapeArabic(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestWrapUnspaced breaks Chinese, Japanese and Korean text anywhere, and a
// word too long for a row where the row ends, within the width.
func TestWrapUnspaced(t *testing.T) {
	fam := family(t)
	ts := &typesetter{fam: fam, faces: map[faceKey]font.Face{}}
	defer ts.close()
	prim := fam.Faces[1].Font
	for _, text := range []string{strings.Repeat("星空の下で歌う", 30), strings.Repeat("W", 80), strings.Repeat("word ", 60)} {
		rows := ts.wrap(prim, 40, text, 600, 50)
		if len(rows) < 2 {
			t.Errorf("%.12q... fits one row of 600 pixels", text)
		}
		for _, r := range rows {
			if r.width > 600.5 {
				t.Errorf("a row of %q is %.1f pixels wide, over 600", r.text, r.width)
			}
		}
	}
}
