package sharecard

import (
	"image"
	"image/color"
	"image/draw"
	"math"

	gotext "github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
)

// maxGlyphArea is the most pixels a glyph may cover; a bigger one is a
// broken font's, and is not drawn.
const maxGlyphArea = 4096 * 4096

// drawGlyph draws glyph gid of d, through fc, scale pixels to the font's
// unit, its origin at x and y, in ink.
func (s *shaper) drawGlyph(img *image.NRGBA, d *fontData, fc *face, gid gotext.GID, scale, x, y float64, ink color.NRGBA) {
	out, ok := fc.GlyphDataOutline(gid)
	if !ok {
		return
	}
	s.fill(img, out.Segments, scale, x, y, ink)
}

// fill fills the outline segs, in a font's units, scale pixels to the
// unit, its origin at x and y, in ink.
func (s *shaper) fill(img *image.NRGBA, segs []gotext.Segment, scale, x, y float64, ink color.NRGBA) {
	if len(segs) == 0 || ink.A == 0 {
		return
	}
	lo, hi := outlineBounds(segs)
	at := image.Rect(int(math.Floor(x+float64(lo.X)*scale)), int(math.Floor(y-float64(hi.Y)*scale)),
		int(math.Ceil(x+float64(hi.X)*scale)), int(math.Ceil(y-float64(lo.Y)*scale)))
	w, h := at.Dx(), at.Dy()
	if w <= 0 || h <= 0 || w*h > maxGlyphArea || !at.Overlaps(img.Bounds()) {
		return
	}
	z := &s.raster
	z.Reset(w, h)
	z.DrawOp = draw.Src
	ox, oy, sc := float32(x-float64(at.Min.X)), float32(y-float64(at.Min.Y)), float32(scale)
	pt := func(p gotext.SegmentPoint) (float32, float32) { return ox + p.X*sc, oy - p.Y*sc }
	open := false
	for _, sg := range segs {
		switch sg.Op {
		case ot.SegmentOpMoveTo:
			if open {
				z.ClosePath()
			}
			z.MoveTo(pt(sg.Args[0]))
			open = true
		case ot.SegmentOpLineTo:
			z.LineTo(pt(sg.Args[0]))
		case ot.SegmentOpQuadTo:
			bx, by := pt(sg.Args[0])
			cx, cy := pt(sg.Args[1])
			z.QuadTo(bx, by, cx, cy)
		case ot.SegmentOpCubeTo:
			bx, by := pt(sg.Args[0])
			cx, cy := pt(sg.Args[1])
			dx, dy := pt(sg.Args[2])
			z.CubeTo(bx, by, cx, cy, dx, dy)
		}
	}
	if open {
		z.ClosePath()
	}
	mask := s.alpha(w, h)
	z.Draw(mask, mask.Bounds(), image.Opaque, image.Point{})
	draw.DrawMask(img, at, image.NewUniform(ink), image.Point{}, mask, image.Point{}, draw.Over)
}

// alpha is a mask of w by h pixels, on the shaper's buffer: the rasterizer
// sets each of its pixels, so it needs no clearing.
func (s *shaper) alpha(w, h int) *image.Alpha {
	if cap(s.mask) < w*h {
		s.mask = make([]byte, w*h)
	}
	return &image.Alpha{Pix: s.mask[:w*h], Stride: w, Rect: image.Rect(0, 0, w, h)}
}

// outlineBounds is the box of the points of segs.
func outlineBounds(segs []gotext.Segment) (lo, hi gotext.SegmentPoint) {
	lo = gotext.SegmentPoint{X: math.MaxFloat32, Y: math.MaxFloat32}
	hi = gotext.SegmentPoint{X: -math.MaxFloat32, Y: -math.MaxFloat32}
	for _, sg := range segs {
		n := 1
		switch sg.Op {
		case ot.SegmentOpQuadTo:
			n = 2
		case ot.SegmentOpCubeTo:
			n = 3
		}
		for _, p := range sg.Args[:n] {
			lo.X, lo.Y = min(lo.X, p.X), min(lo.Y, p.Y)
			hi.X, hi.Y = max(hi.X, p.X), max(hi.Y, p.Y)
		}
	}
	return lo, hi
}
