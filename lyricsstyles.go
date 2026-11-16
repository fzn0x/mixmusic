package main

import (
	"math"
	"slices"

	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/locales"
)

// lyricStyle is how the full-screen lyrics (lyricsstage.go) look in one
// style: what their lines are set in, how those away from the line sung
// fade and blur, and the glow of the words sung. The theme's colours don't
// reach them: the stage sets its own ink over the cover. A style is data:
// the stage draws all of them the same way.
type lyricStyle struct {
	id settings.LyricStyle
	// family, weight and size set the line sung now, size in rem, as
	// look.size takes it, and leading its line height, a multiple of its
	// size. There is no letter spacing: the words sung are painted from a
	// layout of their own (lyricswords.go), which has none.
	family  string
	weight  int
	size    float32
	leading float32
	// dim is the opacity of the lines next to the one sung, fade how much
	// each line farther away takes off it, and floor the least they keep.
	dim, fade, floor float32
	// blur is the blur each line of distance from the one sung takes on, in
	// DIPs, up to maxBlur.
	blur, maxBlur float32
	// glow is the blur of the glow behind the words sung, in DIPs; 0 for
	// none.
	glow float32
	// cover is the corner radius of the cover, in DIPs, and scrim how dark
	// the blurred cover is under the lyrics, from 0 to 1: dark enough that
	// the stage's ink reads at 4.5:1 on any cover, a white one included,
	// which a test checks.
	cover, scrim float32
}

// lyricStyles are the styles, in the order of settings.LyricStyles: a test
// keeps the two the same.
var lyricStyles = []lyricStyle{
	{
		id:     settings.LyricStyleSpotlight,
		family: face, weight: 700, size: 3.2, leading: 1.1,
		dim: 0.38, fade: 0.06, floor: 0.14, blur: 2.5, maxBlur: 10, cover: 16, scrim: 0.64,
	},
	{
		id:     settings.LyricStyleGlow,
		family: faceRound, weight: 700, size: 2.1, leading: 1.25,
		dim: 0.42, fade: 0.06, floor: 0.16, blur: 1.5, maxBlur: 6, glow: 9, cover: 12, scrim: 0.62,
	},
	{
		id:     settings.LyricStyleFocus,
		family: faceMaterial, weight: 700, size: 2, leading: 1.2,
		dim: 0.45, fade: 0.07, floor: 0.16, blur: 1, maxBlur: 5, cover: 6, scrim: 0.64,
	},
	{
		id:     settings.LyricStyleSimple,
		family: face, weight: 600, size: 1.9, leading: 1.3,
		dim: 0.4, fade: 0.08, floor: 0.2, cover: 4, scrim: 0.62,
	},
	{
		id:     settings.LyricStyleBold,
		family: faceArchivo, weight: 800, size: 2.9, leading: 1.02,
		dim: 0.32, fade: 0.05, floor: 0.12, cover: 0, scrim: 0.68,
	},
	{
		id:     settings.LyricStyleCalm,
		family: faceMaterial, weight: 500, size: 1.5, leading: 1.4,
		dim: 0.5, fade: 0.05, floor: 0.25, blur: 0.5, maxBlur: 2, cover: 14, scrim: 0.62,
	},
}

// title is the name of the style, and about says what it looks like, in
// the language shown: what Settings and the command palette call it.
func (st lyricStyle) title() string {
	switch st.id {
	case settings.LyricStyleGlow:
		return locales.LyricsStyleGlow()
	case settings.LyricStyleFocus:
		return locales.LyricsStyleFocus()
	case settings.LyricStyleSimple:
		return locales.LyricsStyleSimple()
	case settings.LyricStyleBold:
		return locales.LyricsStyleBold()
	case settings.LyricStyleCalm:
		return locales.LyricsStyleCalm()
	}
	return locales.LyricsStyleSpotlight()
}

func (st lyricStyle) about() string {
	switch st.id {
	case settings.LyricStyleGlow:
		return locales.LyricsStyleGlowAbout()
	case settings.LyricStyleFocus:
		return locales.LyricsStyleFocusAbout()
	case settings.LyricStyleSimple:
		return locales.LyricsStyleSimpleAbout()
	case settings.LyricStyleBold:
		return locales.LyricsStyleBoldAbout()
	case settings.LyricStyleCalm:
		return locales.LyricsStyleCalmAbout()
	}
	return locales.LyricsStyleSpotlightAbout()
}

// lyricStyleOf is the style with the id, or the first, for one it doesn't
// know.
func lyricStyleOf(id settings.LyricStyle) lyricStyle {
	if i := slices.IndexFunc(lyricStyles, func(st lyricStyle) bool { return st.id == id }); i >= 0 {
		return lyricStyles[i]
	}
	return lyricStyles[0]
}

// opacity is how opaque the line is that lies dist lines from the one sung:
// whole for the line sung, the style's dim for its neighbours, and less
// farther away, down to the floor.
func (st lyricStyle) opacity(dist int) float32 {
	if dist <= 0 {
		return 1
	}
	return max(st.floor, st.dim-st.fade*float32(dist-1))
}

// blurAt is the blur of the line dist lines from the one sung, in whole
// DIPs: a few levels, which the lines keep as the song goes on, so that
// each need not be drawn again.
func (st lyricStyle) blurAt(dist int) float32 {
	if dist <= 0 {
		return 0
	}
	return float32(math.Round(float64(min(st.blur*float32(dist), st.maxBlur))))
}
