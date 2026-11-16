package sharecard

import (
	"bytes"
	"errors"
	"os"
	"sync"
	"weak"

	gotext "github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/font/opentype/tables"
)

// Font is a font a card's text can be set in, read with go-text, which
// shapes text by the font's rules and reads TrueType, CFF and CFF2
// outlines. A font parsed from its data, as the bundled faces are, is read
// whole. A font of the system is read in two steps: the letters it has up
// front, and the rest when a card needs one of them. The rest is held
// weakly: once no card uses it, it goes, and the next card that needs it
// reads it again, so a font a card used once doesn't stay in memory.
type Font struct {
	cmap gotext.Cmap
	// read reads the rest of a system font; it is nil for a parsed one.
	read func() (*fontData, error)
	// data is a parsed font's.
	data *fontData

	mu     sync.Mutex
	weak   weak.Pointer[fontData]
	broken bool
}

// fontData is what a font draws with.
type fontData struct {
	font *gotext.Font
	upem float64
	// variable is set when the font has axes, as a weight's, so that a
	// face is set to the weight of the line it draws.
	variable bool
}

// errNoUnits is the error of a font that sets no units per em.
var errNoUnits = errors.New("sharecard: the font has no units per em")

// ParseFont reads a TrueType or OpenType font.
func ParseFont(data []byte) (*Font, error) {
	ld, err := ot.NewLoader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return parsedFont(ld)
}

// ParseCollectionFonts reads the fonts of a TrueType collection, a .ttc.
func ParseCollectionFonts(data []byte) ([]*Font, error) {
	lds, err := ot.NewLoaders(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	var out []*Font
	for _, ld := range lds {
		f, err := parsedFont(ld)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, nil
}

// parsedFont reads the whole font of ld.
func parsedFont(ld *ot.Loader) (*Font, error) {
	d, err := readFont(ld)
	if err != nil {
		return nil, err
	}
	return &Font{cmap: d.font.Cmap, data: d}, nil
}

// readFont reads the tables of the font of ld that a card draws with.
func readFont(ld *ot.Loader) (*fontData, error) {
	f, err := gotext.NewFont(ld)
	if err != nil {
		return nil, err
	}
	if f.Upem() == 0 {
		return nil, errNoUnits
	}
	return &fontData{font: f, upem: float64(f.Upem()), variable: ld.HasTable(ot.MustNewTag("fvar"))}, nil
}

// openFont reads which letters the font of file has, the first font of a
// collection, and leaves the rest to read when a card needs it. It gives
// nil for a file it can't read or a font it couldn't draw.
func openFont(file string) *Font {
	fh, err := os.Open(file)
	if err != nil {
		return nil
	}
	defer fh.Close()
	if st, err := fh.Stat(); err != nil || st.Size() > 64<<20 {
		return nil
	}
	lds, err := ot.NewLoaders(fh)
	if err != nil || len(lds) == 0 || !drawable(lds[0]) {
		return nil
	}
	cmap, err := readCmap(lds[0])
	if err != nil {
		return nil
	}
	return &Font{cmap: cmap, read: func() (*fontData, error) {
		fh, err := os.Open(file)
		if err != nil {
			return nil, err
		}
		defer fh.Close()
		lds, err := ot.NewLoaders(fh)
		if err != nil {
			return nil, err
		}
		return readFont(lds[0])
	}}
}

// drawable reports whether the font of ld has outlines a card can draw:
// TrueType, CFF or CFF2 ones.
func drawable(ld *ot.Loader) bool {
	for _, tag := range []string{"glyf", "CFF ", "CFF2"} {
		if ld.HasTable(ot.MustNewTag(tag)) {
			return true
		}
	}
	return false
}

// readCmap reads which glyph draws each letter of the font of ld.
func readCmap(ld *ot.Loader) (gotext.Cmap, error) {
	raw, err := ld.RawTable(ot.MustNewTag("cmap"))
	if err != nil {
		return nil, err
	}
	t, _, err := tables.ParseCmap(raw)
	if err != nil {
		return nil, err
	}
	raw, _ = ld.RawTable(ot.MustNewTag("OS/2"))
	os2, _, _ := tables.ParseOs2(raw)
	cmap, _, err := gotext.ProcessCmap(t, os2.FontPage())
	return cmap, err
}

// load is what the font draws with, read again if it went, or nil when it
// can't be read.
func (f *Font) load() *fontData {
	if f.read == nil {
		return f.data
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if d := f.weak.Value(); d != nil {
		return d
	}
	if f.broken {
		return nil
	}
	d, err := f.read()
	if err != nil {
		f.broken = true
		return nil
	}
	f.weak = weak.Make(d)
	return d
}

// covers reports whether the font has a glyph for r.
func (f *Font) covers(r rune) bool {
	g, ok := f.cmap.Lookup(r)
	return ok && g != 0
}
