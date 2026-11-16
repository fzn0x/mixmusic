// Package sharecard draws the image cards MixMusic shares: a cover, a few
// lines of text and the theme's colours, on a canvas of a size of its own.
// It draws offscreen, with image/draw and the fonts it is given, so it
// needs no window and no system fonts, and renders the same everywhere.
// The yearly recap draws its story and square cards with it too.
package sharecard

import (
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"math"
	"strings"
	"unicode"

	xdraw "golang.org/x/image/draw"
)

// Size is the size of a card, in pixels.
type Size struct{ W, H int }

// The sizes of the cards: a story fills a phone's screen, a square suits a
// chat or a feed, and Wide suits a link's preview.
var (
	Story  = Size{1080, 1920}
	Square = Size{1080, 1080}
	Wide   = Size{1200, 630}
)

// Face is one weight of a family: a font and the weight it has, 400 being
// regular and 700 bold.
type Face struct {
	Weight int
	Font   *Font
}

// Family is the faces a card's text is set in. A line takes the face
// nearest the weight it asks for; a letter the face lacks, a Chinese
// character say, comes from the first of Fallback that has it, and when
// none does, from the family's own .notdef glyph, which is blank in
// Selawik, so the letter leaves a gap.
type Family struct {
	Faces    []Face
	Fallback []*Font
}

// Colors are the colours of a card, which the app takes from its theme.
type Colors struct {
	// Background is the card's ground, which the Accent tints at the top.
	Background color.NRGBA
	// Text is the colour of the first lines, Muted the colour of the
	// lines that set Muted, and Accent the colour of the tint and of the
	// lines that set Accent.
	Text, Muted, Accent color.NRGBA
}

// Align is where a line sits across the card.
type Align int

const (
	// AlignAuto starts a line where its language does: left, or right for
	// a line that reads right to left.
	AlignAuto Align = iota
	AlignLeft
	AlignCenter
)

// Line is a line of text, which wraps over as many lines as MaxLines
// allows and ends with an ellipsis if it doesn't fit.
type Line struct {
	Text string
	// Size is the height of the text in pixels, for a card 1080 pixels
	// wide: a card of another width scales it.
	Size   float64
	Weight int
	// Ink is the colour; the zero colour is Colors.Text.
	Ink color.NRGBA
	// MaxLines is how many lines the text may take; 0 is one.
	MaxLines int
	// Gap is the space above the line, in the same pixels as Size.
	Gap float64
}

// CoverAt is where the cover sits.
type CoverAt int

const (
	// CoverTop sets the cover big, above the lines.
	CoverTop CoverAt = iota
	// CoverFoot sets it small, before the foot's lines.
	CoverFoot
)

// Card is a card to draw: Lines run from the top, below the cover when it
// is on top, and Foot sits at the bottom of the card.
type Card struct {
	Size   Size
	Colors Colors
	// Cover is the cover to show, cut to a square; nil shows none.
	Cover   image.Image
	CoverAt CoverAt
	Lines   []Line
	Foot    []Line
	Align   Align
	// Mark is set at the bottom right; nil sets none.
	Mark *Mark
}

// Mark is a mark set at the bottom right of a card, as an app's: its logo,
// then its name, on one line. The foot keeps clear of it.
type Mark struct {
	// Logo is the logo, a little higher than the name's letters, keeping its
	// shape; nil shows none.
	Logo image.Image
	// Name is the name: its Text, Size, Weight and Ink, on one line.
	Name Line
}

// setMark is a Mark set at a scale: the name set, and the logo's size, the
// gap between them and the whole mark's size, in pixels.
type setMark struct {
	name              []setLine
	logoW, logoH, gap float64
	width, height     float64
}

// Rendering is the part of a card's drawing that went wrong.
var (
	errSize = errors.New("sharecard: the card has no size")
	errFont = errors.New("sharecard: the family has no faces")
)

const (
	// design is the width the sizes of lines are for.
	design = 1080.0
	// minShrink is how far the lines shrink to fit the card, before they
	// lose lines.
	minShrink = 0.55
)

