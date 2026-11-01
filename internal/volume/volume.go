// Package volume is the curve of MixMusic's volume: what a position of the
// slider, from 0 to 1, sounds like, and how settings.json keeps it.
//
// A position p plays at the gain (1000^p - 1) / 999. Each tenth of the
// slider is 6 dB, down to -40 dB, and below that the curve bends down to
// silence at 0; the middle is -30 dB. settings.json keeps the square root
// of the gain, which is the position on the curve of earlier versions,
// whose gain was its square: every version plays a saved volume as loud.
package volume

import "math"

// Gain is the gain the position p plays at, from 0 to 1.
func Gain(p float64) float64 {
	p = min(max(p, 0), 1)
	return (math.Pow(1000, p) - 1) / 999
}

// Saved is the position p as settings.json keeps it, which is also the
// share of Spotify Connect's volume it sends a device.
func Saved(p float64) float64 { return math.Sqrt(Gain(p)) }

// Of is the position of a volume settings.json keeps as v.
func Of(v float64) float64 {
	v = min(max(v, 0), 1)
	return math.Log(999*v*v+1) / math.Log(1000)
}
