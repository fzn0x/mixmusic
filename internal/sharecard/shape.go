package sharecard

import (
	"slices"
	"unicode"

	"github.com/go-text/typesetting/bidi"
	"github.com/go-text/typesetting/di"
	gotext "github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/shaping"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"
)

// A card shapes each run of its text with go-text's port of HarfBuzz: a
// run is text of one direction, one script and one font, and the shaper
// applies the font's rules to it, so letters join, conjuncts form and
// marks sit where the font puts them. The runs of a row show in the order
// the bidirectional algorithm gives them.

// shaper is what a card shapes and draws its text with. Its caches and
// faces aren't safe to share, so each card makes its own.
type shaper struct {
	hb     shaping.HarfbuzzShaper
	seg    shaping.Segmenter
	para   bidi.Paragraph
	faces  map[*fontData]*face
	raster vector.Rasterizer
	mask   []byte
}

// face is a font's face.
type face struct {
	*gotext.Face
}

func newShaper() *shaper { return &shaper{faces: map[*fontData]*face{}} }

// face is the face of d for a line of weight. A font is shaped through one
// face, which HarfBuzz's caches keep.
func (s *shaper) face(d *fontData, weight int) *face {
	fc, ok := s.faces[d]
	if !ok {
		fc = &face{Face: gotext.NewFace(d.font)}
		s.faces[d] = fc
	}
	return fc
}

// shaped is a row of text, shaped: its runs, in the order they show, left
// to right.
type shaped struct {
	runs []run
}

// run is a run of a row's text in one font, script and direction, shaped:
// its glyphs in the order they show, its runes start to end of the row's
// text, its bidirectional level, and its width in the font's units.
type run struct {
	data       *fontData
	glyphs     []shaping.Glyph
	start, end int
	level      bidi.Level
	units      float64
}

// width is the width of the row at px.
func (s *shaped) width(px float64) float64 {
	var w float64
	for _, r := range s.runs {
		w += r.units / r.data.upem * px
	}
	return w
}

// shapeKey is a text shaped in a typesetter: its primary font, weight and
// direction.
type shapeKey struct {
	prim   *Font
	weight int
	text   string
	rtl    bool
}

// oneFace is a fontmap of a single face, so that the segmenter splits text
// by its direction and script only: the fonts are chosen by fontFor.
type oneFace struct{ f *gotext.Face }

func (o oneFace) ResolveFace(rune) *gotext.Face { return o.f }

// shape shapes text in prim and its fallbacks, at weight, in a paragraph
// that reads right to left when rtl is set. A text is shaped once and kept,
// at the size of the fonts' units, which any size scales.
func (t *typesetter) shape(prim *Font, weight int, text string, rtl bool) *shaped {
	k := shapeKey{prim, weight, text, rtl}
	if s, ok := t.shapes[k]; ok {
		return s
	}
	s := &shaped{}
	t.shapes[k] = s
	rs := []rune(text)
	pd := t.data(prim)
	if len(rs) == 0 || pd == nil {
		return s
	}
	dir, bdir := di.DirectionLTR, bidi.LeftToRight
	if rtl {
		dir, bdir = di.DirectionRTL, bidi.RightToLeft
	}
	sh := t.sh
	levels := make([]bidi.Level, len(rs))
	b := sh.para.Segment(rs, bdir)
	for i := range b.NumRuns() {
		r := b.Run(i)
		for j := r.Start; j < r.End && j < len(rs); j++ {
			levels[j] = r.Level
		}
	}
	in := shaping.Input{Text: rs, RunEnd: len(rs), Direction: dir}
	for _, seg := range sh.seg.Split(in, oneFace{sh.face(pd, weight).Face}) {
		var prev *Font
		start, cur := seg.RunStart, (*Font)(nil)
		for i := seg.RunStart; i < seg.RunEnd; i++ {
			f := t.fontFor(prim, rs, i, prev)
			prev = f
			if cur != nil && f != cur {
				s.runs = append(s.runs, t.shapeRun(seg, cur, weight, start, i, levels[start]))
				start = i
			}
			cur = f
		}
		if cur != nil {
			s.runs = append(s.runs, t.shapeRun(seg, cur, weight, start, seg.RunEnd, levels[start]))
		}
	}
	reorder(s.runs)
	return s
}