// Render draws the card.
func (c Card) Render(fam Family) (*image.NRGBA, error) {
	if c.Size.W <= 0 || c.Size.H <= 0 || c.Size.W > 8192 || c.Size.H > 8192 {
		return nil, errSize
	}
	if len(fam.Faces) == 0 {
		return nil, errFont
	}
	img := image.NewNRGBA(image.Rect(0, 0, c.Size.W, c.Size.H))
	paintGround(img, c.Colors)
	w, h := float64(c.Size.W), float64(c.Size.H)
	scale := w / design
	margin := math.Round(w * 0.09)
	if c.Size.H < c.Size.W {
		margin = math.Round(h * 0.09)
	}
	inner := w - 2*margin
	r := newTypesetter(fam)

	// A card about as wide as it is tall puts the cover beside its lines; a
	// taller one puts it above them, across their width.
	wide := c.Size.W*5 >= c.Size.H*4
	top, textLeft, textW := margin, margin, inner
	// The mark takes the bottom right, which the foot leaves it.
	mark := r.setMark(c, scale, inner)
	clear := 0.0
	if mark.width > 0 {
		clear = mark.width + 32*scale
	}
	// The foot keeps its place at the bottom, so it is set first.
	footLeft, footW := margin, inner-clear
	footLines := r.set(c, c.Foot, scale, footW, false)
	footH := blockHeight(footLines)
	footTop := h - margin - footH
	switch {
	case c.Cover == nil:
	case c.CoverAt == CoverFoot:
		side := math.Min(math.Max(footH, h*0.1), h*0.16)
		drawCover(img, c.Cover, image.Rect(int(margin), int(h-margin-side), int(margin+side), int(h-margin)), scale)
		footLeft = margin + side + 28*scale
		footW = w - margin - footLeft - clear
		footLines = r.set(c, c.Foot, scale, footW, false)
		footH = blockHeight(footLines)
		footTop = h - margin - side + (side-footH)/2
		if footH > side {
			footTop = h - margin - footH
		}
	case wide:
		// A wide card puts the cover beside its lines.
		side := math.Round(math.Min(inner*0.46, h-2*margin))
		drawCover(img, c.Cover, image.Rect(int(margin), int(top), int(margin+side), int(top+side)), scale)
		textLeft = margin + side + math.Round(margin*0.45)
		textW = w - margin - textLeft
		footLeft, footW = textLeft, textW-clear
		footLines = r.set(c, c.Foot, scale, footW, false)
		footTop = h - margin - blockHeight(footLines)
	default:
		side := inner
		drawCover(img, c.Cover, image.Rect(int(margin), int(top), int(margin+side), int(top+side)), scale)
		top += side + 56*scale
	}
	room := h - margin - top
	switch {
	case len(footLines) > 0:
		room = footTop - top - 32*scale
	case mark.height > 0:
		room = h - margin - mark.height - top - 32*scale
	}
	// The lines shrink to fit the room, then lose lines.
	var lines []setLine
	for shrink := 1.0; ; shrink -= 0.05 {
		lines = r.set(c, c.Lines, scale*shrink, textW, true)
		if blockHeight(lines) <= room || shrink-0.05 < minShrink {
			break
		}
	}
	lines = r.fitHeight(lines, room)
	r.draw(img, lines, textLeft, top, textW, c.Align)
	r.draw(img, footLines, footLeft, footTop, footW, c.Align)
	r.drawMark(img, c.Mark, mark, w-margin, h-margin)
	return img, nil
}

// setMark sets the card's mark at scale, no wider than width.
func (t *typesetter) setMark(c Card, scale, width float64) setMark {
	if c.Mark == nil {
		return setMark{}
	}
	var s setMark
	name := c.Mark.Name
	name.MaxLines = 1
	s.name = t.set(c, []Line{name}, scale, width, false)
	px := name.Size * scale
	if len(s.name) > 0 {
		px = s.name[0].px
		s.width = s.name[0].rows[0].width
		s.height = s.name[0].height
	}
	if logo := c.Mark.Logo; logo != nil && px > 0 && logo.Bounds().Dy() > 0 {
		b := logo.Bounds()
		s.logoH = math.Round(px * 1.3)
		s.logoW = math.Round(s.logoH * float64(b.Dx()) / float64(b.Dy()))
		if s.width > 0 {
			s.gap = math.Round(px * 0.4)
		}
		s.width += s.logoW + s.gap
		s.height = math.Max(s.height, s.logoH)
	}
	return s
}

