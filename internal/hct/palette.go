package hct

import "math"

// Palette is a tonal palette: the colours of one hue and chroma, from
// black, tone 0, to white, tone 100.
type Palette struct {
	Hue, Chroma float64
}

// PaletteOf is the palette of c's hue and chroma.
func PaletteOf(c ARGB) Palette {
	h := FromARGB(c)
	return Palette{h.Hue, h.Chroma}
}

// Tone is the palette's colour of tone t.
func (p Palette) Tone(t float64) ARGB {
	// Yellows lack chroma at 99: the average of 98 and 100 keeps it from
	// turning green.
	if t == 99 && isYellow(p.Hue) {
		a, b := p.Tone(98), p.Tone(100)
		ar, ag, ab := a.RGB()
		br, bg, bb := b.RGB()
		avg := func(x, y uint8) uint8 { return uint8(math.Round((float64(x) + float64(y)) / 2)) }
		return RGB(avg(ar, br), avg(ag, bg), avg(ab, bb))
	}
	return solve(p.Hue, p.Chroma, t)
}

func isYellow(hue float64) bool { return hue >= 105 && hue < 125 }