// shapeRun shapes the runes start to end of in, a run of one direction
// and script, in f.
func (t *typesetter) shapeRun(in shaping.Input, f *Font, weight, start, end int, level bidi.Level) run {
	d := t.data(f)
	in.Face = t.sh.face(d, weight).Face
	in.RunStart, in.RunEnd = start, end
	// At a size of the font's units per em, the glyphs' positions are in
	// its units, which any size scales.
	in.Size = fixed.I(int(d.upem))
	out := t.sh.hb.ShapeNoExtents(in)
	r := run{data: d, glyphs: out.Glyphs, start: start, end: end, level: level}
	for _, g := range out.Glyphs {
		r.units += float64(g.Advance) / 64
	}
	return r
}

// reorder puts runs in the order they show, from their bidirectional
// levels: from the highest level to the lowest odd one, each sequence of
// runs at that level or higher is reversed.
func reorder(runs []run) {
	hi, lo := 0, 256
	for _, r := range runs {
		hi = max(hi, int(r.level))
		if r.level%2 == 1 {
			lo = min(lo, int(r.level))
		}
	}
	for lv := hi; lv >= lo; lv-- {
		for i := 0; i < len(runs); {
			if int(runs[i].level) < lv {
				i++
				continue
			}
			j := i
			for j < len(runs) && int(runs[j].level) >= lv {
				j++
			}
			slices.Reverse(runs[i:j])
			i = j
		}
	}
}

// fontFor is the font that draws rs[i], after a rune drawn in prev. A mark,
// a joiner, a variation selector, a skin tone or a space stays in the font
// of the letter before it when that font has it, so that what joins is
// shaped together. Any other letter comes from the primary font, or the
// first fallback that has it, in the order of the fallbacks.
func (t *typesetter) fontFor(prim *Font, rs []rune, i int, prev *Font) *Font {
	r := rs[i]
	if r < 0x20 {
		return prim
	}
	if prev != nil && (ignorable(r) || sticks(r) && t.has(prev, r)) {
		return prev
	}
	if t.has(prim, r) {
		return prim
	}
	for _, f := range t.fam.Fallback {
		if t.has(f, r) {
			return f
		}
	}
	return prim
}

// sticks reports whether r stays in the font of the letter before it.
func sticks(r rune) bool {
	return unicode.In(r, unicode.Mn, unicode.Mc, unicode.Me, unicode.Zs) || r >= 0x1F3FB && r <= 0x1F3FF
}

// ignorable reports whether r is a joiner, a variation selector or a tag,
// which shaping hides when a font has no glyph for it.
func ignorable(r rune) bool {
	return r == 0x200C || r == 0x200D || r >= 0xFE00 && r <= 0xFE0F || r >= 0xE0020 && r <= 0xE007F ||
		r >= 0xE0100 && r <= 0xE01EF
}

// measure is what wrapping a text needs: its runes, the advance of each,
// in ems, the whole advance of a cluster on its first rune, and where a
// row may begin, at the start of a cluster, never inside one.
type measure struct {
	rs  []rune
	adv []float64
	cut []bool
}

// measure measures text, shaped whole.
func (t *typesetter) measure(prim *Font, weight int, text string, rtl bool) measure {
	s := t.shape(prim, weight, text, rtl)
	rs := []rune(text)
	m := measure{rs: rs, adv: make([]float64, len(rs)), cut: make([]bool, len(rs)+1)}
	m.cut[len(rs)] = true
	for _, r := range s.runs {
		m.cut[r.start] = true
		for _, g := range r.glyphs {
			i := g.TextIndex()
			if i < 0 || i >= len(rs) {
				continue
			}
			m.adv[i] += float64(g.Advance) / 64 / r.data.upem
			m.cut[i] = true
		}
	}
	return m
}

// cluster is the end of the cluster that starts at i.
func (m measure) cluster(i int) int {
	e := i + 1
	for e < len(m.rs) && !m.cut[e] {
		e++
	}
	return e
}

// back is the start of the cluster before end, but not before from.
func (m measure) back(end, from int) int {
	for end--; end > from && !m.cut[end]; end-- {
	}
	return max(end, from)
}