// drawMark draws the mark m, set as s, ending at right and bottom, its logo
// and name centred on one line.
func (t *typesetter) drawMark(img *image.NRGBA, m *Mark, s setMark, right, bottom float64) {
	if m == nil || s.width <= 0 {
		return
	}
	x := right - s.width
	if s.logoW > 0 {
		top := bottom - (s.height+s.logoH)/2
		at := image.Rect(int(math.Round(x)), int(math.Round(top)), int(math.Round(x+s.logoW)), int(math.Round(top+s.logoH)))
		logo := image.NewNRGBA(image.Rect(0, 0, at.Dx(), at.Dy()))
		xdraw.CatmullRom.Scale(logo, logo.Bounds(), m.Logo, m.Logo.Bounds(), xdraw.Src, nil)
		draw.Draw(img, at, logo, image.Point{}, draw.Over)
		x += s.logoW + s.gap
	}
	if len(s.name) > 0 {
		t.draw(img, s.name, x, bottom-(s.height+s.name[0].height)/2, right-x, AlignLeft)
	}
}

// PNG draws the card and writes it as a PNG.
func (c Card) PNG(w io.Writer, fam Family) error {
	img, err := c.Render(fam)
	if err != nil {
		return err
	}
	return png.Encode(w, img)
}

// paintGround fills img with the background, tinted by the accent at the
// top and fading to the plain background by the middle of the card.
func paintGround(img *image.NRGBA, col Colors) {
	b := img.Bounds()
	draw.Draw(img, b, image.NewUniform(col.Background), image.Point{}, draw.Src)
	reach := b.Dy() * 6 / 10
	for y := 0; y < reach; y++ {
		t := 1 - float64(y)/float64(reach)
		// Eased, so the tint has no visible edge.
		a := 0.20 * t * t
		c := mix(col.Background, col.Accent, a)
		row := image.Rect(0, y, b.Dx(), y+1)
		draw.Draw(img, row, image.NewUniform(c), image.Point{}, draw.Src)
	}
}

// mix is a, a part of the way from from to to.
func mix(from, to color.NRGBA, a float64) color.NRGBA {
	l := func(x, y uint8) uint8 { return uint8(math.Round(float64(x)*(1-a) + float64(y)*a)) }
	return color.NRGBA{l(from.R, to.R), l(from.G, to.G), l(from.B, to.B), 255}
}

// drawCover draws the centre square of cover into at, with rounded
// corners.
func drawCover(dst *image.NRGBA, cover image.Image, at image.Rectangle, scale float64) {
	side := min(at.Dx(), at.Dy())
	if side <= 0 {
		return
	}
	at = image.Rect(at.Min.X, at.Min.Y, at.Min.X+side, at.Min.Y+side)
	b := cover.Bounds()
	s := min(b.Dx(), b.Dy())
	if s <= 0 {
		return
	}
	src := image.Rect(b.Min.X+(b.Dx()-s)/2, b.Min.Y+(b.Dy()-s)/2, b.Min.X+(b.Dx()-s)/2+s, b.Min.Y+(b.Dy()-s)/2+s)
	sq := image.NewNRGBA(image.Rect(0, 0, side, side))
	xdraw.CatmullRom.Scale(sq, sq.Bounds(), cover, src, xdraw.Src, nil)
	radius := math.Min(24*scale, float64(side)/2)
	mask := roundedMask(side, radius)
	draw.DrawMask(dst, at, sq, image.Point{}, mask, image.Point{}, draw.Over)
}

// roundedMask is a square mask of side pixels with corners of radius, whose
// edge is antialiased.
func roundedMask(side int, radius float64) *image.Alpha {
	m := image.NewAlpha(image.Rect(0, 0, side, side))
	for y := range side {
		for x := range side {
			cx := math.Min(math.Max(float64(x)+0.5, radius), float64(side)-radius)
			cy := math.Min(math.Max(float64(y)+0.5, radius), float64(side)-radius)
			d := math.Hypot(float64(x)+0.5-cx, float64(y)+0.5-cy)
			a := math.Min(math.Max(radius-d+0.5, 0), 1)
			m.Pix[y*m.Stride+x] = uint8(math.Round(a * 255))
		}
	}
	return m
}

