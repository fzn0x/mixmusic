// Package volume is the curves of MixMusic's volume: what a position of
// the slider, from 0 to 1, sounds like on each, and how settings.json
// keeps it.
//
// On Natural, MixMusic's own curve, a position p plays at the gain
// (1000^p - 1) / 999. Each tenth of the slider is 6 dB, down to -40 dB,
// and below that the curve bends down to silence at 0; the middle is
// -30 dB. On Linear, p plays at the gain p: the middle is -6 dB.
//
// settings.json keeps the square root of the position's gain on Natural,
// whatever the curve, which is the position on the curve of earlier
// versions, whose gain was its square: every version plays a saved volume
// as loud on Natural, and a change of the curve leaves the slider where it
// was.
package volume

import "math"

// Curve is how the positions of the volume slider sound: the gain each
// plays at.
type Curve string

const (
	// Natural has even steps of loudness: each tenth of the slider is
	// 6 dB.
	Natural Curve = "natural"
	// Linear plays each position at its own gain, so that the middle of
	// the slider is half the level.
	Linear Curve = "linear"
)

// Curves are the curves, in the order Settings offers them.
var Curves = []Curve{Natural, Linear}

// Gain is the gain the position p plays at on c, from 0 to 1. A curve it
// doesn't know is Natural.
func (c Curve) Gain(p float64) float64 {
	p = min(max(p, 0), 1)
	if c == Linear {
		return p
	}
	return (math.Pow(1000, p) - 1) / 999
}

// Share is the share of a Spotify Connect device's volume the position p
// is sent as on c: the square root of its gain, for a device that plays a
// share at its square, as MixMusic once did, plays it as loud as MixMusic
// plays p.
func (c Curve) Share(p float64) float64 { return math.Sqrt(c.Gain(p)) }

// Gain is the gain the position p plays at on Natural, from 0 to 1.
func Gain(p float64) float64 { return Natural.Gain(p) }

// Saved is the position p as settings.json keeps it, whatever the curve:
// its share on Natural.
func Saved(p float64) float64 { return Natural.Share(p) }

// Of is the position of a volume settings.json keeps as v.
func Of(v float64) float64 {
	v = min(max(v, 0), 1)
	return math.Log(999*v*v+1) / math.Log(1000)
}
