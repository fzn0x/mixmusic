package main

import (
	"bytes"
	"context"
	"slices"
	"strconv"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/addons"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
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
// lowercase, once for each, and the others as written.
func TestWords(t *testing.T) {
	k := lookOf(themeWith(nil), settings.DensityComfortable)
	if got := k.words("Liked songs on YouTube Music"); got != "liked songs on youtube music" {
		t.Errorf("light: %q", got)
	}
	// The frames lower the same words: once lowered, they cost nothing.
	if n := testing.AllocsPerRun(10, func() { k.words("Liked songs on YouTube Music") }); n != 0 {
		t.Errorf("lowering words again allocates %v times", n)
	}
	k = lookOf(themeWith(map[string]string{"headings": "regular"}), settings.DensityComfortable)
	if got := k.words("Liked songs on YouTube Music"); got != "Liked songs on YouTube Music" {
		t.Errorf("regular: %q", got)
	}
}

// defaultLayout are the layout tokens at the values a theme without them
// has, for a theme of the radius given.
func defaultLayout(radius float64) map[string]string {
	return map[string]string{
		"radius-small": strconv.FormatFloat(radius*0.6, 'f', -1, 32) + "px", "shape": "box", "row": "52px",
		"gutter": "40px", "gap": "16px", "rail": "248px", "cover": "172px", "player-height": "92px",
		"player": "docked", "controls": "outline", "headings": "light", "cards": "none", "motion": "standard",
	}
}

// pagesOf draws the main pages of a shell in theme, at a density, with
// the layout tokens given, nil for none, as images, by name.
func pagesOf(t *testing.T, theme string, density settings.Density, layout map[string]string) map[string][]byte {
	t.Helper()
	s := newTestShell(t)
	st := s.svc.Preferences.Get()
	st.FollowSystem, st.Theme, st.Density = false, theme, density
	if _, err := s.svc.Preferences.Set(st); err != nil {
		t.Fatal(err)
	}
	s.refresh()
	for i := range s.inv.Themes {
		if s.inv.Themes[i].ID == theme {
			s.inv.Themes[i].Layout = layout
		}
	}
	s.restyle()
	album := media.Album{ID: "al-1", Source: media.SourceYouTube, Title: "Hurry Up, We're Dreaming",
		Artists: []media.Ref{{ID: "ar-m83", Name: "M83"}}, Year: "2011", Tracks: testTracks}
	var items []media.Item
	for i := range testTracks {
		items = append(items, media.Item{Kind: media.ItemTrack, Track: &testTracks[i]})
	}
	s.loads["home"] = &loaded[[]media.Shelf]{done: true, value: []media.Shelf{
		{Title: "Listen again", Source: media.SourceYouTube, Items: items}, {Title: "Albums for you", Source: media.SourceYouTube,
			Items: []media.Item{{Kind: media.ItemAlbum, Album: &album}}}}}
	s.loads["album:al-1"] = &loaded[media.Album]{done: true, value: album}
	for _, tr := range testTracks[:3] {
		if err := s.svc.Collection.SetLiked(context.Background(), tr, true); err != nil {
			t.Fatal(err)
		}
	}
	tt := testerOf(t, s, 1280, 820)
	s.loads["home"] = &loaded[[]media.Shelf]{done: true, value: []media.Shelf{
		{Title: "Listen again", Source: media.SourceYouTube, Items: items}, {Title: "Albums for you", Source: media.SourceYouTube,
			Items: []media.Item{{Kind: media.ItemAlbum, Album: &album}}}}}
	s.p.PlayTracks(testTracks, 1)
	s.settle(t, func(st player.State) bool { return st.Playing })
	pages := map[string][]byte{}
	for _, path := range []string{"/", "/album/al-1", "/now-playing", "/library/liked", "/settings/appearance"} {
		show(s, tt, path)
		pages[path] = slices.Clone(tt.Image().Pix)
	}
	mini := ui.NewTester(s.miniView, 420, 132)
	mini.Frame()
	pages["mini"] = slices.Clone(mini.Image().Pix)
	return pages
}

// TestDefaultLayoutDrawsAsBefore draws the main pages of themes without
// layout tokens, then with every token set to the value a theme without
// it has: the pages are the same to the pixel, at both densities.
func TestDefaultLayoutDrawsAsBefore(t *testing.T) {
	for _, c := range []struct {
		theme   string
		radius  float64
		density settings.Density
	}{{"mix", 0, settings.DensityComfortable}, {"mix", 0, settings.DensityCompact}, {"sakura", 20, settings.DensityComfortable}} {
		before := pagesOf(t, c.theme, c.density, nil)
		after := pagesOf(t, c.theme, c.density, defaultLayout(c.radius))
		for name, img := range before {
			if !bytes.Equal(img, after[name]) {
				t.Errorf("%s, %s: %s differs with the default layout tokens", c.theme, c.density, name)
			}
		}
	}
}

// TestThemeCommands checks that the palette has a command for each theme,
// made once for the add-ons rather than at each frame, which chooses it.
func TestThemeCommands(t *testing.T) {
	s := newTestShell(t)
	var themes []command
	for _, cmd := range s.commands() {
		if cmd.group == "themes" {
			themes = append(themes, cmd)
		}
	}
	if len(themes) != len(s.inv.Themes) {
		t.Fatalf("%d theme commands for %d themes", len(themes), len(s.inv.Themes))
	}
	for _, cmd := range themes {
		if cmd.id == "theme.crate" {
			cmd.run()
		}
	}
	if got := s.svc.Preferences.Get().ActiveTheme(true); got != "crate" {
		t.Errorf("the theme is %q, not crate", got)
	}
}