// setLine is a line of text set: its pieces, each a row of text, with its
// face, ink and height.
type setLine struct {
	rows []row
	ink  color.NRGBA
	// px is the size of the text, height the room a row takes, and gap the
	// space above the line.
	px, height, gap float64
	weight          int
	rtl             bool
}

// row is one row of a wrapped line, in the order of its text.
type row struct {
	text  string
	width float64
}

// typesetter measures and draws text in a family, keeping the texts it
// shaped and the fonts it read until the card is drawn.
type typesetter struct {
	fam    Family
	sh     *shaper
	shapes map[shapeKey]*shaped
	fonts  map[*Font]*fontData
}

func newTypesetter(fam Family) *typesetter {
	return &typesetter{fam: fam, sh: newShaper(), shapes: map[shapeKey]*shaped{}, fonts: map[*Font]*fontData{}}
}

// data is what f draws with, kept for the card, or nil when it can't be
// read.
func (t *typesetter) data(f *Font) *fontData {
	d, ok := t.fonts[f]
	if !ok {
		d = f.load()
		t.fonts[f] = d
	}
	return d
}

// has reports whether f has a glyph for r and can draw it.
func (t *typesetter) has(f *Font, r rune) bool { return f.covers(r) && t.data(f) != nil }

// primary is the face of the family nearest weight.
func (t *typesetter) primary(weight int) *Font {
	best := t.fam.Faces[0]
	for _, f := range t.fam.Faces[1:] {
		if abs(f.Weight-weight) < abs(best.Weight-weight) {
			best = f
		}
	}
	return best.Font
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// advance is the width of text at px, in pixels, shaped as a row of a
// line that reads right to left when rtl is set.
func (t *typesetter) advance(prim *Font, weight int, px float64, text string, rtl bool) float64 {
	return t.shape(prim, weight, text, rtl).width(px)
}

// set lays out lines at scale, wrapped to width pixels. With gaps set, the
// lines keep the space above them that they ask for.
func (t *typesetter) set(c Card, lines []Line, scale, width float64, gaps bool) []setLine {
	var out []setLine
	for _, l := range lines {
		text := strings.TrimSpace(strings.Map(func(r rune) rune {
			if r == '\n' || r == '\r' || r == '\t' {
				return ' '
			}
			return r
		}, l.Text))
		if text == "" || l.Size <= 0 {
			continue
		}
		px := math.Max(l.Size*scale, 6)
		weight := l.Weight
		if weight == 0 {
			weight = 400
		}
		ink := l.Ink
		if ink == (color.NRGBA{}) {
			ink = c.Colors.Text
		}
		maxLines := max(l.MaxLines, 1)
		prim := t.primary(weight)
		rtl := isRTL(text)
		rows := t.wrap(prim, weight, px, text, rtl, width, maxLines)
		sl := setLine{rows: rows, ink: ink, weight: weight, px: px, height: px * 1.25, rtl: rtl}
		if len(out) > 0 {
			sl.gap = l.Gap * scale
		}
		out = append(out, sl)
	}
	return out
}

// wrap breaks text into at most maxLines rows no wider than width at px,
// at spaces, or between any two clusters of a word too long, and ends the
// last row with an ellipsis if text goes on. A cluster, a letter with its
// marks or a conjunct, never breaks.
func (t *typesetter) wrap(prim *Font, weight int, px float64, text string, rtl bool, width float64, maxLines int) []row {
	var rows []row
	m := t.measure(prim, weight, text, rtl)
	rs := m.rs
	i := 0
	for i < len(rs) && len(rows) < maxLines {
		for i < len(rs) && rs[i] == ' ' && len(rows) > 0 {
			i++
		}
		if i >= len(rs) {
			break
		}
		last := len(rows) == maxLines-1
		end := t.fit(m, px, i, width, last)
		line := func(end int) string {
			s := strings.TrimRight(string(rs[i:end]), " ")
			if last && end < len(rs) {
				s += "…"
			}
			return s
		}
		// A row shaped alone may come out wider than its clusters measured
		// in the whole text, as the ellipsis does, so it gives back a
		// cluster at a time until it fits.
		cut := end
		s := line(cut)
		w := t.advance(prim, weight, px, s, rtl)
		for w > width+0.5 && cut > i && (last && end < len(rs) || cut > m.cluster(i)) {
			cut = m.back(cut, i)
			s = line(cut)
			w = t.advance(prim, weight, px, s, rtl)
		}
		if cut > i {
			end = cut
		}
		rows = append(rows, row{text: s, width: w})
		i = end
	}
	return rows
}

// fit is where a row that starts at from ends: after the last space that
// fits width, or after as many clusters as fit when no space does. With
// last set, the row takes as many clusters as fit.
func (t *typesetter) fit(m measure, px float64, from int, width float64, last bool) int {
	rs := m.rs
	var w float64
	lastSpace, n := -1, from
	for n < len(rs) {
		e := m.cluster(n)
		var a float64
		for _, adv := range m.adv[n:e] {
			a += adv * px
		}
		if w+a > width && n > from {
			break
		}
		w += a
		if rs[n] == ' ' {
			lastSpace = n
		}
		n = e
	}
	if n == len(rs) {
		return n
	}
	// Break at the last space, unless the letters are of a script written
	// without them, which breaks anywhere, or the row is the last, which
	// takes all that fits.
	if lastSpace > from && !last && !unspaced(rs[n]) && !unspaced(rs[n-1]) {
		return lastSpace + 1
	}
	return n
}

// unspaced reports whether r is of a script written without spaces.
func unspaced(r rune) bool {
	return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Thai, unicode.Hangul)
}

