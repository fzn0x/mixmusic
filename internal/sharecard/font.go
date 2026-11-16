package sharecard

import (
	"bytes"
	"errors"
	"image/color"
	"io"
	"os"
	"sync"
	"time"
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
// weakly, and kept a minute after the last card that read it: then it
// goes, and the next card that needs it reads it again, so a font a card
// used once doesn't stay in memory.
type Font struct {
	cmap gotext.Cmap
	// colour is set when the font has glyphs in colour.
	colour bool
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
	// layers and palette are the glyphs of the first version of a COLR
	// table, layers of outlines in colours; sbix has the bitmaps of an
	// sbix table, and cbdt is set when go-text has bitmaps of a CBDT one.
	layers  *colrLayers
	palette []color.NRGBA
	sbix    *sbix
	cbdt    bool
}

// The tables go-text isn't given: a card reads the layers of COLR's first
// version itself, as the second version of Segoe UI Emoji's is gradients
// it doesn't draw; it reads an sbix bitmap when a glyph needs it, as the
// sbix table is most of Apple Color Emoji's 180 MB; and it doesn't draw
// SVG glyphs.
var (
	colrTag = ot.MustNewTag("COLR")
	sbixTag = ot.MustNewTag("sbix")
	svgTag  = ot.MustNewTag("SVG ")
)

// opener opens a font's bytes again, to read its bitmaps when it draws.
type opener func() (io.ReaderAt, io.Closer, error)

// The errors of reading a font.
var (
	errNoUnits = errors.New("sharecard: the font has no units per em")
	errNoFont  = errors.New("sharecard: the collection has no such font")
)

// ParseFont reads a TrueType or OpenType font.
func ParseFont(data []byte) (*Font, error) { return parsedFont(data, 0) }

// ParseCollectionFonts reads the fonts of a TrueType collection, a .ttc.
func ParseCollectionFonts(data []byte) ([]*Font, error) {
	lds, err := ot.NewLoaders(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	var out []*Font
	for i := range lds {
		f, err := parsedFont(data, i)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, nil
}

// parsedFont reads the whole of the index-th font of data.
func parsedFont(data []byte, index int) (*Font, error) {
	d, err := readFont(bytes.NewReader(data), index, func() (io.ReaderAt, io.Closer, error) {
		return bytes.NewReader(data), nil, nil
	})
	if err != nil {
		return nil, err
	}
	return &Font{cmap: d.font.Cmap, colour: d.layers != nil || d.sbix != nil || d.cbdt, data: d}, nil
}

// readFont reads the tables a card draws with of the index-th font of r,
// a font or a collection, which open opens again.
func readFont(r ot.Resource, index int, open opener) (*fontData, error) {
	lds, err := ot.NewLoaders(r)
	if err != nil {
		return nil, err
	}
	if index >= len(lds) {
		return nil, errNoFont
	}
	ld, use := lds[index], lds[index]
	if ld.HasTable(colrTag) || ld.HasTable(sbixTag) || ld.HasTable(svgTag) {
		var ts []ot.Table
		for _, tag := range ld.Tables() {
			if tag == colrTag || tag == sbixTag || tag == svgTag {
				continue
			}
			raw, err := ld.RawTable(tag)
			if err != nil {
				return nil, err
			}
			ts = append(ts, ot.Table{Tag: tag, Content: raw})
		}
		if use, err = ot.NewLoader(bytes.NewReader(ot.WriteTTF(ts))); err != nil {
			return nil, err
		}
	}
	f, err := gotext.NewFont(use)
	if err != nil {
		return nil, err
	}
	if f.Upem() == 0 {
		return nil, errNoUnits
	}
	d := &fontData{font: f, upem: float64(f.Upem()), variable: ld.HasTable(ot.MustNewTag("fvar")),
		cbdt: ld.HasTable(ot.MustNewTag("CBDT"))}
	readColour(d, ld, r, index, open)
	return d, nil
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
	lds, err := ot.NewLoaders(fh)
	if err != nil || len(lds) == 0 || !drawable(lds[0]) || tooBig(fh) {
		return nil
	}
	cmap, err := readCmap(lds[0])
	if err != nil {
		return nil
	}
	open := func() (io.ReaderAt, io.Closer, error) {
		fh, err := os.Open(file)
		return fh, fh, err
	}
	return &Font{cmap: cmap, colour: colourful(lds[0]), read: func() (*fontData, error) {
		fh, err := os.Open(file)
		if err != nil {
			return nil, err
		}
		defer fh.Close()
		return readFont(fh, 0, open)
	}}
}

// tooBig reports whether the tables of the first font of r that a card
// reads at once, all but its sbix bitmaps, take more than 64 MB.
func tooBig(r io.ReaderAt) bool {
	ranges, err := tableRanges(r, 0)
	if err != nil {
		return true
	}
	var n int64
	for tag, rg := range ranges {
		if tag != sbixTag {
			n += int64(rg.length)
		}
	}
	return n > 64<<20
}

// drawable reports whether the font of ld has glyphs a card can draw:
// TrueType, CFF or CFF2 outlines, or bitmaps in colour.
func drawable(ld *ot.Loader) bool {
	for _, tag := range []string{"glyf", "CFF ", "CFF2", "sbix", "CBDT"} {
		if ld.HasTable(ot.MustNewTag(tag)) {
			return true
		}
	}
	return false
}

// colourful reports whether the font of ld has glyphs in colour.
func colourful(ld *ot.Loader) bool {
	return ld.HasTable(colrTag) || ld.HasTable(sbixTag) || ld.HasTable(ot.MustNewTag("CBDT"))
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

// keepFor is how long the system fonts the last card read stay after it,
// so that cards drawn one after another, as the recap's two, read them
// once.
const keepFor = time.Minute

// kept holds the system fonts the last card read, until keepFor after it.
var kept struct {
	mu    sync.Mutex
	fonts []*fontData
	timer *time.Timer
}

// keep holds ds, the system fonts a card read, in place of the last card's,
// until keepFor after it.
func keep(ds []*fontData) {
	kept.mu.Lock()
	defer kept.mu.Unlock()
	kept.fonts = ds
	if kept.timer == nil {
		kept.timer = time.AfterFunc(keepFor, letGo)
	} else {
		kept.timer.Reset(keepFor)
	}
}

// letGo lets the fonts the last card read go.
func letGo() {
	kept.mu.Lock()
	defer kept.mu.Unlock()
	kept.fonts = nil
}

// covers reports whether the font has a glyph for r.
func (f *Font) covers(r rune) bool {
	g, ok := f.cmap.Lookup(r)
	return ok && g != 0
}
