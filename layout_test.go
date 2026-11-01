package main

import (
	"testing"

	"github.com/fzn0x/mixmusic/internal/addons"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// themeWith is the theme base, as Mix, with the layout tokens given.
func themeWith(layout map[string]string) addons.Theme {
	return addons.Theme{ID: "test", Name: "Test", Scheme: "dark", Colors: fallbackTheme, Radius: "10px", Layout: layout}
}

// lookOf is the look of a theme, at the density given.
func lookOf(th addons.Theme, density settings.Density) look {
	s := settings.Default()
	s.FollowSystem, s.Theme, s.Density = false, th.ID, density
	return newLook(s, []addons.Theme{th}, true, "")
}

// TestLayoutDefaults checks that a theme without layout tokens has Mix's
// measures and styles, and the compact density's.
func TestLayoutDefaults(t *testing.T) {
	k := lookOf(themeWith(nil), settings.DensityComfortable)
	if k.row != 52 || k.rail != 248 || k.player != 92 || k.gutter != 40 || k.gap != 16 || k.cover != 172 || k.radiusSm != 6 {
		t.Errorf("measures: row %v, rail %v, player %v, gutter %v, gap %v, cover %v, small radius %v",
			k.row, k.rail, k.player, k.gutter, k.gap, k.cover, k.radiusSm)
	}
	if k.pill || k.tonal || k.cards || k.floating || k.headings != headingsLight || k.motion != motionStandard {
		t.Errorf("styles: %+v", k)
	}
	if k.selected != k.accent || k.selectedText != k.accentText {
		t.Error("what is chosen isn't flooded with the accent")
	}
	k = lookOf(themeWith(nil), settings.DensityCompact)
	if k.row != 40 || k.rail != 224 || k.player != 80 || k.gutter != 28 {
		t.Errorf("compact: row %v, rail %v, player %v, gutter %v", k.row, k.rail, k.player, k.gutter)
	}
}

// TestLayoutTokens checks that a theme's layout tokens set the look, and
// that the compact density takes off them what it takes off Mix's.
func TestLayoutTokens(t *testing.T) {
	th := themeWith(map[string]string{
		"radius-small": "4px", "shape": "pill", "row": "64px", "gutter": "2rem", "gap": "12px", "rail": "220px",
		"cover": "200px", "player-height": "76px", "player": "floating", "controls": "tonal", "headings": "bold",
		"cards": "filled", "motion": "expressive",
	})
	k := lookOf(th, settings.DensityComfortable)
	if k.row != 64 || k.rail != 220 || k.player != 76 || k.gutter != 32 || k.gap != 12 || k.cover != 200 || k.radiusSm != 4 {
		t.Errorf("measures: row %v, rail %v, player %v, gutter %v, gap %v, cover %v, small radius %v",
			k.row, k.rail, k.player, k.gutter, k.gap, k.cover, k.radiusSm)
	}
	if !k.pill || !k.tonal || !k.cards || !k.floating || k.headings != headingsBold || k.motion != motionExpressive {
		t.Errorf("styles: %+v", k)
	}
	if k.selected == k.accent {
		t.Error("tonal controls flood what is chosen with the accent")
	}
	k = lookOf(th, settings.DensityCompact)
	if k.row != 49 || k.rail != 199 || k.player != 66 || k.gutter != 22 {
		t.Errorf("compact: row %v, rail %v, player %v, gutter %v", k.row, k.rail, k.player, k.gutter)
	}
	th.Layout["row"] = "44px"
	if k = lookOf(th, settings.DensityCompact); k.row != 40 {
		t.Errorf("a compact row of 44px is %v, too short for its artwork", k.row)
	}
}

// TestWords checks that the light headings set the interface's words in
// lowercase, and the others as written.
func TestWords(t *testing.T) {
	k := lookOf(themeWith(nil), settings.DensityComfortable)
	if got := k.words("Liked songs on YouTube Music"); got != "liked songs on youtube music" {
		t.Errorf("light: %q", got)
	}
	k = lookOf(themeWith(map[string]string{"headings": "regular"}), settings.DensityComfortable)
	if got := k.words("Liked songs on YouTube Music"); got != "Liked songs on YouTube Music" {
		t.Errorf("regular: %q", got)
	}
}
