package main

import (
	"fmt"
	"image/color"
	"strings"
	"testing"

	"github.com/fzn0x/mixmusic/internal/addons"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// builtinTheme is the built-in theme id.
func builtinTheme(t *testing.T, s *shell, id string) addons.Theme {
	t.Helper()
	for _, th := range s.inv.Themes {
		if th.ID == id {
			return th
		}
	}
	t.Fatalf("no built-in theme %s", id)
	return addons.Theme{}
}

// TestMaterialThemesAreTheirSeeds checks that the colours the Material
// themes' manifests list, for the marketplace and the plugins, are those
// of their seed's scheme, which the windows make.
func TestMaterialThemesAreTheirSeeds(t *testing.T) {
	s := newTestShell(t)
	for _, id := range []string{"material", "material-dark"} {
		th := builtinTheme(t, s, id)
		m := materialOf(materialSeed, th.Scheme == "dark")
		var want strings.Builder
		ok := true
		for _, key := range addons.ColorKeys {
			c := m.tokens[key]
			hex := fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
			fmt.Fprintf(&want, "    %q: %q,\n", key, hex)
			if got, _ := parseColor(th.Colors[key]); got != c {
				ok = false
			}
		}
		if !ok {
			t.Errorf("%s's colours aren't its seed's; they would be:\n%s", id, want.String())
		}
	}
}

// TestMaterialSeed checks that the Material themes make their colours of
// the artwork's while it plays, as the accent follows it, of the accent
// the user chose otherwise, and of their own seed without either.
func TestMaterialSeed(t *testing.T) {
	s := newTestShell(t)
	th := builtinTheme(t, s, "material-dark")
	look := func(accent, artwork string, follow bool) look {
		st := settings.Default()
		st.FollowSystem, st.Theme, st.Accent, st.DynamicAccent = false, th.ID, accent, follow
		return newLook(st, s.inv.Themes, true, artwork)
	}
	primary := func(seed color.NRGBA) color.NRGBA { return materialOf(seed, true).tokens["accent"] }
	blue, green := color.NRGBA{R: 0x21, G: 0x4f, B: 0xd8, A: 255}, color.NRGBA{R: 0x2e, G: 0xa0, B: 0x43, A: 255}
	for _, c := range []struct {
		name, accent, artwork string
		follow                bool
		seed                  color.NRGBA
	}{
		{"its own", "", "", true, materialSeed},
		{"the artwork's", "", "#214fd8", true, blue},
		{"the user's", "#2ea043", "", true, green},
		{"the artwork's over the user's", "#2ea043", "#214fd8", true, blue},
		{"the user's, not following the artwork", "#2ea043", "#214fd8", false, green},
	} {
		k := look(c.accent, c.artwork, c.follow)
		if got, want := nrgba(k.accent), primary(c.seed); got != want {
			t.Errorf("%s: the accent is %v, want %v", c.name, got, want)
		}
	}
	// Its surfaces take the seed's hue too: Material's tinted neutrals.
	if look("", "#214fd8", true).bg == look("", "", true).bg {
		t.Error("the ground doesn't change with the seed")
	}
}
