package main

// The Material themes' colours: the scheme of Material's core palettes of
// a seed colour, made in HCT (internal/hct). The seed is the artwork's colour
// while it plays, as Material's dynamic colour takes the wallpaper's, the
// accent the user chose, or MixMusic's pink.

import (
	"image/color"
	"sync"

	"github.com/fzn0x/mixmusic/internal/hct"
)

// materialSeed is the seed of the Material themes' own scheme: the pink of
// MixMusic's mark.
var materialSeed = color.NRGBA{R: 0xe6, G: 0x00, B: 0x7e, A: 0xff}

// materialColors are the colours of a Material scheme: the theme's
// tokens, and the roles the look takes beside them.
type materialColors struct {
	tokens map[string]color.NRGBA
	// selected fills what is chosen, as the current page: the secondary
	// container, and selectedText inks it.
	selected, selectedText color.NRGBA
}

var materials = struct {
	sync.Mutex
	schemes map[materialKey]*materialColors
}{schemes: map[materialKey]*materialColors{}}

type materialKey struct {
	seed hct.ARGB
	dark bool
}

// materialOf is the scheme of seed, light or dark, made once: the
// settings' theme cards make the look of each theme at each frame.
func materialOf(seed color.NRGBA, dark bool) *materialColors {
	key := materialKey{hct.RGB(seed.R, seed.G, seed.B), dark}
	materials.Lock()
	defer materials.Unlock()
	if m := materials.schemes[key]; m != nil {
		return m
	}
	// A seed for each track that played: a few are kept, not all.
	if len(materials.schemes) >= 32 {
		clear(materials.schemes)
	}
	m := makeMaterial(key.seed, dark)
	materials.schemes[key] = m
	return m
}

// token is the colour token name of the scheme m, if m is one.
func (m *materialColors) token(name string) (color.NRGBA, bool) {
	if m == nil {
		return color.NRGBA{}, false
	}
	c, ok := m.tokens[name]
	return c, ok
}

func makeMaterial(seed hct.ARGB, dark bool) *materialColors {
	p := hct.Core(seed)
	s := p.Scheme(dark)
	c := func(a hct.ARGB) color.NRGBA {
		r, g, b := a.RGB()
		return color.NRGBA{R: r, G: g, B: b, A: 255}
	}
	tone := func(light, dk float64) float64 {
		if dark {
			return dk
		}
		return light
	}
	// The marks of the services and success keep their hues, at the tones
	// of the scheme's accents.
	fixed := func(hue, chroma float64) color.NRGBA { return c(hct.From(hue, chroma, tone(42, 75)).ARGB()) }
	return &materialColors{
		tokens: map[string]color.NRGBA{
			"bg": c(s.Surface), "surface": c(s.SurfaceContainer), "surface-2": c(s.SurfaceContainerHigh),
			"surface-3": c(s.SurfaceContainerHighest), "border": c(s.OutlineVariant),
			"text": c(s.OnSurface), "text-muted": c(s.OnSurfaceVariant),
			"accent": c(s.Primary), "accent-text": c(s.OnPrimary),
			"danger": c(s.Error), "success": fixed(145, 48),
			"youtube": fixed(27, 80), "spotify": fixed(142, 60),
			// The tiles: the primary's and the tertiary's containers, and
			// both a step stronger. A light seed's containers lack chroma:
			// the secondary's would match the primary's.
			"tile-1": c(s.PrimaryContainer), "tile-2": c(s.TertiaryContainer), "tile-3": c(s.InversePrimary),
			"tile-4": c(p.Tertiary.Tone(tone(80, 40))), "tile-text": c(s.OnSurface),
		},
		selected: c(s.SecondaryContainer), selectedText: c(s.OnSecondaryContainer),
	}
}
