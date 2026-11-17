package sharecard

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"math"
	"sort"

	gotext "github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/font/opentype/tables"
	xdraw "golang.org/x/image/draw"
)

// Emoji and other glyphs in colour come from three kinds of tables: COLR's
// layers, outlines filled in the colours of a CPAL palette, as Segoe UI
// Emoji has; sbix bitmaps, as Apple Color Emoji has; and CBDT bitmaps, as
// Noto Color Emoji has. The second version of COLR, whose glyphs are
// gradients, isn't drawn: a card draws the layers of the first version,
// which Segoe UI Emoji keeps for what doesn't draw the second.

// paletteText is the palette entry that stands for the colour of the text.
const paletteText = 0xFFFF

// colrLayers are the records of the first version of a COLR table: for
// each glyph in colour, by its glyph, its layers, and for each layer, the
// glyph of its outline and its palette entry.
type colrLayers struct {
	base   []byte // 6 bytes each: glyph, first layer, number of layers
	layers []byte // 4 bytes each: glyph, palette entry
}

// parseCOLR reads the records of COLR's first version from raw, or nil
// when it has none.
func parseCOLR(raw []byte) *colrLayers {
	if len(raw) < 14 {
		return nil
	}
	be := binary.BigEndian
	nBase, baseAt := int(be.Uint16(raw[2:])), int64(be.Uint32(raw[4:]))
	layersAt, nLayers := int64(be.Uint32(raw[8:])), int(be.Uint16(raw[12:]))
	if nBase == 0 || baseAt+int64(6*nBase) > int64(len(raw)) || layersAt+int64(4*nLayers) > int64(len(raw)) {
		return nil
	}
	// The records are copied out of the table, which may hold megabytes
	// of the second version's.
	return &colrLayers{
		base:   bytes.Clone(raw[baseAt : baseAt+int64(6*nBase)]),
		layers: bytes.Clone(raw[layersAt : layersAt+int64(4*nLayers)]),
	}
}

// of is the first layer of glyph and the number of its layers, or false
// when it has none.
func (c *colrLayers) of(glyph gotext.GID) (first, n int, ok bool) {
	be := binary.BigEndian
	count := len(c.base) / 6
	i := sort.Search(count, func(i int) bool { return gotext.GID(be.Uint16(c.base[6*i:])) >= glyph })
	if i == count || gotext.GID(be.Uint16(c.base[6*i:])) != glyph {
		return 0, 0, false
	}
	first, n = int(be.Uint16(c.base[6*i+2:])), int(be.Uint16(c.base[6*i+4:]))
	return first, n, first+n <= len(c.layers)/4
}

// layer is the glyph and the palette entry of layer i.
func (c *colrLayers) layer(i int) (gotext.GID, uint16) {
	be := binary.BigEndian
	return gotext.GID(be.Uint16(c.layers[4*i:])), be.Uint16(c.layers[4*i+2:])
}

// sbix is an sbix table, read where it lies when a glyph needs a bitmap.
type sbix struct {
	open    opener
	at      int64 // where the table starts
	length  int64
	glyphs  int
	strikes []sbixStrike
}

// sbixStrike is one size of an sbix table's bitmaps: their pixels per em,
// and where the strike starts in the table.
type sbixStrike struct {
	ppem uint16
	at   int64
}

// readColour reads the colour tables of ld, the index-th font of r, into d:
// COLR's layers with the first palette of CPAL, and where the sbix bitmaps
// are, which open opens to read.
func readColour(d *fontData, ld *ot.Loader, r io.ReaderAt, index int, open opener) {
	if raw, err := ld.RawTable(colrTag); err == nil {
		if layers := parseCOLR(raw); layers != nil {
			if raw, err := ld.RawTable(ot.MustNewTag("CPAL")); err == nil {
				if cpal, _, err := tables.ParseCPAL(raw); err == nil && len(cpal.ColorRecordIndices) > 0 {
					start := int(cpal.ColorRecordIndices[0])
					end := min(start+int(cpal.NumPaletteEntries), len(cpal.ColorRecordsArray))
					for _, c := range cpal.ColorRecordsArray[min(start, end):end] {
						d.palette = append(d.palette, color.NRGBA{c.Red, c.Green, c.Blue, c.Alpha})
					}
					d.layers = layers
				}
			}
		}
	}
	if ld.HasTable(sbixTag) {
		d.sbix = readSbix(ld, r, index, open)
	}
}

