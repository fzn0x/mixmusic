package main

// A theme's layout tokens (docs/themes.md): its shapes, its density and the
// style of its parts, which a design language sets beside its colours.
// Left out, each is Mix's own, so that a theme without them looks as
// themes did before them.

import (
	"math"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/addons"
)

// headingStyle is how titles are set.
type headingStyle int

const (
	// headingsLight sets them in giant light lowercase words, Mix's.
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
	// motionStandard moves nothing on its own: Mix's presses and hovers
	// show at once.
	motionStandard motionStyle = iota
	// motionSmooth moves them on springs that settle without overshooting.
	motionSmooth
	// motionExpressive moves them on Material's expressive springs, which
	// overshoot a little, and morphs the shapes of the buttons pressed.
	motionExpressive
)

// layoutOf reads the layout tokens of th into k, scaled by the density.
func (k *look) layoutOf(th addons.Theme) {
	px := func(key string, def float32) float32 {
		if v, ok := addons.Pixels(th.Layout[key]); ok {
			return float32(v)
		}
		return def
	}
	style := func(key string, values ...string) int {
		for i, v := range values {
			if th.Layout[key] == v {
				return i
			}
		}
		return 0
	}
	k.radiusSm = px("radius-small", k.radius*0.6)
	k.pill = th.Layout["shape"] == "pill"
	k.tonal = th.Layout["controls"] == "tonal"
	k.cards = th.Layout["cards"] == "filled"
	k.floating = th.Layout["player"] == "floating"
	k.headings = headingStyle(style("headings", "light", "regular", "bold"))
	k.motion = motionStyle(style("motion", "standard", "smooth", "expressive"))
	k.row, k.rail, k.panel, k.player, k.gutter = px("row", 52), px("rail", 248), 340, px("player-height", 92), px("gutter", 40)
	k.gap, k.cover = px("gap", 16), px("cover", 172)
	if k.compact {
		// Compact takes off a theme's measures what it takes off Mix's:
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
// set them: lowercase in Mix's light style, as written otherwise.
func (k *look) words(s string) string {
	if k.headings == headingsLight {
		return strings.ToLower(s)
	}
	return s
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

// tracking is the letter spacing of words of size set at Mix's tracking
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
