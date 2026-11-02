package main

// The colours of a colour file, which pywal or matugen make of the
// wallpaper, over the theme's tokens.

import (
	"image/color"
	"math"
	"slices"
	"strconv"

	"github.com/fzn0x/mixmusic/internal/colorfile"
)

// palette is the colours of the theme's tokens.
type palette struct {
	bg, surface, surface2, surface3, border color.NRGBA
	text, muted, accent, accentText         color.NRGBA
	danger, success, youtube, spotify       color.NRGBA
	tiles                                   [4]color.NRGBA
	tileText                                color.NRGBA
}

// paletteOf is the palette of the colours tok gives the tokens.
func paletteOf(tok func(name string) color.NRGBA) palette {
	p := palette{
		bg: tok("bg"), surface: tok("surface"), surface2: tok("surface-2"), surface3: tok("surface-3"), border: tok("border"),
		text: tok("text"), muted: tok("text-muted"), accent: tok("accent"), accentText: tok("accent-text"),
		danger: tok("danger"), success: tok("success"), youtube: tok("youtube"), spotify: tok("spotify"),
		tileText: tok("tile-text"),
	}
	for i := range p.tiles {
		p.tiles[i] = tok("tile-" + strconv.Itoa(i+1))
	}
	return p
}

var (
	black = color.NRGBA{A: 255}
	white = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
)

// keepReadable keeps a colour file's palette readable, as docs/themes.md
// asks of themes: text, muted text and danger at 4.5:1 on the ground, the
// accent, success and the marks at 3:1, and the ink on the accent and on
// every tile at 4.5:1. With white tiles, as Mix's option has them, the
// ink stays white and the tiles darken under it.
func (p *palette) keepReadable(whiteTiles bool) {
	bg := p.bg
	p.text = readableAt(p.text, bg, 4.5)
	p.muted = readableAt(p.muted, bg, 4.5)
	p.danger = readableAt(p.danger, bg, 4.5)
	p.accent = readableAt(p.accent, bg, 3)
	p.success, p.youtube, p.spotify = readableAt(p.success, bg, 3), readableAt(p.youtube, bg, 3), readableAt(p.spotify, bg, 3)
	if ink := over(p.accentText, p.accent); contrast(ink, p.accent) >= 4.5 {
		p.accentText = ink
	} else {
		p.accentText = inkOn(p.accent)
	}
	least := func(ink color.NRGBA) float64 {
		l := math.Inf(1)
		for _, t := range p.tiles {
			l = min(l, contrast(ink, t))
		}
		return l
	}
	for i, t := range p.tiles {
		p.tiles[i] = over(t, bg)
	}
	ink := over(p.tileText, bg)
	switch {
	case whiteTiles:
		ink = white
	case least(ink) < 4.5 && least(black) >= least(white):
		ink = black
	case least(ink) < 4.5:
		ink = white
	}
	p.tileText = ink
	for i, t := range p.tiles {
		p.tiles[i] = readableAt(t, ink, 4.5)
	}
}

// glassOf is a colour at an opacity of a at least that shows as c over the
// ground: a surface of the theme's glass, in a file's colour.
func glassOf(c, ground color.NRGBA, a uint8) color.NRGBA {
	t := float64(a) / 255
	chans := [3][2]uint8{{c.R, ground.R}, {c.G, ground.G}, {c.B, ground.B}}
	for _, ch := range chans {
		d := float64(ch[0]) - float64(ch[1])
		switch {
		case d > 0:
			t = max(t, d/(255-float64(ch[1])))
		case d < 0:
			t = max(t, -d/float64(ch[1]))
		}
	}
	t = min(t, 1)
	if t == 0 {
		return color.NRGBA{}
	}
	at := func(i int) uint8 {
		return uint8(math.Round(clamp01((float64(chans[i][1])+(float64(chans[i][0])-float64(chans[i][1]))/t)/255) * 255))
	}
	return color.NRGBA{R: at(0), G: at(1), B: at(2), A: uint8(math.Round(t * 255))}
}