// readSbix reads where the strikes of the sbix table of ld are, the
// index-th font of r.
func readSbix(ld *ot.Loader, r io.ReaderAt, index int, open opener) *sbix {
	ranges, err := tableRanges(r, index)
	if err != nil {
		return nil
	}
	rg := ranges[sbixTag]
	raw, err := ld.RawTable(ot.MustNewTag("maxp"))
	if err != nil || len(raw) < 6 || rg.length < 8 {
		return nil
	}
	s := &sbix{open: open, at: int64(rg.offset), length: int64(rg.length), glyphs: int(binary.BigEndian.Uint16(raw[4:]))}
	var head [8]byte
	if _, err := r.ReadAt(head[:], s.at); err != nil {
		return nil
	}
	n := int64(binary.BigEndian.Uint32(head[4:]))
	if n == 0 || 8+4*n > s.length {
		return nil
	}
	offsets := make([]byte, 4*n)
	if _, err := r.ReadAt(offsets, s.at+8); err != nil {
		return nil
	}
	for i := range n {
		at := int64(binary.BigEndian.Uint32(offsets[4*i:]))
		var ppem [2]byte
		if at+4 > s.length {
			continue
		}
		if _, err := r.ReadAt(ppem[:], s.at+at); err != nil {
			return nil
		}
		s.strikes = append(s.strikes, sbixStrike{ppem: binary.BigEndian.Uint16(ppem[:]), at: at})
	}
	sort.Slice(s.strikes, func(i, j int) bool { return s.strikes[i].ppem < s.strikes[j].ppem })
	if len(s.strikes) == 0 {
		return nil
	}
	return s
}

// sbixBitmap is the bitmap of an sbix glyph: its PNG, its strike's pixels
// per em, and where its corner at the bottom left sits from the glyph's
// origin, in the strike's pixels.
type sbixBitmap struct {
	png    []byte
	ppem   uint16
	ox, oy int16
}

// errSbix is the error of an sbix glyph that has no bitmap to draw.
var errSbix = errors.New("sharecard: no sbix bitmap")

// bitmap reads the bitmap of glyph from the smallest strike of at least px
// pixels per em, or the biggest.
func (s *sbix) bitmap(glyph gotext.GID, px float64) (sbixBitmap, error) {
	st := s.strikes[len(s.strikes)-1]
	for _, c := range s.strikes {
		if float64(c.ppem) >= px {
			st = c
			break
		}
	}
	if int(glyph) >= s.glyphs {
		return sbixBitmap{}, errSbix
	}
	r, closer, err := s.open()
	if err != nil {
		return sbixBitmap{}, err
	}
	if closer != nil {
		defer closer.Close()
	}
	// A 'dupe' glyph names the glyph whose bitmap it has.
	for range 2 {
		var offs [8]byte
		if _, err := r.ReadAt(offs[:], s.at+st.at+4+4*int64(glyph)); err != nil {
			return sbixBitmap{}, err
		}
		from, to := int64(binary.BigEndian.Uint32(offs[:])), int64(binary.BigEndian.Uint32(offs[4:]))
		if to-from < 8 || to-from > 4<<20 || st.at+to > s.length {
			return sbixBitmap{}, errSbix
		}
		data := make([]byte, to-from)
		if _, err := r.ReadAt(data, s.at+st.at+from); err != nil {
			return sbixBitmap{}, err
		}
		b := sbixBitmap{png: data[8:], ppem: st.ppem, ox: int16(binary.BigEndian.Uint16(data)), oy: int16(binary.BigEndian.Uint16(data[2:]))}
		switch string(data[4:8]) {
		case "png ":
			return b, nil
		case "dupe":
			if len(data) < 10 {
				return sbixBitmap{}, errSbix
			}
			glyph = gotext.GID(binary.BigEndian.Uint16(data[8:]))
			continue
		}
		break
	}
	return sbixBitmap{}, errSbix
}

// tableRange is where a table lies in a font's file.
type tableRange struct{ offset, length uint32 }

