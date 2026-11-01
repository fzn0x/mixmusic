package main

import (
	"image/color"
	"math"
	"strconv"
	"strings"
)

// parseColor reads a colour as themes write it: #rgb, #rgba, #rrggbb,
// #rrggbbaa, rgb(), rgba(), hsl(), hsla(), oklch(), oklab() or
// transparent, the forms addons accepts.
func parseColor(s string) (color.NRGBA, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "transparent" {
		return color.NRGBA{}, true
	}
	if hex, ok := strings.CutPrefix(s, "#"); ok {
		return parseHex(hex)
	}
	name, args, ok := strings.Cut(s, "(")
	if !ok || !strings.HasSuffix(args, ")") {
		return color.NRGBA{}, false
	}
	parts, alpha, ok := colorArgs(strings.TrimSuffix(args, ")"))
	if !ok || len(parts) != 3 {
		return color.NRGBA{}, false
	}
	var r, g, b float64
	switch name {
	case "rgb", "rgba":
		var vals [3]float64
		for i, p := range parts {
			v, pct, ok := number(p)
			if !ok {
				return color.NRGBA{}, false
			}
			if pct {
				v *= 255.0 / 100
			}
			vals[i] = v / 255
		}
		r, g, b = vals[0], vals[1], vals[2]
	case "hsl", "hsla":
		h, okH := angle(parts[0])
		sat, _, okS := number(parts[1])
		lit, _, okL := number(parts[2])
		if !okH || !okS || !okL {
			return color.NRGBA{}, false
		}
		r, g, b = hslToRGB(h, sat/100, lit/100)
	case "oklch", "oklab":
		l, pct, okL := number(parts[0])
		if !okL {
			return color.NRGBA{}, false
		}
		if pct {
			l /= 100
		}
		var a, bb float64
		if name == "oklch" {
			c, cPct, okC := number(parts[1])
			h, okH := angle(parts[2])
			if !okC || !okH {
				return color.NRGBA{}, false
			}
			if cPct {
				c *= 0.4 / 100
			}
			a, bb = c*math.Cos(h*math.Pi/180), c*math.Sin(h*math.Pi/180)
		} else {
			var okA, okB, aPct, bPct bool
			a, aPct, okA = number(parts[1])
			bb, bPct, okB = number(parts[2])
			if !okA || !okB {
				return color.NRGBA{}, false
			}
			if aPct {
				a *= 0.4 / 100
			}
			if bPct {
				bb *= 0.4 / 100
			}
		}
		r, g, b = oklabToRGB(l, a, bb)
	default:
		return color.NRGBA{}, false
	}
	return color.NRGBA{R: unit(r), G: unit(g), B: unit(b), A: unit(alpha)}, true
}

func parseHex(hex string) (color.NRGBA, bool) {
	switch len(hex) {
	case 3, 4:
		var d []byte
		for _, c := range hex {
			d = append(d, byte(c), byte(c))
		}
		hex = string(d)
	case 6, 8:
	default:
		return color.NRGBA{}, false
	}
	v, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return color.NRGBA{}, false
	}
	if len(hex) == 6 {
		v = v<<8 | 0xff
	}
	return color.NRGBA{R: uint8(v >> 24), G: uint8(v >> 16), B: uint8(v >> 8), A: uint8(v)}, true
}

// colorArgs splits the arguments of a colour function, with commas or
// spaces and an alpha after a slash or as a fourth argument.
func colorArgs(s string) (parts []string, alpha float64, ok bool) {
	alpha = 1
	main, slash, hasSlash := strings.Cut(s, "/")
	parts = strings.FieldsFunc(main, func(r rune) bool { return r == ',' || r == ' ' })
	a := ""
	switch {
	case hasSlash:
		a = strings.TrimSpace(slash)
	case len(parts) == 4:
		a, parts = parts[3], parts[:3]
	}
	if a != "" {
		v, pct, ok := number(a)
		if !ok {
			return nil, 0, false
		}
		if pct {
			v /= 100
		}
		alpha = v
	}
	return parts, alpha, true
}

// number reads a number, which may end in %.
func number(s string) (v float64, pct bool, ok bool) {
	s, pct = strings.CutSuffix(strings.TrimSpace(s), "%")
	v, err := strconv.ParseFloat(s, 64)
	return v, pct, err == nil && !math.IsNaN(v) && !math.IsInf(v, 0)
}

// angleUnits are the units of hues, in degrees; grad before rad, which it
// ends with.
var angleUnits = []struct {
	suffix string
	scale  float64
}{{"deg", 1}, {"turn", 360}, {"grad", 0.9}, {"rad", 180 / math.Pi}}

// angle reads a hue in degrees, which may say deg, turn, grad or rad.
func angle(s string) (float64, bool) {
	for _, u := range angleUnits {
		if v, ok := strings.CutSuffix(s, u.suffix); ok {
			f, err := strconv.ParseFloat(v, 64)
			return f * u.scale, err == nil
		}
	}
	f, err := strconv.ParseFloat(s, 64)
	return f, err == nil
}

