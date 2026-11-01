package hct

import "math"

// viewing are the viewing conditions CAM16 sees colours in: Material's
// default, sRGB's D65 white, a gray world of L* 50 and an average
// surround.
type viewing struct {
	n, aw, nbb, ncb, c, nc float64
	rgbD                   [3]float64
	fl, flRoot, z          float64
}

var vc = func() viewing {
	white := whitePointD65
	adapting := 200 / math.Pi * yFromLstar(50) / 100
	const backgroundLstar, surround = 50.0, 2.0
	rW := white[0]*0.401288 + white[1]*0.650173 + white[2]*-0.051461
	gW := white[0]*-0.250268 + white[1]*1.204414 + white[2]*0.045854
	bW := white[0]*-0.002079 + white[1]*0.048952 + white[2]*0.953127
	f := 0.8 + surround/10
	var c float64
	if f >= 0.9 {
		c = lerp(0.59, 0.69, (f-0.9)*10)
	} else {
		c = lerp(0.525, 0.59, (f-0.8)*10)
	}
	d := f * (1 - (1/3.6)*math.Exp((-adapting-42)/92))
	d = min(max(d, 0), 1)
	rgbD := [3]float64{d*(100/rW) + 1 - d, d*(100/gW) + 1 - d, d*(100/bW) + 1 - d}
	k := 1 / (5*adapting + 1)
	k4 := k * k * k * k
	k4F := 1 - k4
	fl := k4*adapting + 0.1*k4F*k4F*math.Cbrt(5*adapting)
	n := yFromLstar(backgroundLstar) / white[1]
	z := 1.48 + math.Sqrt(n)
	nbb := 0.725 / math.Pow(n, 0.2)
	factor := func(d, w float64) float64 {
		f := math.Pow(fl*d*w/100, 0.42)
		return 400 * f / (f + 27.13)
	}
	aw := (2*factor(rgbD[0], rW) + factor(rgbD[1], gW) + 0.05*factor(rgbD[2], bW)) * nbb
	return viewing{n: n, aw: aw, nbb: nbb, ncb: nbb, c: c, nc: f, rgbD: rgbD, fl: fl, flRoot: math.Pow(fl, 0.25), z: z}
}()

func lerp(a, b, t float64) float64 { return (1-t)*a + t*b }

// cam16 is a colour as CAM16 sees it: its hue, chroma, lightness J,
// brightness Q, colourfulness M and saturation s.
type cam16 struct{ hue, chroma, j, q, m, s float64 }

// camOf is c in CAM16.
func camOf(c ARGB) cam16 {
	xyz := xyzOf(c)
	x, y, z := xyz[0], xyz[1], xyz[2]
	rC := 0.401288*x + 0.650173*y - 0.051461*z
	gC := -0.250268*x + 1.204414*y + 0.045854*z
	bC := -0.002079*x + 0.048952*y + 0.953127*z
	rD, gD, bD := vc.rgbD[0]*rC, vc.rgbD[1]*gC, vc.rgbD[2]*bC
	adapt := func(d float64) float64 {
		af := math.Pow(vc.fl*math.Abs(d)/100, 0.42)
		return signum(d) * 400 * af / (af + 27.13)
	}
	rA, gA, bA := adapt(rD), adapt(gD), adapt(bD)
	a := (11*rA + -12*gA + bA) / 11
	b := (rA + gA - 2*bA) / 9
	u := (20*rA + 20*gA + 21*bA) / 20
	p2 := (40*rA + 20*gA + bA) / 20
	hue := sanitizeDegrees(math.Atan2(b, a) * 180 / math.Pi)
	ac := p2 * vc.nbb
	j := 100 * math.Pow(ac/vc.aw, vc.c*vc.z)
	q := 4 / vc.c * math.Sqrt(j/100) * (vc.aw + 4) * vc.flRoot
	huePrime := hue
	if hue < 20.14 {
		huePrime += 360
	}
	eHue := 0.25 * (math.Cos(huePrime*math.Pi/180+2) + 3.8)
	p1 := 50000.0 / 13.0 * eHue * vc.nc * vc.ncb
	t := p1 * math.Sqrt(a*a+b*b) / (u + 0.305)
	alpha := math.Pow(t, 0.9) * math.Pow(1.64-math.Pow(0.29, vc.n), 0.73)
	chroma := alpha * math.Sqrt(j/100)
	m := chroma * vc.flRoot
	s := 50 * math.Sqrt(alpha*vc.c/(vc.aw+4))
	return cam16{hue: hue, chroma: chroma, j: j, q: q, m: m, s: s}
}
