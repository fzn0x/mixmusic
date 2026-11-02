// Package hct is Material's colour science: CAM16, the colour appearance
// model, and HCT, its hue and chroma with L*'s tone, in which Material
// makes tonal palettes and the colour roles of a scheme from one seed
// colour. It is a port of Material Color Utilities, by Google, under the
// Apache License 2.0 of LICENSE.material-color-utilities.
//
// Colours are ARGB, as Material Color Utilities has them: 0xAARRGGBB.
package hct

import (
	_ "embed"
	"math"
)

// License is the license of Material Color Utilities, whose copies must
// reproduce it.
//
//go:embed LICENSE.material-color-utilities
var License string

// ARGB is a colour, 0xAARRGGBB.
type ARGB uint32

// RGB is the opaque colour of r, g and b.
func RGB(r, g, b uint8) ARGB {
	return ARGB(0xff000000 | uint32(r)<<16 | uint32(g)<<8 | uint32(b))
}

// RGB returns c's red, green and blue.
func (c ARGB) RGB() (r, g, b uint8) { return uint8(c >> 16), uint8(c >> 8), uint8(c) }

// Color is a colour by its hue, chroma and tone.
type Color struct {
	// Hue is CAM16's, in degrees from 0 to 360; Chroma CAM16's, from 0 to
	// about 150 for the most colourful colours sRGB has; Tone is L*, from
	// 0, black, to 100, white.
	Hue, Chroma, Tone float64
	argb              ARGB
}

// FromARGB is c in HCT.
func FromARGB(c ARGB) Color {
	cam := camOf(c)
	return Color{Hue: cam.hue, Chroma: cam.chroma, Tone: lstarOf(c), argb: c}
}

// From is the sRGB colour nearest to hue, chroma and tone: of that hue and
// tone, and of the chroma, or the most chroma sRGB has at them.
func From(hue, chroma, tone float64) Color { return FromARGB(solve(hue, chroma, tone)) }

// ARGB is the colour.
func (c Color) ARGB() ARGB { return c.argb }

// The sRGB colour space's matrices and white point, as Material Color
// Utilities has them.
var (
	srgbToXYZ = [3][3]float64{
		{0.41233895, 0.35762064, 0.18051042},
		{0.2126, 0.7152, 0.0722},
		{0.01932141, 0.11916382, 0.95034478},
	}
	xyzToSRGB = [3][3]float64{
		{3.2413774792388685, -1.5376652402851851, -0.49885366846268053},
		{-0.9691452513005321, 1.8758853451067872, 0.04156585616912061},
		{0.05562093689691305, -0.20395524564742123, 1.0571799111220335},
	}
	whitePointD65 = [3]float64{95.047, 100.0, 108.883}
)

// mul is the matrix m times the vector v.
func mul(v [3]float64, m [3][3]float64) [3]float64 {
	return [3]float64{
		v[0]*m[0][0] + v[1]*m[0][1] + v[2]*m[0][2],
		v[0]*m[1][0] + v[1]*m[1][1] + v[2]*m[1][2],
		v[0]*m[2][0] + v[1]*m[2][1] + v[2]*m[2][2],
	}
}

// linearized is a component of sRGB, from 0 to 255, linear, from 0 to
// 100.
func linearized(c uint8) float64 {
	n := float64(c) / 255
	if n <= 0.040449936 {
		return n / 12.92 * 100
	}
	return math.Pow((n+0.055)/1.055, 2.4) * 100
}

// trueDelinearized is a linear component, from 0 to 100, in sRGB, from 0
// to 255, unrounded.
func trueDelinearized(c float64) float64 {
	n := c / 100
	if n <= 0.0031308 {
		return n * 12.92 * 255
	}
	return (1.055*math.Pow(n, 1/2.4) - 0.055) * 255
}

// delinearized is trueDelinearized, rounded and clamped.
func delinearized(c float64) uint8 {
	return uint8(min(max(math.Round(trueDelinearized(c)), 0), 255))
}

func fromLinRGB(l [3]float64) ARGB {
	return RGB(delinearized(l[0]), delinearized(l[1]), delinearized(l[2]))
}

func fromXYZ(x, y, z float64) ARGB { return fromLinRGB(mul([3]float64{x, y, z}, xyzToSRGB)) }

func xyzOf(c ARGB) [3]float64 {
	r, g, b := c.RGB()
	return mul([3]float64{linearized(r), linearized(g), linearized(b)}, srgbToXYZ)
}

func labF(t float64) float64 {
	const e, kappa = 216.0 / 24389.0, 24389.0 / 27.0
	if t > e {
		return math.Cbrt(t)
	}
	return (kappa*t + 16) / 116
}

func labInvF(ft float64) float64 {
	const e, kappa = 216.0 / 24389.0, 24389.0 / 27.0
	if ft3 := ft * ft * ft; ft3 > e {
		return ft3
	}
	return (116*ft - 16) / kappa
}

// lstarOf is c's L*, its tone.
func lstarOf(c ARGB) float64 { return 116*labF(xyzOf(c)[1]/100) - 16 }

// yFromLstar is the luminance Y, from 0 to 100, of L*.
func yFromLstar(lstar float64) float64 { return 100 * labInvF((lstar+16)/116) }

// gray is the gray of L*.
func gray(lstar float64) ARGB {
	v := delinearized(yFromLstar(lstar))
	return RGB(v, v, v)
}

// sanitizeDegrees is an angle from 0 to 360.
func sanitizeDegrees(d float64) float64 {
	d = math.Mod(d, 360)
	if d < 0 {
		d += 360
	}
	return d
}

func signum(x float64) float64 {
	switch {
	case x < 0:
		return -1
	case x > 0:
		return 1
	}
	return 0
}