func hslToRGB(h, s, l float64) (r, g, b float64) {
	h = math.Mod(math.Mod(h, 360)+360, 360) / 360
	s, l = clamp01(s), clamp01(l)
	if s == 0 {
		return l, l, l
	}
	q := l * (1 + s)
	if l >= 0.5 {
		q = l + s - l*s
	}
	p := 2*l - q
	hue := func(t float64) float64 {
		t = math.Mod(t+1, 1)
		switch {
		case t < 1.0/6:
			return p + (q-p)*6*t
		case t < 0.5:
			return q
		case t < 2.0/3:
			return p + (q-p)*(2.0/3-t)*6
		}
		return p
	}
	return hue(h + 1.0/3), hue(h), hue(h - 1.0/3)
}

// oklabToRGB converts a colour of Björn Ottosson's Oklab to sRGB.
func oklabToRGB(L, a, b float64) (float64, float64, float64) {
	l := L + 0.3963377774*a + 0.2158037573*b
	m := L - 0.1055613458*a - 0.0638541728*b
	s := L - 0.0894841775*a - 1.2914855480*b
	l, m, s = l*l*l, m*m*m, s*s*s
	gamma := func(c float64) float64 {
		if c <= 0.0031308 {
			return 12.92 * c
		}
		return 1.055*math.Pow(c, 1/2.4) - 0.055
	}
	return gamma(+4.0767416621*l - 3.3077115913*m + 0.2309699292*s),
		gamma(-1.2684380046*l + 2.6097574011*m - 0.3413193965*s),
		gamma(-0.0041960863*l - 0.7034186147*m + 1.7076147010*s)
}

// rgbToHSL is hslToRGB's inverse: c's hue in degrees, and its saturation
// and lightness from 0 to 1.
func rgbToHSL(c color.NRGBA) (h, s, l float64) {
	r, g, b := float64(c.R)/255, float64(c.G)/255, float64(c.B)/255
	hi, lo := max(r, g, b), min(r, g, b)
	l = (hi + lo) / 2
	d := hi - lo
	if d == 0 {
		return 0, 0, l
	}
	s = d / (1 - math.Abs(2*l-1))
	switch hi {
	case r:
		h = math.Mod((g-b)/d+6, 6)
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	return 60 * h, clamp01(s), l
}

// toned is c with its hue, its lightness brought between lo and hi and
// its saturation up to sat at least, unless c is a gray: the colour a
// cover lends a ground, which a near-black or a washed-out cover would
// leave unseen.
func toned(c color.NRGBA, lo, hi, sat float64) color.NRGBA {
	h, s, l := rgbToHSL(c)
	if s > 0.08 {
		s = max(s, sat)
	}
	r, g, b := hslToRGB(h, s, min(max(l, lo), hi))
	return color.NRGBA{R: unit(r), G: unit(g), B: unit(b), A: 255}
}

func clamp01(v float64) float64 { return min(max(v, 0), 1) }

func unit(v float64) uint8 { return uint8(math.Round(clamp01(v) * 255)) }

// mix mixes b into a by t, from 0 (a) to 1 (b), as CSS's color-mix in sRGB.
func mix(a, b color.NRGBA, t float64) color.NRGBA {
	f := func(x, y uint8) uint8 { return uint8(math.Round(float64(x) + (float64(y)-float64(x))*t)) }
	return color.NRGBA{R: f(a.R, b.R), G: f(a.G, b.G), B: f(a.B, b.B), A: f(a.A, b.A)}
}

// alpha returns c at an opacity of a, of its own.
func alpha(c color.NRGBA, a float64) color.NRGBA {
	c.A = uint8(math.Round(float64(c.A) * clamp01(a)))
	return c
}

// over composites c over an opaque ground.
func over(c, ground color.NRGBA) color.NRGBA {
	t := float64(c.A) / 255
	out := mix(ground, color.NRGBA{R: c.R, G: c.G, B: c.B, A: 255}, t)
	out.A = 255
	return out
}

// luminance is the relative luminance of c, as WCAG computes it.
func luminance(c color.NRGBA) float64 {
	lin := func(v uint8) float64 {
		f := float64(v) / 255
		if f <= 0.04045 {
			return f / 12.92
		}
		return math.Pow((f+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.R) + 0.7152*lin(c.G) + 0.0722*lin(c.B)
}

// inkOn picks black or white, whichever reads better on c.
func inkOn(c color.NRGBA) color.NRGBA {
	if luminance(c) > 0.179 {
		return color.NRGBA{A: 255}
	}
	return color.NRGBA{R: 255, G: 255, B: 255, A: 255}
}

// contrast is the contrast ratio of a and b, as WCAG computes it.
func contrast(a, b color.NRGBA) float64 {
	la, lb := luminance(a), luminance(b)
	return (max(la, lb) + 0.05) / (min(la, lb) + 0.05)
}

// readableOn shifts an accent toward white on a dark ground, or black on a
// light one, until it reads on it at 3:1, as large text and controls need,
// keeping its hue, as the web interface did.
func readableOn(accent, ground color.NRGBA) color.NRGBA {
	toward := color.NRGBA{A: 255}
	if luminance(ground) < 0.5 {
		toward = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	}
	c := accent
	c.A = 255
	for i := 0; i < 10 && contrast(c, ground) < 3; i++ {
		c = mix(c, toward, 0.15)
	}
	return c
}
