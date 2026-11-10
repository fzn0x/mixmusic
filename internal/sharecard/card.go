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
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
	"golang.org/x/text/unicode/bidi"
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
	Font   *sfnt.Font
}

// Family is the faces a card's text is set in. A line takes the face
// nearest the weight it asks for; a letter the face lacks, a Chinese
// character say, comes from the first of Fallback that has it, and from
// the family's own .notdef box when none does.
type Family struct {
	Faces    []Face
	Fallback []*sfnt.Font
}

// ParseFont reads a TrueType or OpenType font.
func ParseFont(data []byte) (*sfnt.Font, error) { return opentype.Parse(data) }

// ParseCollectionFonts reads the fonts of a TrueType collection, a .ttc.
func ParseCollectionFonts(data []byte) ([]*sfnt.Font, error) {
	c, err := opentype.ParseCollection(data)
	if err != nil {
		return nil, err
	}
	var out []*sfnt.Font
	for i := range c.NumFonts() {
		f, err := c.Font(i)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, nil
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
	r := &typesetter{fam: fam, faces: map[faceKey]font.Face{}}
	defer r.close()

	wide := c.Size.W > c.Size.H
	top, textLeft, textW := margin, margin, inner
	// The foot keeps its place at the bottom, so it is set first.
	footLeft, footW := margin, inner
	footLines := r.set(c, c.Foot, scale, footW, false)
	footH := blockHeight(footLines)
	footTop := h - margin - footH
	switch {
	case c.Cover == nil:
	case c.CoverAt == CoverFoot:
		side := math.Min(math.Max(footH, h*0.1), h*0.16)
		drawCover(img, c.Cover, image.Rect(int(margin), int(h-margin-side), int(margin+side), int(h-margin)), scale)
		footLeft = margin + side + 28*scale
		footW = w - margin - footLeft
		footLines = r.set(c, c.Foot, scale, footW, false)
		footH = blockHeight(footLines)
		footTop = h - margin - side + (side-footH)/2
		if footH > side {
			footTop = h - margin - footH
		}
	case wide:
		// A wide card puts the cover beside its lines.
		side := math.Min(inner*0.5, h-2*margin)
		drawCover(img, c.Cover, image.Rect(int(margin), int(top), int(margin+side), int(top+side)), scale)
		textLeft = margin + side + margin*0.8
		textW = w - margin - textLeft
		footLeft, footW = textLeft, textW
		footLines = r.set(c, c.Foot, scale, footW, false)
		footTop = h - margin - blockHeight(footLines)
	default:
		side := math.Min(inner, h*0.42)
		drawCover(img, c.Cover, image.Rect(int(margin), int(top), int(margin+side), int(top+side)), scale)
		top += side + 56*scale
	}
	room := h - margin - top
	if len(footLines) > 0 {
		room = footTop - top - 32*scale
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
	return img, nil
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

// row is one row of a wrapped line, in the order it shows, left to right.
type row struct {
	text  string
	width float64
}

type faceKey struct {
	font *sfnt.Font
	size int
}

// typesetter measures and draws text in a family, keeping the faces it
// made.
type typesetter struct {
	fam   Family
	faces map[faceKey]font.Face
	buf   sfnt.Buffer
}

func (t *typesetter) close() {
	for _, f := range t.faces {
		f.Close()
	}
}

// face is the face of f at size px.
func (t *typesetter) face(f *sfnt.Font, px float64) font.Face {
	k := faceKey{f, int(math.Round(px * 64))}
	if fc, ok := t.faces[k]; ok {
		return fc
	}
	fc, err := opentype.NewFace(f, &opentype.FaceOptions{Size: px, DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		return nil
	}
	t.faces[k] = fc
	return fc
}

// primary is the face of the family nearest weight.
func (t *typesetter) primary(weight int) *sfnt.Font {
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

// fontFor is the font that draws r: the primary one, or the first
// fallback that has a glyph for it.
func (t *typesetter) fontFor(primary *sfnt.Font, r rune) *sfnt.Font {
	if r < 0x20 {
		return primary
	}
	if i, err := primary.GlyphIndex(&t.buf, r); err == nil && i != 0 {
		return primary
	}
	for _, f := range t.fam.Fallback {
		if i, err := f.GlyphIndex(&t.buf, r); err == nil && i != 0 {
			return f
		}
	}
	return primary
}

// advance is the width of text at px, in pixels.
func (t *typesetter) advance(primary *sfnt.Font, px float64, text string) float64 {
	var w fixed.Int26_6
	for _, r := range text {
		fc := t.face(t.fontFor(primary, r), px)
		if fc == nil {
			continue
		}
		a, _ := fc.GlyphAdvance(r)
		w += a
	}
	return float64(w) / 64
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
		rows := t.wrap(prim, px, text, width, maxLines)
		sl := setLine{rows: rows, ink: ink, weight: weight, px: px, height: px * 1.25, rtl: isRTL(text)}
		if len(out) > 0 {
			sl.gap = l.Gap * scale
		}
		out = append(out, sl)
	}
	return out
}

// wrap breaks text into at most maxLines rows no wider than width at px,
// at spaces, or anywhere in a word too long, and ends the last row with an
// ellipsis if text goes on.
func (t *typesetter) wrap(prim *sfnt.Font, px float64, text string, width float64, maxLines int) []row {
	var rows []row
	text = shapeArabic(text)
	rs := []rune(text)
	i := 0
	for i < len(rs) && len(rows) < maxLines {
		for i < len(rs) && rs[i] == ' ' && len(rows) > 0 {
			i++
		}
		if i >= len(rs) {
			break
		}
		last := len(rows) == maxLines-1
		end, w := t.fit(prim, px, rs[i:], width, last)
		line := string(rs[i : i+end])
		if last && i+end < len(rs) {
			line = strings.TrimRight(line, " ") + "…"
			w = t.advance(prim, px, line)
			// The ellipsis may overflow a row that was filled.
			for w > width && len([]rune(line)) > 1 {
				rr := []rune(line)
				line = strings.TrimRight(string(rr[:len(rr)-2]), " ") + "…"
				w = t.advance(prim, px, line)
			}
		} else {
			line = strings.TrimRight(line, " ")
			w = t.advance(prim, px, line)
		}
		rows = append(rows, row{text: line, width: w})
		i += end
	}
	return rows
}

// fit is how many of rs fit a row of width: up to the last space that
// fits, or as many letters as fit when no space does. With last set, the
// row takes as many as fit, leaving room for an ellipsis if rs goes on.
func (t *typesetter) fit(prim *sfnt.Font, px float64, rs []rune, width float64, last bool) (int, float64) {
	var w float64
	lastSpace, n := -1, 0
	for n < len(rs) {
		a := t.advance(prim, px, string(rs[n]))
		if w+a > width && n > 0 {
			break
		}
		w += a
		if rs[n] == ' ' {
			lastSpace = n
		}
		n++
	}
	if n == len(rs) {
		return n, w
	}
	// Break at the last space, unless the letters are of a script written
	// without them, which breaks anywhere, or the row is the last, which
	// takes all that fits.
	if lastSpace > 0 && !last && !unspaced(rs[n]) && !unspaced(rs[n-1]) {
		return lastSpace + 1, w
	}
	return n, w
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
				r.width = t.advance(t.primary(last.weight), last.px, r.text)
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
		face := t.face(prim, l.px)
		if face == nil {
			continue
		}
		m := face.Metrics()
		ascent := float64(m.Ascent) / 64
		descent := float64(m.Descent) / 64
		lead := (l.height - ascent - descent) / 2
		for _, r := range l.rows {
			x := left
			rtl := l.rtl
			switch {
			case align == AlignCenter:
				x = left + (width-r.width)/2
			case align == AlignAuto && rtl:
				x = left + width - r.width
			}
			t.drawRow(img, prim, l.px, visual(r.text), x, y+lead+ascent, l.ink)
			y += l.height
		}
	}
}

// visual is text in the order it shows, left to right: the runs that read
// right to left, of Hebrew and Arabic, reversed.
func visual(text string) string {
	hasRTL := false
	for _, r := range text {
		if unicode.In(r, unicode.Hebrew, unicode.Arabic, unicode.Syriac, unicode.Thaana) {
			hasRTL = true
			break
		}
	}
	if !hasRTL {
		return text
	}
	var p bidi.Paragraph
	if _, err := p.SetString(text); err != nil {
		return text
	}
	o, err := p.Order()
	if err != nil {
		return text
	}
	var sb strings.Builder
	for i := range o.NumRuns() {
		run := o.Run(i)
		if run.Direction() == bidi.RightToLeft {
			sb.WriteString(bidi.ReverseString(run.String()))
		} else {
			sb.WriteString(run.String())
		}
	}
	return sb.String()
}

// drawRow draws text with its baseline at y, from x.
func (t *typesetter) drawRow(img *image.NRGBA, prim *sfnt.Font, px float64, text string, x, y float64, ink color.NRGBA) {
	src := image.NewUniform(ink)
	dot := fixed.P(0, 0)
	dot.X = fixed.Int26_6(math.Round(x * 64))
	dot.Y = fixed.Int26_6(math.Round(y * 64))
	for _, r := range text {
		fc := t.face(t.fontFor(prim, r), px)
		if fc == nil {
			continue
		}
		dr, mask, maskp, adv, ok := fc.Glyph(dot, r)
		if ok {
			draw.DrawMask(img, dr, src, image.Point{}, mask, maskp, draw.Over)
		}
		dot.X += adv
	}
}
