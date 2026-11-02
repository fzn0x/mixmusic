package main

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/colorfile"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// TestDesignLanguagesContrast checks that the Material and Walnut themes
// keep every text role readable, light and dark, in every layout design:
// 4.5:1 for text, 3:1 for the marks of the services and for the accent in
// Tiles, with their own colours and with the accents of many artworks, of
// which the Material themes make all their colours.
func TestDesignLanguagesContrast(t *testing.T) {
	s := newTestShell(t)
	for _, id := range []string{"material", "material-dark", "walnut", "walnut-light"} {
		for _, layout := range settings.Layouts {
			for _, art := range contrastArtworks {
				k := contrastLook(s, id, layout, art)
				ground := func(c ui.Color) ui.Color { return col(over(nrgba(c), nrgba(k.bg))) }
				// Tiles takes an accent at 3:1 on the ground, as it does
				// in every theme, and sets it large.
				accent := []contrastRole{{"the accent on the ground", k.accent, k.bg, 3}}
				if designOf(layout).tonal {
					accent = []contrastRole{
						{"the accent on the ground", k.accent, k.bg, 4.5},
						{"the accent on surface", k.accent, ground(k.surface), 4.5},
					}
				}
				checkContrast(t, fmt.Sprintf("%s, %s, artwork %q", id, layout, art), append(accent, []contrastRole{
					{"text on the ground", k.text, k.bg, 4.5},
					{"text on surface", k.text, ground(k.surface), 4.5},
					{"text on surface-2", k.text, ground(k.surface2), 4.5},
					{"text on surface-3", k.text, ground(k.surface3), 4.5},
					{"muted text on the ground", k.muted, k.bg, 4.5},
					{"muted text on surface", k.muted, ground(k.surface), 4.5},
					{"muted text on surface-2", k.muted, ground(k.surface2), 4.5},
					{"ink on the accent", k.accentText, k.accent, 4.5},
					{"ink on what is chosen", k.selectedText, k.selected, 4.5},
					{"secondary ink on what is chosen", k.selectedSub, k.selected, 4.5},
					{"ink on a tonal button", k.tonalInk, k.tonalFill, 4.5},
					{"ink on tile 1", k.tileText, k.tile(0), 4.5},
					{"ink on tile 2", k.tileText, k.tile(1), 4.5},
					{"ink on tile 3", k.tileText, k.tile(2), 4.5},
					{"ink on tile 4", k.tileText, k.tile(3), 4.5},
					{"errors", k.danger, k.bg, 4.5},
					{"YouTube's mark", k.youtube, k.bg, 3},
					{"Spotify's mark", k.spotify, k.bg, 3},
				}...))
			}
		}
	}
}

// TestTonalLayoutsContrast checks that every built-in theme reads in the
// layout designs of tonal controls, which set what is chosen and the
// secondary buttons on fills of the theme's surfaces, and the accent as
// small text on them: at 4.5:1.
func TestTonalLayoutsContrast(t *testing.T) {
	s := newTestShell(t)
	for _, th := range s.inv.Themes {
		for _, layout := range settings.Layouts {
			if !designOf(layout).tonal {
				continue
			}
			for _, art := range contrastArtworks {
				k := contrastLook(s, th.ID, layout, art)
				checkContrast(t, fmt.Sprintf("%s, %s, artwork %q", th.ID, layout, art), []contrastRole{
					{"ink on what is chosen", k.selectedText, k.selected, 4.5},
					{"secondary ink on what is chosen", k.selectedSub, k.selected, 4.5},
					{"ink on a tonal button", k.tonalInk, k.tonalFill, 4.5},
					{"the accent on surface", k.accent, col(over(nrgba(k.surface), nrgba(k.bg))), 4.5},
				})
			}
		}
	}
}

// TestColorFileTonalContrast checks that a colour file's colours read in
// the layout designs of tonal controls too, in a light, a dark and a glass
// theme, as TestTonalLayoutsContrast checks the themes' own, and that a
// Material theme wears the file's colours, not ones made of a seed.
func TestColorFileTonalContrast(t *testing.T) {
	s := newTestShell(t)
	for _, name := range colorFiles {
		f := testColors(t, name)
		for _, id := range []string{"mix", "daylight", "aurora", "material"} {
			for _, layout := range settings.Layouts {
				if !designOf(layout).tonal {
					continue
				}
				for _, art := range []string{"", "#ffe600", "#0a1a6e"} {
					st := settings.Default()
					st.FollowSystem, st.Theme, st.Layout, st.ColorSource = false, id, layout, settings.ColorsFile
					k := newLook(st, s.inv.Themes, true, art, f)
					checkContrast(t, fmt.Sprintf("%s in %s, %s, artwork %q", id, name, layout, art), []contrastRole{
						{"text on the ground", k.text, k.bg, 4.5},
						{"ink on what is chosen", k.selectedText, k.selected, 4.5},
						{"secondary ink on what is chosen", k.selectedSub, k.selected, 4.5},
						{"ink on a tonal button", k.tonalInk, k.tonalFill, 4.5},
						{"the accent on surface", k.accent, col(over(nrgba(k.surface), nrgba(k.bg))), 4.5},
					})
					if id == "material" && art == "" {
						if own := contrastLook(s, id, layout, ""); k.bg == own.bg {
							t.Errorf("material in %s, %s kept its own ground", name, layout)
						}
					}
				}
			}
		}
	}
}

// contrastArtworks are the colours of artworks the contrast is checked
// with: none, then vivid, dull, dark and light ones.
var contrastArtworks = []string{"", "#ffe600", "#00c8ff", "#808080", "#5a3a1a", "#0a1a6e", "#ff0000", "#00ff66", "#f5f5f5", "#111111"}

// contrastLook is the look of theme id in layout, with the artwork art.
func contrastLook(s *shell, id string, layout settings.Layout, art string) look {
	st := settings.Default()
	st.FollowSystem, st.Theme, st.Layout = false, id, layout
	return newLook(st, s.inv.Themes, true, art, colorfile.Colors{})
}

// contrastRole is ink on a ground, which must read at minRatio.
type contrastRole struct {
	role     string
	ink, bg  ui.Color
	minRatio float64
}

func checkContrast(t *testing.T, what string, roles []contrastRole) {
	t.Helper()
	for _, p := range roles {
		if r := contrast(nrgba(p.ink), nrgba(p.bg)); r < p.minRatio {
			t.Errorf("%s: %s at %.2f:1", what, p.role, r)
		}
	}
}
