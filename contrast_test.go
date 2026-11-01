package main

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/settings"
)

// TestDesignLanguagesContrast checks that the Material and Crate themes
// keep every text role readable, light and dark: 4.5:1 for text, 3:1 for
// the marks of the services, with their own colours and with the accents
// of many artworks, of which the Material themes make all their colours.
func TestDesignLanguagesContrast(t *testing.T) {
	s := newTestShell(t)
	artworks := []string{"", "#ffe600", "#00c8ff", "#808080", "#5a3a1a", "#0a1a6e", "#ff0000", "#00ff66", "#f5f5f5", "#111111"}
	for _, id := range []string{"material", "material-dark", "crate", "crate-light"} {
		for _, art := range artworks {
			st := settings.Default()
			st.FollowSystem, st.Theme = false, id
			k := newLook(st, s.inv.Themes, true, art)
			ground := func(c ui.Color) ui.Color { return col(over(nrgba(c), nrgba(k.bg))) }
			for _, p := range []struct {
				role     string
				ink, bg  ui.Color
				minRatio float64
			}{
				{"text on the ground", k.text, k.bg, 4.5},
				{"text on surface", k.text, ground(k.surface), 4.5},
				{"text on surface-2", k.text, ground(k.surface2), 4.5},
				{"text on surface-3", k.text, ground(k.surface3), 4.5},
				{"muted text on the ground", k.muted, k.bg, 4.5},
				{"muted text on surface", k.muted, ground(k.surface), 4.5},
				{"muted text on surface-2", k.muted, ground(k.surface2), 4.5},
				{"the accent on the ground", k.accent, k.bg, 4.5},
				{"the accent on surface", k.accent, ground(k.surface), 4.5},
				{"ink on the accent", k.accentText, k.accent, 4.5},
				{"ink on what is chosen", k.selectedText, k.selected, 4.5},
				{"secondary ink on what is chosen", k.selectedSub, k.selected, 4.5},
				{"ink on tile 1", k.tileText, k.tile(0), 4.5},
				{"ink on tile 2", k.tileText, k.tile(1), 4.5},
				{"ink on tile 3", k.tileText, k.tile(2), 4.5},
				{"ink on tile 4", k.tileText, k.tile(3), 4.5},
				{"errors", k.danger, k.bg, 4.5},
				{"YouTube's mark", k.youtube, k.bg, 3},
				{"Spotify's mark", k.spotify, k.bg, 3},
			} {
				if r := contrast(nrgba(p.ink), nrgba(p.bg)); r < p.minRatio {
					t.Errorf("%s, artwork %q: %s at %.2f:1", id, art, p.role, r)
				}
			}
		}
	}
}
