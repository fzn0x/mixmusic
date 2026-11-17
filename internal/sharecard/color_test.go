package sharecard

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"
)

// inColour counts the pixels of img in a colour, not a grey, and the
// colours: the text's ink and the ground are greys.
func inColour(img *image.NRGBA) (pixels, colours int) {
	seen := map[color.NRGBA]bool{}
	for i := 0; i < len(img.Pix); i += 4 {
		r, g, b := img.Pix[i], img.Pix[i+1], img.Pix[i+2]
		if max(r, g, b)-min(r, g, b) > 80 {
			pixels++
			seen[color.NRGBA{r, g, b, 255}] = true
		}
	}
	return pixels, len(seen)
}

// systemFixture puts a font of testdata/fonts in a folder, as a system's
// font, and opens it so.
func systemFixture(t *testing.T, name string) *Font {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "fonts", name))
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatal(err)
	}
	fonts := loadFonts([]string{file})
	if len(fonts) != 1 || !fonts[0].colour {
		t.Fatalf("%s opens as %d fonts, not one in colour", name, len(fonts))
	}
	return fonts[0]
}

// TestEmojiBitmaps draws the party popper from a CBDT bitmap, as Noto
// Color Emoji has, and from an sbix one, as Apple Color Emoji has, read
// from the font's file when the card draws it. Both are the same bitmap at
// the same size: they draw as much colour.
func TestEmojiBitmaps(t *testing.T) {
	latin := selawik(t)
	cbdt := fixture(t, "NotoColorEmoji-CBDT-subset.ttf")
	img, ground, ts, s := drawn(t, latin, []*Font{cbdt}, "🎉")
	fromCBDT, _ := inColour(img)
	if fromCBDT < 1000 {
		t.Fatalf("the CBDT emoji draws %d pixels in colour", fromCBDT)
	}
	// It sits in the box its bitmap's metrics give, which its picture
	// fills but for a margin.
	r := s.runs[0]
	box := glyphBox(t, ts, r, r.glyphs[0].GlyphID, 0, 0, 120, 100, 300)
	if got := inkBox(img, ground); !got.In(box.Inset(-1)) || got.Dx() < box.Dx()*3/4 || got.Dy() < box.Dy()*3/4 {
		t.Errorf("the emoji is at %v, its metrics put it at %v", got, box)
	}
	sbix := systemFixture(t, "NotoColorEmoji-sbix.ttc")
	img, _, _, _ = drawn(t, latin, []*Font{sbix}, "🎉")
	fromSbix, _ := inColour(img)
	if fromSbix < fromCBDT*9/10 || fromSbix > fromCBDT*11/10 {
		t.Errorf("the sbix emoji draws %d pixels in colour, the CBDT one %d", fromSbix, fromCBDT)
	}
}

// TestColrLayers draws a letter of Coral Pixels, whose glyphs are layers
// in the colours of its palette, in those colours.
func TestColrLayers(t *testing.T) {
	coral := fixture(t, "CoralPixels-subset.ttf")
	d := coral.load()
	if d.layers == nil || len(d.palette) == 0 {
		t.Fatal("the font's COLR layers weren't read")
	}
	img, _, _, _ := drawn(t, coral, nil, "AB")
	if pixels, colours := inColour(img); pixels < 1000 || colours < 5 {
		t.Errorf("AB draws %d pixels in %d colours", pixels, colours)
	}
}

// TestEmojiPresentation takes a letter followed by U+FE0F, which asks for
// it as an emoji, from a font that has it in colour, though the primary
// font has it too.
func TestEmojiPresentation(t *testing.T) {
	latin, coral := selawik(t), fixture(t, "CoralPixels-subset.ttf")
	ts := newTypesetter(Family{Faces: []Face{{400, latin}}, Fallback: []*Font{coral}})
	if f := ts.fontFor(latin, []rune("A️"), 0, nil); f != coral {
		t.Error("A with U+FE0F isn't taken from the font in colour")
	}
	if f := ts.fontFor(latin, []rune("A"), 0, nil); f != latin {
		t.Error("A alone isn't taken from the primary font")
	}
}

// TestSystemEmoji draws an emoji in colour from the system's fonts, where
// the system has a font with emoji in colour: Segoe UI Emoji on Windows,
// Apple Color Emoji on macOS, Noto Color Emoji on Linux.
func TestSystemEmoji(t *testing.T) {
	fam := family(t)
	var found *Font
	for _, f := range fam.Fallback {
		if f.colour && f.covers('🎉') {
			found = f
			break
		}
	}
	if found == nil {
		t.Skip("the system has no font with emoji in colour")
	}
	c := Card{Size: Square, Colors: dark, Lines: []Line{{Text: "🎉", Size: 120, Weight: 700}}}
	img, err := c.Render(fam)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := inColour(img); got < 1000 {
		t.Errorf("the emoji draws %d pixels in colour", got)
	}
	if dir := os.Getenv("MIXMUSIC_CARD_DIR"); dir != "" {
		save(t, dir, "emoji", img)
	}
}

// TestTooBig reads a font's size without its sbix bitmaps: Apple Color
// Emoji's 180 MB are nearly all bitmaps, read one at a time.
func TestTooBig(t *testing.T) {
	dir := func(tables map[string]uint32) *bytes.Reader {
		var b bytes.Buffer
		be := binary.BigEndian
		b.Write([]byte{0, 1, 0, 0})
		_ = binary.Write(&b, be, uint16(len(tables)))
		b.Write(make([]byte, 6))
		for tag, n := range tables {
			b.WriteString(tag)
			_ = binary.Write(&b, be, [3]uint32{0, 1000, n})
		}
		return bytes.NewReader(b.Bytes())
	}
	if tooBig(dir(map[string]uint32{"cmap": 1 << 20, "morx": 4 << 20, "sbix": 180 << 20})) {
		t.Error("a font of 180 MB of sbix bitmaps is too big")
	}
	if !tooBig(dir(map[string]uint32{"cmap": 1 << 20, "glyf": 80 << 20})) {
		t.Error("a font of 80 MB of outlines isn't too big")
	}
}