// tableRanges reads where the tables of the index-th font of r, a font or a
// collection, lie.
func tableRanges(r io.ReaderAt, index int) (map[ot.Tag]tableRange, error) {
	be := binary.BigEndian
	var head [12]byte
	if _, err := r.ReadAt(head[:], 0); err != nil {
		return nil, err
	}
	var at int64
	if string(head[:4]) == "ttcf" {
		if uint32(index) >= be.Uint32(head[8:]) {
			return nil, errNoFont
		}
		var off [4]byte
		if _, err := r.ReadAt(off[:], 12+4*int64(index)); err != nil {
			return nil, err
		}
		at = int64(be.Uint32(off[:]))
		if _, err := r.ReadAt(head[:], at); err != nil {
			return nil, err
		}
	} else if index != 0 {
		return nil, errNoFont
	}
	n := int(be.Uint16(head[4:]))
	dir := make([]byte, 16*n)
	if _, err := r.ReadAt(dir, at+12); err != nil {
		return nil, err
	}
	out := make(map[ot.Tag]tableRange, n)
	for i := range n {
		rec := dir[16*i:]
		out[ot.Tag(be.Uint32(rec))] = tableRange{be.Uint32(rec[8:]), be.Uint32(rec[12:])}
	}
	return out, nil
}

// drawColour draws glyph gid of d in colour, when it has colour: its COLR
// layers, its sbix bitmap or its CBDT bitmap. It reports whether it drew.
func (s *shaper) drawColour(img *image.NRGBA, d *fontData, fc *face, gid gotext.GID, scale, x, y float64, ink color.NRGBA) bool {
	if d.layers != nil {
		if first, n, ok := d.layers.of(gid); ok {
			for i := first; i < first+n; i++ {
				g, entry := d.layers.layer(i)
				c := ink
				if entry != paletteText {
					if int(entry) >= len(d.palette) {
						continue
					}
					c = d.palette[entry]
				}
				if out, ok := fc.GlyphDataOutline(g); ok {
					s.fill(img, out.Segments, scale, x, y, c)
				}
			}
			return true
		}
	}
	if d.sbix != nil {
		if b, err := d.sbix.bitmap(gid, scale*d.upem); err == nil {
			if src := s.decode(d, gid, b.ppem, b.png); src != nil {
				k := scale * d.upem / float64(b.ppem)
				w, h := float64(src.Bounds().Dx())*k, float64(src.Bounds().Dy())*k
				left, bottom := x+float64(b.ox)*k, y-float64(b.oy)*k
				s.drawBitmap(img, src, left, bottom-h, w, h)
				return true
			}
		}
	}
	if d.cbdt {
		if b, ok := fc.GlyphDataBitmap(gid); ok && b.Format == gotext.PNG {
			if e, ok := fc.GlyphExtents(gid); ok {
				if src := s.decode(d, gid, 0, b.Data); src != nil {
					s.drawBitmap(img, src, x+float64(e.XBearing)*scale, y-float64(e.YBearing)*scale,
						float64(e.Width)*scale, -float64(e.Height)*scale)
					return true
				}
			}
		}
	}
	return false
}

// bitmapKey is a bitmap a shaper decoded: its font, glyph and strike.
type bitmapKey struct {
	d    *fontData
	gid  gotext.GID
	ppem uint16
}

// decode decodes the PNG of a glyph's bitmap, once for the card.
func (s *shaper) decode(d *fontData, gid gotext.GID, ppem uint16, data []byte) image.Image {
	k := bitmapKey{d, gid, ppem}
	if img, ok := s.bitmaps[k]; ok {
		return img
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil || img.Bounds().Empty() {
		img = nil
	}
	if s.bitmaps == nil {
		s.bitmaps = map[bitmapKey]image.Image{}
	}
	s.bitmaps[k] = img
	return img
}

// drawBitmap draws src into the box at left and top, w by h pixels.
func (s *shaper) drawBitmap(img *image.NRGBA, src image.Image, left, top, w, h float64) {
	at := image.Rect(int(math.Round(left)), int(math.Round(top)), int(math.Round(left+w)), int(math.Round(top+h)))
	if at.Empty() || at.Dx()*at.Dy() > maxGlyphArea || !at.Overlaps(img.Bounds()) {
		return
	}
	xdraw.CatmullRom.Scale(img, at, src, src.Bounds(), draw.Over, nil)
}