// fileTokens are the colours a colour file gives the tokens in a mode,
// "dark" or "light", the scheme of the theme worn, and whether they are
// dark: a file with a set of each gives the set of that mode, one with a
// single set gives it in either, and its ground tells which it is unless
// it says. The tokens it gives no colour leave the theme's.
func fileTokens(f colorfile.Colors, mode string) (map[string]color.NRGBA, bool) {
	set := f.Mode(mode)
	if set == nil {
		return nil, false
	}
	var toks map[string]color.NRGBA
	switch f.Format {
	case colorfile.FormatPywal:
		toks = pywalTokens(set)
	case colorfile.FormatMaterial:
		other := "light"
		if mode == "light" {
			other = "dark"
		}
		toks = materialTokens(set, f.Mode(other))
	default:
		toks = map[string]color.NRGBA{}
		for name, v := range set {
			if c, ok := parseColor(v); ok {
				toks[name] = c
			}
		}
	}
	bg, ok := toks["bg"]
	if !ok {
		return nil, false
	}
	bg.A = 255
	toks["bg"] = bg
	switch {
	case !f.OneMode():
		return toks, mode == "dark"
	case f.Scheme != "":
		return toks, f.Scheme == "dark"
	}
	return toks, luminance(bg) <= 0.179
}

// fileMode tells, of a file with a single set of colours, whether they are
// dark: such a file decides whether MixMusic is light or dark.
func fileMode(f colorfile.Colors) (dark, one bool) {
	if !f.OneMode() {
		return false, false
	}
	toks, dark := fileTokens(f, "dark")
	return dark, toks != nil
}

// pywalTokens are the tokens of pywal's colours: its background and
// foreground, the surfaces between them, the most vivid of color1 to
// color6 as the accent and the tiles, and color8 as muted text where it
// reads, as the issue's table has them.
func pywalTokens(set map[string]string) map[string]color.NRGBA {
	get := func(name string) (color.NRGBA, bool) { return parseColor(set[name]) }
	bg, ok := get("background")
	fg, okFG := get("foreground")
	if !ok || !okFG {
		return nil
	}
	bg.A, fg.A = 255, 255
	toks := map[string]color.NRGBA{
		"bg": bg, "text": fg,
		"surface": mix(bg, fg, 0.045), "surface-2": mix(bg, fg, 0.09), "surface-3": mix(bg, fg, 0.15),
		"border": alpha(fg, 0.16), "text-muted": mix(fg, bg, 0.36),
	}
	if c8, ok := get("color8"); ok && contrast(over(c8, bg), bg) >= 4.5 {
		toks["text-muted"] = over(c8, bg)
	}
	var vivid []color.NRGBA
	for i := 1; i <= 6; i++ {
		if c, ok := get("color" + strconv.Itoa(i)); ok && !slices.Contains(vivid, over(c, bg)) {
			vivid = append(vivid, over(c, bg))
		}
	}
	if len(vivid) == 0 {
		return toks
	}
	slices.SortStableFunc(vivid, func(a, b color.NRGBA) int {
		va, vb := vividness(a), vividness(b)
		switch {
		case va > vb:
			return -1
		case va < vb:
			return 1
		}
		return 0
	})
	toks["accent"], toks["accent-text"] = vivid[0], inkOn(vivid[0])
	for i := range 4 {
		toks["tile-"+strconv.Itoa(i+1)] = vivid[i%len(vivid)]
	}
	toks["tile-text"] = black // keepReadable picks the ink that reads on all four
	return toks
}

// vividness is how vivid c is, as the artwork's accent rates it, s²·v:
// greys and near blacks come last.
func vividness(c color.NRGBA) float64 {
	r, g, b := float64(c.R)/255, float64(c.G)/255, float64(c.B)/255
	v := max(r, g, b)
	if v == 0 {
		return -1
	}
	s := (v - min(r, g, b)) / v
	w := s * s * v
	if s < 0.25 || v < 0.2 {
		w--
	}
	return w
}

