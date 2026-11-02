package main

// The layout designs: the shapes, the density and the style of the parts
// of the interface, which Settings › Appearance chooses apart from the
// theme, whose colours and faces they wear.

import (
	"math"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/settings"
)

// headingStyle is how titles are set.
type headingStyle int

const (
	// headingsLight sets them in giant light lowercase words, Tiles'.
	headingsLight headingStyle = iota
	// headingsRegular sets them as written, at a regular weight and the
	// sizes of Material's type scale.
	headingsRegular
	// headingsBold sets them as written, heavy and tight.
	headingsBold
)

// motionStyle is how what the user presses and chooses moves.
type motionStyle int

const (
	// motionStandard moves nothing on its own: Tiles' presses and hovers
	// show at once.
	motionStandard motionStyle = iota
	// motionSmooth moves them on Material's standard springs, which hardly
	// overshoot.
	motionSmooth
	// motionExpressive moves them on Material's expressive springs, which
	// overshoot a little, and morphs the shapes of the buttons pressed.
	motionExpressive
)

// floatMargin is the room around what floats, as a floating player bar
// and the side panel beside it.
const floatMargin = 12

// layoutDesign is a layout design's measures and styles. pill rounds
// buttons, chips and the search box fully; tonal fills controls rather
// than outlining them; cards sets covers in filled cards; floating floats
// the player bar over the window's foot.
type layoutDesign struct {
	// radius and small are the corners of what is large, as tiles and
	// dialogs, and of what is small, as rows and covers. Tiles has the
	// theme's radius, and 0.6 of it.
	radius, small                         float32
	row, rail, player, gutter, gap, cover float32
	pill, tonal, cards, floating          bool
	headings                              headingStyle
	motion                                motionStyle
}

// layoutDesigns are the layout designs, by the setting's values.
var layoutDesigns = map[settings.Layout]layoutDesign{
	settings.LayoutTiles: {row: 52, rail: 248, player: 92, gutter: 40, gap: 16, cover: 172},
	// After Material 3 Expressive: its contrast of shapes, 28px on tiles,
	// dialogs and the player bar and 12px on rows and covers, its type
	// scale and its springs.
	settings.LayoutExpressive: {radius: 28, small: 12, row: 64, rail: 256, player: 80, gutter: 32, gap: 16, cover: 176,
		pill: true, tonal: true, cards: true, floating: true, headings: headingsRegular, motion: motionExpressive},
	// Dense, with big covers, heavy headings and springs that hardly
	// overshoot.
	settings.LayoutCrate: {radius: 8, small: 4, row: 48, rail: 220, player: 76, gutter: 24, gap: 12, cover: 196,
		tonal: true, cards: true, headings: headingsBold, motion: motionSmooth},
	// Expressive's shapes and springs on Crate's density, covers and
	// headings.
	settings.LayoutBlend: {radius: 20, small: 10, row: 56, rail: 232, player: 80, gutter: 28, gap: 14, cover: 188,
		pill: true, tonal: true, cards: true, floating: true, headings: headingsBold, motion: motionExpressive},
}

// designOf is the layout design the setting l names, Blend's for one
// unknown.
func designOf(l settings.Layout) layoutDesign {
	if d, ok := layoutDesigns[l]; ok {
		return d
	}
	return layoutDesigns[settings.LayoutBlend]
}

// layoutOf sets the layout design d in k, scaled by the density; k.radius
// is the theme's.
func (k *look) layoutOf(d layoutDesign) {
	k.radiusSm = k.radius * 0.6
	if d.radius > 0 {
		k.radius, k.radiusSm = d.radius, d.small
	}
	k.pill, k.tonal, k.cards, k.floating = d.pill, d.tonal, d.cards, d.floating
	k.headings, k.motion = d.headings, d.motion
	k.row, k.rail, k.panel, k.player, k.gutter = d.row, d.rail, 340, d.player, d.gutter
	k.gap, k.cover = d.gap, d.cover
	if k.compact {
		// Compact takes off a design's measures what it takes off Tiles':
		// 52, 248, 92 and 40 become 40, 224, 80 and 28. A row keeps room
		// for its artwork.
		scale := func(v, from, to float32) float32 { return float32(math.Round(float64(v * to / from))) }
		k.row = max(40, scale(k.row, 52, 40))
		k.rail, k.player, k.gutter = scale(k.rail, 248, 224), scale(k.player, 92, 80), scale(k.gutter, 40, 28)
	}
}

