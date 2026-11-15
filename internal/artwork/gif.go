package artwork

// Covers that move: GIFs, as a mix's cover or a music file's (#569). What
// one takes decoded is read from its blocks before it is decoded, so that
// a GIF of thousands of frames isn't decoded only to be refused, and a
// mix's GIF is kept as its centre square, as its still is.

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
)

// IsGIF reports whether data is a GIF, by its signature.
func IsGIF(data []byte) bool {
	return bytes.HasPrefix(data, []byte("GIF87a")) || bytes.HasPrefix(data, []byte("GIF89a"))
}

// GIFSize is what a GIF takes decoded, as it is read from its blocks.
type GIFSize struct {
	// Frames counts its frames.
	Frames int
	// Pixels counts the pixels of its frames, which decode a byte each.
	Pixels int64
	// Width and Height are of the picture the frames draw into.
	Width, Height int
	// Previous is set when a frame is undone to what was under it, which
	// takes a copy of the picture.
	Previous bool
}

// Memory is the bytes the GIF takes decoded and playing, as MyGo's GIF
// decoder keeps it: its frames' pixels, a byte each, the picture they
// draw into, four bytes a pixel, and its copy for a frame undone to what
// was under it.
func (s GIFSize) Memory() int64 {
	pic := 4 * int64(s.Width) * int64(s.Height)
	if s.Previous {
		pic *= 2
	}
	return s.Pixels + pic
}

// errGIF is a GIF whose blocks don't read.
var errGIF = errors.New("artwork: a GIF that doesn't read")

// SizeOfGIF reads what a GIF takes decoded from its blocks, without
// decoding its frames.
func SizeOfGIF(data []byte) (GIFSize, error) {
	var s GIFSize
	if !IsGIF(data) || len(data) < 13 {
		return s, errGIF
	}
	s.Width, s.Height = int(binary.LittleEndian.Uint16(data[6:])), int(binary.LittleEndian.Uint16(data[8:]))
	at := 13
	if flags := data[10]; flags&0x80 != 0 {
		at += 3 << (flags&7 + 1)
	}
	// skip passes over the sub-blocks from at, to the one of no bytes that
	// ends them.
	skip := func() bool {
		for at < len(data) {
			n := int(data[at])
			at += 1 + n
			if n == 0 {
				return true
			}
		}
		return false
	}
	var bounds image.Rectangle
	for at < len(data) {
		switch data[at] {
		case 0x3b: // the trailer
			if s.Width == 0 || s.Height == 0 {
				s.Width, s.Height = bounds.Max.X, bounds.Max.Y
			}
			return s, nil
		case 0x21: // an extension
			if at+2 > len(data) {
				return s, errGIF
			}
			if data[at+1] == 0xf9 && at+4 <= len(data) && data[at+2] >= 1 {
				// A graphic control's disposal: 3 undoes to what was under.
				if (data[at+3]>>2)&7 == 3 {
					s.Previous = true
				}
			}
			at += 2
			if !skip() {
				return s, errGIF
			}
		case 0x2c: // an image
			if at+10 > len(data) {
				return s, errGIF
			}
			x, y := int(binary.LittleEndian.Uint16(data[at+1:])), int(binary.LittleEndian.Uint16(data[at+3:]))
			w, h := int(binary.LittleEndian.Uint16(data[at+5:])), int(binary.LittleEndian.Uint16(data[at+7:]))
			flags := data[at+9]
			at += 10
			if flags&0x80 != 0 {
				at += 3 << (flags&7 + 1)
			}
			at++ // the LZW code size
			if at > len(data) || !skip() {
				return s, errGIF
			}
			s.Frames++
			s.Pixels += int64(w) * int64(h)
			bounds = bounds.Union(image.Rect(x, y, x+w, y+h))
		default:
			return s, errGIF
		}
	}
	// No trailer: decoders show the frames that came.
	if s.Frames == 0 {
		return s, errGIF
	}
	if s.Width == 0 || s.Height == 0 {
		s.Width, s.Height = bounds.Max.X, bounds.Max.Y
	}
	return s, nil
}

// SquareGIF returns the centre square of a GIF that moves, at most px
// across, and its first frame as a still, as Square makes it: a JPEG, or
// a PNG when it isn't opaque. A square GIF no larger than px comes back
// as it is. The frames keep their colours, their delays and how each is
// undone: each pixel of a smaller square is the nearest of the GIF's, as
// a GIF's frames have a palette each, which smoothing would leave. A GIF
// of one frame is no moving cover: moving is nil.
func SquareGIF(data []byte, px int) (moving, still []byte, err error) {
	g, err := gif.DecodeAll(bytes.NewReader(data))
	if err != nil {
		return nil, nil, fmt.Errorf("artwork: %w", err)
	}
	w, h := g.Config.Width, g.Config.Height
	if w == 0 || h == 0 {
		var r image.Rectangle
		for _, f := range g.Image {
			r = r.Union(f.Bounds())
		}
		w, h = r.Max.X, r.Max.Y
	}
	side := min(w, h)
	if side <= 0 || px <= 0 || len(g.Image) == 0 {
		return nil, nil, errors.New("artwork: no picture to square")
	}
	first := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(first, g.Image[0].Bounds(), g.Image[0], g.Image[0].Bounds().Min, draw.Over)
	if still, err = squareOf(first, px); err != nil || len(g.Image) == 1 {
		return nil, still, err
	}
	if w == h && w <= px {
		return data, still, nil
	}
	n := min(side, px)
	ox, oy := (w-side)/2, (h-side)/2
	// at is the pixel of the GIF, across or down, that pixel i of the
	// square shows: the nearest to its centre.
	at := func(o, i int) int { return o + (2*i+1)*side/(2*n) }
	for i, f := range g.Image {
		g.Image[i] = squareFrame(f, n, func(i int) int { return at(ox, i) }, func(i int) int { return at(oy, i) })
	}
	g.Config.Width, g.Config.Height = n, n
	var out bytes.Buffer
	if err := gif.EncodeAll(&out, g); err != nil {
		return nil, nil, fmt.Errorf("artwork: %w", err)
	}
	return out.Bytes(), still, nil
}

// squareFrame is the part of frame f a square n across shows, each of its
// pixels f's at x(i) across and y(j) down: what f covers of the square,
// or, where it covers none of it, a pixel that changes nothing, to keep
// its delay.
func squareFrame(f *image.Paletted, n int, x, y func(int) int) *image.Paletted {
	b := f.Bounds()
	span := func(lo, hi int, at func(int) int) (from, to int) {
		from, to = n, 0
		for i := range n {
			if v := at(i); v >= lo && v < hi {
				from, to = min(from, i), i+1
			}
		}
		return from, to
	}
	x0, x1 := span(b.Min.X, b.Max.X, x)
	y0, y1 := span(b.Min.Y, b.Max.Y, y)
	if x0 >= x1 || y0 >= y1 {
		return image.NewPaletted(image.Rect(0, 0, 1, 1), color.Palette{color.RGBA{}})
	}
	out := image.NewPaletted(image.Rect(x0, y0, x1, y1), f.Palette)
	for j := y0; j < y1; j++ {
		row := out.Pix[out.PixOffset(x0, j):]
		sy := y(j)
		for i := x0; i < x1; i++ {
			row[i-x0] = f.Pix[f.PixOffset(x(i), sy)]
		}
	}
	return out
}
