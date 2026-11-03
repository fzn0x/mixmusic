package main

import (
	"math"

	"github.com/egoist/mygo"
)

// The player around the notch (#46) takes its size from the camera
// housing, whatever the MacBook: these are in points, as MyGo's windows
// are on macOS, and don't follow MixMusic's zoom.
const (
	// notchOpenWidth is the least width of the open panel, and notchBody
	// how much taller than the housing it grows: about 480 × 190 points
	// around a housing 32 points tall.
	notchOpenWidth = 480
	notchBody      = 158
	// notchInset is the room between the small cover, or the bars, and
	// the edges of the black around them.
	notchInset = 7
	// notchOpenRoom is the room the open panel's top row keeps beside
	// the small cover's place for the button that opens MixMusic.
	notchOpenRoom = 40
)

// housing is the camera housing at the top of a screen, in MyGo's screen
// coordinates, from the top-left corner of the primary screen.
type housing struct {
	X, Y, W, H int
}

// notchScreen is what AppKit tells of the top of a screen with a camera
// housing: the number of the screen (its NSScreenNumber, which is
// mygo.Display.ID), the widths of the unobscured parts of the menu bar
// left and right of the housing (auxiliaryTopLeftArea and
// auxiliaryTopRightArea), their height, and the gap between them.
type notchScreen struct {
	id                  int64
	left, right, height float64
	gap                 float64
}

// housingOf returns the housing of the built-in screen among displays, as
// screens tell it, and whether there is one: external displays, a closed
// lid and Macs without a notch have none.
func housingOf(displays []mygo.Display, screens []notchScreen) (housing, bool) {
	for _, d := range displays {
		if !d.Internal {
			continue
		}
		for _, s := range screens {
			if s.id != d.ID || s.left <= 0 || s.right <= 0 || s.gap <= 0 || s.height <= 0 {
				continue
			}
			h := housing{
				X: d.Bounds.X + int(math.Round(s.left)),
				Y: d.Bounds.Y,
				W: int(math.Round(s.gap)),
				H: int(math.Round(s.height)),
			}
			if h.W <= 0 || h.H <= 0 || h.X+h.W > d.Bounds.X+d.Bounds.Width || h.H*2 > d.Bounds.Height {
				continue
			}
			return h, true
		}
	}
	return housing{}, false
}

// art is the side of the small cover, and of the bars' box, beside the
// housing while compact.
func (h housing) art() int { return max(h.H-10, 8) }

// wing is how far the black widens the housing on each side while
// compact: the small cover on the left, the bars on the right.
func (h housing) wing() int { return h.art() + 2*notchInset }

// compact is the window while compact: the housing, widened by a wing on
// each side.
func (h housing) compact() mygo.Rectangle {
	return mygo.Rectangle{X: h.X - h.wing(), Y: h.Y, Width: h.W + 2*h.wing(), Height: h.H}
}

// open is the window while the panel is open or moving: at least
// notchOpenWidth wide, and wider around a wider housing, centred on it.
// It is wider than the housing by an even width, so that the housing sits
// in its middle to the point.
func (h housing) open() mygo.Rectangle {
	w := max(notchOpenWidth, h.W+2*(h.wing()+notchOpenRoom))
	if (w-h.W)%2 != 0 {
		w++
	}
	return mygo.Rectangle{X: h.X - (w-h.W)/2, Y: h.Y, Width: w, Height: h.H + notchBody}
}

// compactRadius rounds the bottom corners of the compact black, as the
// housing's own; openRadius those of the open panel.
func (h housing) compactRadius() float32 { return float32(h.H) / 3 }
func (h housing) openRadius() float32    { return 22 }