// materialTokens are the tokens of matugen's Material roles, other being
// the roles of the other mode: matugen 2.4 gives source_color in the light
// one only.
func materialTokens(set, other map[string]string) map[string]color.NRGBA {
	get := func(names ...string) (color.NRGBA, bool) {
		for _, name := range names {
			if c, ok := parseColor(set[name]); ok {
				return c, true
			}
		}
		return color.NRGBA{}, false
	}
	bg, ok := get("surface", "background")
	primary, okP := get("primary")
	if !ok || !okP {
		return nil
	}
	bg.A = 255
	text, ok := get("on_surface", "on_background")
	if !ok {
		text = inkOn(bg)
	}
	toks := map[string]color.NRGBA{"bg": bg, "text": text, "accent": primary, "tile-1": primary}
	put := func(token string, fallback color.NRGBA, roles ...string) {
		if c, ok := get(roles...); ok {
			toks[token] = c
		} else {
			toks[token] = fallback
		}
	}
	put("surface", mix(bg, text, 0.045), "surface_container_low")
	put("surface-2", mix(bg, text, 0.09), "surface_container_high", "surface_container")
	put("surface-3", mix(bg, text, 0.15), "surface_container_highest")
	put("border", alpha(text, 0.16), "outline_variant", "outline")
	put("text-muted", mix(text, bg, 0.36), "on_surface_variant")
	put("accent-text", inkOn(primary), "on_primary")
	put("tile-text", inkOn(primary), "on_primary")
	put("tile-2", primary, "tertiary")
	put("tile-3", toks["tile-2"], "secondary")
	if c, ok := get("error"); ok {
		toks["danger"] = c
	}
	// The fourth tile is the wallpaper's own colour, unless it is all but
	// the first's: then it falls between the first two.
	source, ok := get("source_color")
	if !ok {
		source, ok = parseColor(other["source_color"])
	}
	if !ok || oklabDistance(source, primary) < 0.04 {
		source = oklabMix(primary, toks["tile-2"], 0.5)
	}
	toks["tile-4"] = source
	return toks
}

// rgbToOklab converts an sRGB colour to Björn Ottosson's Oklab.
func rgbToOklab(c color.NRGBA) (L, a, b float64) {
	lin := func(v uint8) float64 {
		f := float64(v) / 255
		if f <= 0.04045 {
			return f / 12.92
		}
		return math.Pow((f+0.055)/1.055, 2.4)
	}
	r, g, bl := lin(c.R), lin(c.G), lin(c.B)
	l := math.Cbrt(0.4122214708*r + 0.5363325363*g + 0.0514459929*bl)
	m := math.Cbrt(0.2119034982*r + 0.6806995451*g + 0.1073969566*bl)
	s := math.Cbrt(0.0883024619*r + 0.2817188376*g + 0.6299787005*bl)
	return 0.2104542553*l + 0.7936177850*m - 0.0040720468*s,
		1.9779984951*l - 2.4285922050*m + 0.4505937099*s,
		0.0259040371*l + 0.7827717662*m - 0.8086757660*s
}

// oklabDistance is how far apart a and b are in Oklab.
func oklabDistance(a, b color.NRGBA) float64 {
	l1, a1, b1 := rgbToOklab(a)
	l2, a2, b2 := rgbToOklab(b)
	return math.Sqrt((l1-l2)*(l1-l2) + (a1-a2)*(a1-a2) + (b1-b2)*(b1-b2))
}

// oklabMix mixes b into a by t in Oklab.
func oklabMix(a, b color.NRGBA, t float64) color.NRGBA {
	l1, a1, b1 := rgbToOklab(a)
	l2, a2, b2 := rgbToOklab(b)
	r, g, bl := oklabToRGB(l1+(l2-l1)*t, a1+(a2-a1)*t, b1+(b2-b1)*t)
	return color.NRGBA{R: unit(r), G: unit(g), B: unit(bl), A: 255}
}
