// Package taskbar puts the player's buttons on the thumbnail of a window's
// taskbar button, which Windows shows while the pointer rests on the
// button: previous, play or pause, and next, as music players have them
// there.
package taskbar

import (
	"errors"
	"image"
	"image/color"
)

// ErrUnsupported is returned where windows have no thumbnail buttons.
var ErrUnsupported = errors.New("taskbar: thumbnail buttons are not supported here")

// Button is a button of the thumbnail.
type Button int

const (
	Previous Button = iota
	PlayPause
	Next
)

// Bar is the buttons on a window's thumbnail.
type Bar struct{ b *bar }

// Attach puts the buttons on the thumbnail of the window hwnd, and has
// click take their clicks. It runs on the window's thread, which click runs
// on too. The buttons show a player that doesn't play, with nothing to
// play, until Show.
func Attach(hwnd uintptr, click func(Button)) (*Bar, error) {
	b, err := attach(hwnd, click)
	if err != nil {
		return nil, err
	}
	return &Bar{b}, nil
}

// Show shows whether the player plays, and whether it has a track: without
// one, the buttons are off. It returns at once, from any goroutine; the
// buttons go with their window.
func (b *Bar) Show(playing, track bool) { b.b.show(state{playing, track}) }

// state is what the buttons show: whether the player plays, and whether it
// has a track, without which they're off.
type state struct{ playing, track bool }

// point is a point of a glyph, in a square of side 1, y down.
type point struct{ x, y float64 }

// glyph is the convex shapes an icon is made of.
type glyph [][]point

// The glyphs of the buttons, as the player bar draws them.
var (
	glyphPrevious = glyph{
		{{0.22, 0.25}, {0.32, 0.25}, {0.32, 0.75}, {0.22, 0.75}},
		{{0.80, 0.25}, {0.80, 0.75}, {0.36, 0.50}},
	}
	glyphPlay  = glyph{{{0.30, 0.20}, {0.80, 0.50}, {0.30, 0.80}}}
	glyphPause = glyph{
		{{0.26, 0.22}, {0.42, 0.22}, {0.42, 0.78}, {0.26, 0.78}},
		{{0.58, 0.22}, {0.74, 0.22}, {0.74, 0.78}, {0.58, 0.78}},
	}
	glyphNext = glyph{
		{{0.20, 0.25}, {0.64, 0.50}, {0.20, 0.75}},
		{{0.68, 0.25}, {0.78, 0.25}, {0.78, 0.75}, {0.68, 0.75}},
	}
)

// samples is how many samples a pixel's side takes: their share inside the
// glyph is the pixel's alpha, which smooths the edges.
const samples = 4

// render draws g in ink, size pixels square.
func (g glyph) render(size int, ink color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := range size {
		for x := range size {
			in := 0
			for sy := range samples {
				for sx := range samples {
					p := point{(float64(x) + (float64(sx)+0.5)/samples) / float64(size), (float64(y) + (float64(sy)+0.5)/samples) / float64(size)}
					if g.covers(p) {
						in++
					}
				}
			}
			if in > 0 {
				c := ink
				c.A = uint8(int(ink.A) * in / (samples * samples))
				img.SetNRGBA(x, y, c)
			}
		}
	}
	return img
}

// covers tells whether p is inside one of the glyph's shapes.
func (g glyph) covers(p point) bool {
	for _, shape := range g {
		if inside(shape, p) {
			return true
		}
	}
	return false
}

// inside tells whether p is inside the convex polygon shape: on the same
// side of all of its edges.
func inside(shape []point, p point) bool {
	var pos, neg bool
	for i, a := range shape {
		b := shape[(i+1)%len(shape)]
		cross := (b.x-a.x)*(p.y-a.y) - (b.y-a.y)*(p.x-a.x)
		pos = pos || cross > 0
		neg = neg || cross < 0
	}
	return !(pos && neg)
}