// round is the corner radius of a control of height h: a pill's, or the
// small radius.
func (k *look) round(h float32) float32 {
	if k.pill {
		return h / 2
	}
	return k.radiusSm
}

// words are the interface's own words, and the titles, as the headings
// set them: lowercase in Tiles' light style, as written otherwise. The
// lowercase words are kept, so that a frame lowers none twice: the
// interface's own are a few dozen.
func (k *look) words(s string) string {
	if k.headings != headingsLight {
		return s
	}
	if l, ok := k.lowered[s]; ok {
		return l
	}
	l := strings.ToLower(s)
	if l == s {
		return s // lowered without a copy
	}
	if k.lowered == nil || len(k.lowered) >= 256 {
		k.lowered = map[string]string{}
	}
	k.lowered[s] = l
	return l
}

// sentence is a label of the interface's, as "sign out": as it is in the
// light style, whose words are lowercase, and with its first letter in
// capitals otherwise. Those are kept, as the lowered words are.
func (k *look) sentence(s string) string {
	if k.headings == headingsLight || s == "" {
		return s
	}
	if c, ok := k.capped[s]; ok {
		return c
	}
	r, n := utf8.DecodeRuneInString(s)
	up := unicode.ToUpper(r)
	if up == r {
		return s // capitalised without a copy
	}
	c := string(up) + s[n:]
	if k.capped == nil || len(k.capped) >= 256 {
		k.capped = map[string]string{}
	}
	k.capped[s] = c
	return c
}

// labelWeight sets the weight of the labels of buttons and chips: the
// face's regular one in the light style, as before, and heavier in the
// others, as Material's labels are.
func (k *look) labelWeight(t ui.Element) {
	switch k.headings {
	case headingsRegular:
		t.FontWeight(500)
	case headingsBold:
		t.FontWeight(600)
	}
}

// displayWeight is the weight of the largest words, as the pages' titles;
// titleWeight that of the sections' titles, the tiles' and the panels';
// and leadWeight that of the sentences set large under them.
func (k *look) displayWeight() int {
	switch k.headings {
	case headingsRegular:
		return 400
	case headingsBold:
		return 800
	}
	return 300
}

func (k *look) titleWeight() int {
	switch k.headings {
	case headingsRegular:
		return 500
	case headingsBold:
		return 800
	}
	return 300
}

func (k *look) leadWeight() int {
	if k.headings == headingsLight {
		return 300
	}
	return 400
}

// tracking is the letter spacing of words of size set at Tiles' tracking
// of em: theirs in the light style, none in the regular one, whose sizes
// Material's type scale tracks, and at most -0.01em in the bold one.
func (k *look) tracking(em, size float32) float32 {
	switch k.headings {
	case headingsRegular:
		return 0
	case headingsBold:
		return max(em, -0.01) * size
	}
	return em * size
}

// spring is the motion of what moves on its own, in the theme's style: a
// spatial spring, for what moves or changes size, or one of effects, for
// what changes colour or fades, which never overshoots.
func (k *look) spring(spatial bool) ui.Animation {
	switch {
	case k.motion == motionExpressive && spatial:
		// Material's expressive default spatial spring: a damping ratio of
		// 0.8 at a stiffness of 380.
		return springOf(380, 0.8)
	case k.motion == motionExpressive:
		return springOf(1600, 1)
	case spatial:
		// Material's standard default spatial spring.
		return springOf(700, 0.9)
	}
	return springOf(1600, 1)
}

// springOf is the spring of stiffness and damping ratio, of a mass of 1.
func springOf(stiffness, ratio float64) ui.Animation {
	return ui.Animation{Spring: ui.SpringOptions{Stiffness: stiffness, Damping: 2 * ratio * math.Sqrt(stiffness)}}
}

// moves reports whether presses, hovers and choices move on springs.
func (k *look) moves() bool { return k.motion != motionStandard }

// scrimRadius rounds the scrim over a row's artwork as the artwork is, in
// the themes of tonal controls; others keep the square one they had.
func (k *look) scrimRadius() float32 {
	if k.tonal {
		return k.radiusSm
	}
	return 0
}

// bigSize is the size of the large words of an empty page, as "nothing
// playing": Tiles' 3.5rem, or Material's display small.
func (k *look) bigSize() float32 {
	if k.headings == headingsLight {
		return k.size(3.5)
	}
	return k.size(2.25)
}
