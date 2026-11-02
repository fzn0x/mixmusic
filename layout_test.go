package main

import (
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/addons"
	"github.com/fzn0x/mixmusic/internal/colorfile"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// lookOf is the look of a theme as Mix, of corners of 10px, in the
// layout design and at the density given.
func lookOf(layout settings.Layout, density settings.Density) look {
	th := addons.Theme{ID: "test", Name: "Test", Scheme: "dark", Colors: fallbackTheme, Radius: "10px"}
	s := settings.Default()
	s.FollowSystem, s.Theme, s.Layout, s.Density = false, th.ID, layout, density
	return newLook(s, []addons.Theme{th}, true, "", colorfile.Colors{})
}

// TestTilesLayout checks that Tiles has the measures and styles the
// interface had before the layout designs, the theme's corners, and the
// compact density's measures.
func TestTilesLayout(t *testing.T) {
	k := lookOf(settings.LayoutTiles, settings.DensityComfortable)
	if k.row != 52 || k.rail != 248 || k.player != 92 || k.gutter != 40 || k.gap != 16 || k.cover != 172 || k.radius != 10 || k.radiusSm != 6 {
		t.Errorf("measures: row %v, rail %v, player %v, gutter %v, gap %v, cover %v, radius %v and %v",
			k.row, k.rail, k.player, k.gutter, k.gap, k.cover, k.radius, k.radiusSm)
	}
	if k.pill || k.tonal || k.cards || k.floating || k.headings != headingsLight || k.motion != motionStandard {
		t.Errorf("styles: %+v", k)
	}
	if k.selected != k.accent || k.selectedText != k.accentText {
		t.Error("what is chosen isn't flooded with the accent")
	}
	k = lookOf(settings.LayoutTiles, settings.DensityCompact)
	if k.row != 40 || k.rail != 224 || k.player != 80 || k.gutter != 28 {
		t.Errorf("compact: row %v, rail %v, player %v, gutter %v", k.row, k.rail, k.player, k.gutter)
	}
}

// TestLayoutDesigns checks that each layout design sets the look, with
// corners of its own rather than the theme's, that the compact density
// takes off its measures what it takes off Tiles', and that a layout
// unknown is Blend.
func TestLayoutDesigns(t *testing.T) {
	if len(layoutDesigns) != len(settings.Layouts) {
		t.Fatalf("%d layout designs for %d layouts", len(layoutDesigns), len(settings.Layouts))
	}
	for _, c := range []struct {
		layout                                 settings.Layout
		row, rail, player, gutter, gap, cover  float32
		radius, small                          float32
		pill, floating                         bool
		headings                               headingStyle
		motion                                 motionStyle
		compactRow, compactRail, compactPlayer float32
		compactGutter                          float32
	}{
		{settings.LayoutBlend, 56, 232, 80, 28, 14, 188, 20, 10, true, true, headingsBold, motionExpressive, 43, 210, 70, 20},
		{settings.LayoutExpressive, 64, 256, 80, 32, 16, 176, 28, 12, true, true, headingsRegular, motionExpressive, 49, 231, 70, 22},
		{settings.LayoutCrate, 48, 220, 76, 24, 12, 196, 8, 4, false, false, headingsBold, motionSmooth, 40, 199, 66, 17},
	} {
		k := lookOf(c.layout, settings.DensityComfortable)
		if k.row != c.row || k.rail != c.rail || k.player != c.player || k.gutter != c.gutter || k.gap != c.gap || k.cover != c.cover ||
			k.radius != c.radius || k.radiusSm != c.small {
			t.Errorf("%s: row %v, rail %v, player %v, gutter %v, gap %v, cover %v, radius %v and %v", c.layout,
				k.row, k.rail, k.player, k.gutter, k.gap, k.cover, k.radius, k.radiusSm)
		}
		if k.pill != c.pill || k.floating != c.floating || !k.tonal || !k.cards || k.headings != c.headings || k.motion != c.motion {
			t.Errorf("%s: styles: %+v", c.layout, k)
		}
		if k.selected == k.accent {
			t.Errorf("%s: tonal controls flood what is chosen with the accent", c.layout)
		}
		k = lookOf(c.layout, settings.DensityCompact)
		if k.row != c.compactRow || k.rail != c.compactRail || k.player != c.compactPlayer || k.gutter != c.compactGutter {
			t.Errorf("%s, compact: row %v, rail %v, player %v, gutter %v", c.layout, k.row, k.rail, k.player, k.gutter)
		}
	}
	if k, want := lookOf("bogus", settings.DensityComfortable), lookOf(settings.LayoutBlend, settings.DensityComfortable); k.row != want.row || k.cover != want.cover {
		t.Errorf("an unknown layout has rows of %v and covers of %v, not Blend's", k.row, k.cover)
	}
}

// TestWords checks that the light headings set the interface's words in
// lowercase, once for each, and the others as written.
func TestWords(t *testing.T) {
	k := lookOf(settings.LayoutTiles, settings.DensityComfortable)
	if got := k.words("Liked songs on YouTube Music"); got != "liked songs on youtube music" {
		t.Errorf("light: %q", got)
	}
	// The frames lower the same words: once lowered, they cost nothing.
	if n := testing.AllocsPerRun(10, func() { k.words("Liked songs on YouTube Music") }); n != 0 {
		t.Errorf("lowering words again allocates %v times", n)
	}
	for _, l := range []settings.Layout{settings.LayoutBlend, settings.LayoutExpressive, settings.LayoutCrate} {
		k = lookOf(l, settings.DensityComfortable)
		if got := k.words("Liked songs on YouTube Music"); got != "Liked songs on YouTube Music" {
			t.Errorf("%s: %q", l, got)
		}
		if got := k.sentence("waiting for the network"); got != "Waiting for the network" {
			t.Errorf("%s: the label %q", l, got)
		}
		if n := testing.AllocsPerRun(10, func() { k.sentence("waiting for the network") }); n != 0 {
			t.Errorf("%s: capitalising a label again allocates %v times", l, n)
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
		if cmd.id == "theme.walnut" {
			cmd.run()
		}
	}
	if got := s.svc.Preferences.Get().ActiveTheme(true); got != "walnut" {
		t.Errorf("the theme is %q, not walnut", got)
	}
}

// TestChooseLayout chooses each layout design in Settings › Appearance:
// the setting is saved and the window takes its measures at once. The
// search of the settings finds the choices by what they say.
func TestChooseLayout(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 2400)
	show(s, tt, "/settings/appearance")
	for _, c := range []struct {
		label  string
		layout settings.Layout
		row    float32
	}{{"crate", settings.LayoutCrate, 48}, {"Expressive", settings.LayoutExpressive, 64}, {"Blend", settings.LayoutBlend, 56}, {"Tiles", settings.LayoutTiles, 52}} {
		click(t, s, tt, c.label)
		s.refresh() // as the app's watch does on a change of the settings
		tt.Frame()
		if got := s.svc.Preferences.Get().Layout; got != c.layout || s.lk.row != c.row {
			t.Errorf("chose %s: the setting is %q and rows are %vpx", c.label, got, s.lk.row)
		}
	}
	show(s, tt, settingsPath("appearance", "big covers"))
	tt.Advance(time.Second)
	showsOnly(t, tt, "searching big covers", []string{"layout", "crate", "blend"}, []string{"Follow the artwork", "Density"})
}

// TestLayoutEditedInSettingsFile applies a layout design written in
// settings.json while MixMusic runs, and puts back one unknown.
func TestLayoutEditedInSettingsFile(t *testing.T) {
	s := fileShell(t)
	tt := tester(t, s)
	editSettings(t, s, `{"layout": "crate"}`)
	s.refresh()
	tt.Frame()
	if s.set.Layout != settings.LayoutCrate || s.lk.row != 48 {
		t.Errorf("the layout is %q, with rows of %vpx", s.set.Layout, s.lk.row)
	}
	editSettings(t, s, `{"layout": "grid"}`)
	s.refresh()
	tt.Frame()
	if s.set.Layout != settings.LayoutBlend || s.lk.row != 56 {
		t.Errorf("an unknown layout gave %q, with rows of %vpx", s.set.Layout, s.lk.row)
	}
}