// isRTL reports whether text begins in a script read right to left.
func isRTL(text string) bool {
	for _, r := range text {
		switch {
		case unicode.In(r, unicode.Hebrew, unicode.Arabic, unicode.Syriac, unicode.Thaana):
			return true
		case unicode.IsLetter(r):
			return false
		}
	}
	return false
}

// blockHeight is the height of lines.
func blockHeight(lines []setLine) float64 {
	var h float64
	for _, l := range lines {
		h += l.gap + l.height*float64(len(l.rows))
	}
	return h
}

// fitHeight drops the rows of lines that don't fit room, from the last
// line's last row, ending the last row that stays with an ellipsis.
func (t *typesetter) fitHeight(lines []setLine, room float64) []setLine {
	for blockHeight(lines) > room && len(lines) > 0 {
		last := &lines[len(lines)-1]
		if len(last.rows) > 1 {
			last.rows = last.rows[:len(last.rows)-1]
			r := &last.rows[len(last.rows)-1]
			if !strings.HasSuffix(r.text, "…") {
				r.text = strings.TrimRight(r.text, " ") + "…"
				r.width = t.advance(t.primary(last.weight), last.weight, last.px, r.text, last.rtl)
			}
			continue
		}
		lines = lines[:len(lines)-1]
	}
	return lines
}

// draw draws lines from top, in the box that starts at left and is width
// wide.
func (t *typesetter) draw(img *image.NRGBA, lines []setLine, left, top, width float64, align Align) {
	y := top
	for _, l := range lines {
		y += l.gap
		prim := t.primary(l.weight)
		d := t.data(prim)
		if d == nil {
			continue
		}
		ext, _ := t.sh.face(d, l.weight).FontHExtents()
		ascent := float64(ext.Ascender) / d.upem * l.px
		descent := -float64(ext.Descender) / d.upem * l.px
		lead := (l.height - ascent - descent) / 2
		for _, r := range l.rows {
			x := left
			switch {
			case align == AlignCenter:
				x = left + (width-r.width)/2
			case align == AlignAuto && l.rtl:
				x = left + width - r.width
			}
			t.drawRow(img, prim, l.weight, l.px, r.text, l.rtl, x, y+lead+ascent, l.ink)
			y += l.height
		}
	}
}

// drawRow draws text, shaped, with its baseline at y, from x.
func (t *typesetter) drawRow(img *image.NRGBA, prim *Font, weight int, px float64, text string, rtl bool, x, y float64, ink color.NRGBA) {
	pen := x
	for _, r := range t.shape(prim, weight, text, rtl).runs {
		fc := t.sh.face(r.data, weight)
		scale := px / r.data.upem
		for _, g := range r.glyphs {
			gx := pen + float64(g.XOffset)/64*scale
			gy := y - float64(g.YOffset)/64*scale
			t.sh.drawGlyph(img, r.data, fc, g.GlyphID, scale, gx, gy, ink)
			pen += float64(g.Advance) / 64 * scale
		}
	}
}
