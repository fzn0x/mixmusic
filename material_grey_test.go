package main

import (
	"fmt"
	"image/color"
	"testing"

	"github.com/fzn0x/mixmusic/internal/colorfile"
	"github.com/fzn0x/mixmusic/internal/hct"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// TestMaterialGreyCovers checks that a Material theme following a black,
// white, grey or near-grey cover keeps a grey (or calm) window, not a
// cyan or pink one (#573).
func TestMaterialGreyCovers(t *testing.T) {
	s := newTestShell(t)
	th := builtinTheme(t, s, "material-dark")
	chroma := func(c color.NRGBA) float64 { return hct.FromARGB(hct.RGB(c.R, c.G, c.B)).Chroma }
	for _, c := range []struct {
		name, cover string
		maxAccent   float64
		minAccent   float64
	}{
		{"black", "#0a0a0a", 3, 0},
		{"white", "#f5f5f5", 3, 0},
		{"grey", "#808080", 3, 0},
		{"warm grey", "#121010", 3, 0},
		{"near grey", "#7a7a85", 20, 8},
	} {
		st := settings.Default()
		st.FollowSystem, st.Theme, st.DynamicAccent = false, th.ID, true
		k := newLook(st, s.inv.Themes, true, c.cover, colorfile.Colors{})
		acc, bg := nrgba(k.accent), nrgba(k.bg)
		t.Logf("%s %s: accent %s (chroma %.1f), ground %s (chroma %.1f)", c.name, c.cover,
			fmt.Sprintf("#%02x%02x%02x", acc.R, acc.G, acc.B), chroma(acc), fmt.Sprintf("#%02x%02x%02x", bg.R, bg.G, bg.B), chroma(bg))
		if ca := chroma(acc); ca > c.maxAccent || ca < c.minAccent {
			t.Errorf("%s cover: accent chroma %.1f, want %v to %v", c.name, ca, c.minAccent, c.maxAccent)
		}
		if cb := chroma(bg); cb > 5 {
			t.Errorf("%s cover: ground chroma %.1f", c.name, cb)
		}
	}
}
