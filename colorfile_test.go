package main

import (
	"image/color"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/colorfile"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// colorFiles are the colour files of internal/colorfile's testdata, made in
// the shapes pywal and matugen write.
var colorFiles = []string{"pywal-3.8.json", "pywal-light.json", "pywal-3.3.0-windows.json",
	"matugen-4.2.json", "matugen-3.1.json", "matugen-2.4.json", "matugen-template.json", "theme.json"}

func testColors(t testing.TB, name string) colorfile.Colors {
	t.Helper()
	c, _, err := colorfile.Read(filepath.Join("internal", "colorfile", "testdata", name))
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return c
}

func hex(c color.NRGBA) string { return hexColor(col(c)) }

func TestFileTokensPywal(t *testing.T) {
	toks, dark := fileTokens(testColors(t, "pywal-3.8.json"), "light")
	if !dark {
		t.Error("pywal's dark colours read as light")
	}
	bg, fg := toks["bg"], toks["text"]
	for name, want := range map[string]string{
		"bg": "#0d1117", "text": "#c5c8cc",
		"surface": hex(mix(bg, fg, 0.045)), "surface-2": hex(mix(bg, fg, 0.09)), "surface-3": hex(mix(bg, fg, 0.15)),
		// color8, #5d6064, doesn't read on the ground at 4.5:1.
		"text-muted": hex(mix(fg, bg, 0.36)),
		// The most vivid of color1 to color6, as the artwork's accent rates
		// them, in turn.
		"accent": "#c2734b", "tile-1": "#c2734b", "tile-2": "#d68a5c", "tile-3": "#3e6a8a", "tile-4": "#5b7c99",
	} {
		if got := hex(toks[name]); got != want {
			t.Errorf("%s is %s, want %s", name, got, want)
		}
	}
	if b := toks["border"]; b.A != 41 || hex(b) != "#c5c8cc" {
		t.Errorf("the border is %v, not the foreground at 16%%", b)
	}
	for _, name := range []string{"danger", "success", "youtube", "spotify"} {
		if _, ok := toks[name]; ok {
			t.Errorf("pywal's colours set %s, which stays the theme's", name)
		}
	}
	// color8 that reads is the muted text; a palette of fewer than four
	// colours fills the tiles from the top again.
	set := map[string]string{"background": "#000000", "foreground": "#ffffff", "color8": "#999999",
		"color1": "#ff0000", "color2": "#ff0000", "color3": "#808080", "color4": "#ff0000", "color5": "#00ff00", "color6": "#ff0000"}
	toks = pywalTokens(set)
	if got := hex(toks["text-muted"]); got != "#999999" {
		t.Errorf("a color8 that reads gave muted text %s", got)
	}
	got := [4]string{hex(toks["tile-1"]), hex(toks["tile-2"]), hex(toks["tile-3"]), hex(toks["tile-4"])}
	if got != [4]string{"#ff0000", "#00ff00", "#808080", "#ff0000"} {
		t.Errorf("the tiles of three colours, the grey last: %v", got)
	}
	if light, dark := fileTokens(testColors(t, "pywal-light.json"), "dark"); dark || hex(light["bg"]) != "#eee9e3" {
		t.Errorf("pywal's light colours: dark %v, %v", dark, light["bg"])
	}
}

func TestFileTokensMaterial(t *testing.T) {
	f := testColors(t, "matugen-4.2.json")
	dark, isDark := fileTokens(f, "dark")
	light, isLight := fileTokens(f, "light")
	if !isDark || isLight {
		t.Errorf("matugen's modes: dark %v, light %v", isDark, isLight)
	}
	for name, want := range map[string]string{
		"bg": "#141218", "surface": "#1d1b20", "surface-2": "#2b2930", "surface-3": "#36343b", "border": "#49454f",
		"text": "#e6e0e9", "text-muted": "#cac4d0", "accent": "#d0bcff", "accent-text": "#381e72", "danger": "#f2b8b5",
		"tile-1": "#d0bcff", "tile-2": "#efb8c8", "tile-3": "#ccc2dc", "tile-4": "#6750a4", "tile-text": "#381e72",
	} {
		if got := hex(dark[name]); got != want {
			t.Errorf("dark %s is %s, want %s", name, got, want)
		}
	}
	if hex(light["bg"]) != "#fef7ff" || hex(light["accent"]) != "#6750a4" {
		t.Errorf("light: %s %s", hex(light["bg"]), hex(light["accent"]))
	}
	// matugen 2.4's dark set has no source_color: the light set's stands.
	if old, _ := fileTokens(testColors(t, "matugen-2.4.json"), "dark"); hex(old["tile-4"]) != "#6750a4" {
		t.Errorf("2.4's dark tile-4 is %s", hex(old["tile-4"]))
	}
	// A source colour all but the primary gives a tile between the first
	// two.
	near := materialTokens(map[string]string{"surface": "#101010", "primary": "#d0bcff", "tertiary": "#efb8c8", "source_color": "#d1bdff"}, nil)
	if t4 := near["tile-4"]; oklabDistance(t4, near["tile-1"]) < 0.04 || oklabDistance(t4, near["tile-2"]) < 0.01 {
		t.Errorf("tile-4 %s of a source colour like the primary", hex(t4))
	}
}

func TestFileTokensTokens(t *testing.T) {
	toks, dark := fileTokens(testColors(t, "matugen-template.json"), "light")
	if dark || hex(toks["bg"]) != "#fef7ff" || hex(toks["tile-4"]) != "#6750a4" || len(toks) != 15 {
		t.Errorf("the template's light set: dark %v, %v", dark, toks)
	}
	toks, dark = fileTokens(testColors(t, "theme.json"), "dark")
	if dark || hex(toks["bg"]) != "#f3f7f5" || len(toks) != 18 {
		t.Errorf("a light theme.json: dark %v, %d tokens", dark, len(toks))
	}
}

// fileLook is the look of a theme in a colour file's colours.
func fileLook(s *shell, theme string, f colorfile.Colors) look {
	set := s.set
	set.FollowSystem, set.Theme, set.Accent, set.ColorSource = false, theme, "", settings.ColorsFile
	return newLook(set, s.inv.Themes, true, "", f)
}

// TestFileColorsReadable keeps every colour file readable under every
// built-in theme, as docs/themes.md asks of themes, and a palette whose
// ground and ink are both of a middle tone too.
func TestFileColorsReadable(t *testing.T) {
	s := newTestShell(t)
	files := map[string]colorfile.Colors{}
	for _, name := range colorFiles {
		files[name] = testColors(t, name)
	}
	mid, err := colorfile.Parse([]byte(`{"special": {"background": "#777777", "foreground": "#999999"},
		"colors": {"color0": "#777777", "color1": "#7a7a7a", "color2": "#888888", "color3": "#808080", "color4": "#7f7f7f",
		"color5": "#858585", "color6": "#7c7c7c", "color8": "#787878"}}`))
	if err != nil {
		t.Fatal(err)
	}
	files["mid-tones"] = mid
	n := 0
	for _, th := range s.inv.Themes {
		if !th.Builtin {
			continue
		}
		for name, f := range files {
			n++
			k := fileLook(s, th.ID, f)
			bg := nrgba(k.bg)
			on := func(what string, c, ground ui.Color, ratio float64) {
				if got := contrast(over(nrgba(c), nrgba(ground)), nrgba(ground)); got < ratio {
					t.Errorf("%s in %s: %s %s on %s at %.2f:1, want %.1f:1", th.ID, name, what, hexColor(c), hexColor(ground), got, ratio)
				}
			}
			on("text", k.text, k.bg, 4.5)
			on("muted text", k.muted, k.bg, 4.5)
			on("danger", k.danger, k.bg, 4.5)
			on("the accent", k.accent, k.bg, 3)
			on("ink on the accent", k.accentText, k.accent, 4.5)
			on("success", k.success, k.bg, 3)
			on("the YouTube mark", k.youtube, k.bg, 3)
			on("the Spotify mark", k.spotify, k.bg, 3)
			for i, tile := range k.tiles {
				on("ink on tile "+string(rune('1'+i)), k.tileText, tile, 4.5)
			}
			if bg.A != 255 {
				t.Errorf("%s in %s: the ground shows through", th.ID, name)
			}
		}
	}
	if n == 0 {
		t.Fatal("no built-in themes")
	}
}

// TestLookColorFileModes gives a file with a set of each mode's colours to
// the theme of that mode, and decides with a file of one set whether the
// look is dark; the theme cards keep their own colours.
func TestLookColorFileModes(t *testing.T) {
	s := newTestShell(t)
	matugen := testColors(t, "matugen-4.2.json")
	if k := fileLook(s, "mix", matugen); !k.dark || hexColor(k.bg) != "#141218" {
		t.Errorf("Mix in matugen's colours: dark %v, ground %s", k.dark, hexColor(k.bg))
	}
	if k := fileLook(s, "daylight", matugen); k.dark || hexColor(k.bg) != "#fef7ff" {
		t.Errorf("Daylight in matugen's colours: dark %v, ground %s", k.dark, hexColor(k.bg))
	}
	if k := fileLook(s, "mix", testColors(t, "pywal-light.json")); k.dark || k.theme.Dark || hexColor(k.bg) != "#eee9e3" {
		t.Errorf("Mix in pywal's light colours: dark %v, ground %s", k.dark, hexColor(k.bg))
	}
	if k := fileLook(s, "daylight", testColors(t, "pywal-3.8.json")); !k.dark || !k.theme.Dark {
		t.Error("Daylight in pywal's dark colours isn't dark")
	}
	// The settings without a colour file, as the theme cards have them,
	// wear the theme's own.
	own := newLook(settings.Settings{Theme: "mix", FontScale: 1}, s.inv.Themes, true, "", matugen)
	if hexColor(own.bg) != "#000000" {
		t.Errorf("a theme card wears the file's ground %s", hexColor(own.bg))
	}
	// A file not read yet, or gone, leaves the theme's colours.
	if k := fileLook(s, "mix", colorfile.Colors{}); hexColor(k.bg) != "#000000" || !k.dark {
		t.Errorf("Mix without a file's colours: %s", hexColor(k.bg))
	}
}

// TestLookColorFileAccent takes the accent from the artwork, then from the
// user's choice, then from the file; the theme keeps its type, corners and
// glass.
func TestLookColorFileAccent(t *testing.T) {
	s := newTestShell(t)
	f := testColors(t, "matugen-4.2.json")
	set := s.set
	set.FollowSystem, set.Theme, set.ColorSource, set.DynamicAccent = false, "mochi", settings.ColorsFile, true
	set.Accent = ""
	// Mochi is light: it wears matugen's light colours.
	if k := newLook(set, s.inv.Themes, true, "", f); hexColor(k.accent) != "#6750a4" || hexColor(k.themeAccent) != "#6750a4" {
		t.Errorf("the file's accent: %s", hexColor(k.accent))
	}
	set.Accent = "#ff8800"
	k := newLook(set, s.inv.Themes, true, "", f)
	readable := func(c string) string {
		a, _ := parseColor(c)
		return hexColor(col(readableOn(a, nrgba(k.bg))))
	}
	if hexColor(k.accent) != readable("#ff8800") || hexColor(k.themeAccent) != "#6750a4" {
		t.Errorf("the user's accent over the file's: %s, the file's %s", hexColor(k.accent), hexColor(k.themeAccent))
	}
	if a := newLook(set, s.inv.Themes, true, "#00e0a0", f); hexColor(a.accent) != readable("#00e0a0") {
		t.Errorf("the artwork's accent over the user's: %s", hexColor(a.accent))
	}
	theme := newLook(settings.Settings{Theme: "mochi", FontScale: 1}, s.inv.Themes, true, "", colorfile.Colors{})
	if k.body != theme.body || k.display != theme.display || k.radius != theme.radius || k.fx != theme.fx {
		t.Errorf("Mochi in a file's colours lost its type, corners or effects: %+v", k)
	}
	// Aurora's glass stays glass, which shows the file's surface on the
	// ground.
	aurora := fileLook(s, "aurora", testColors(t, "pywal-3.8.json"))
	want := mix(nrgba(aurora.bg), nrgba(aurora.text), 0.045)
	if got := over(nrgba(aurora.surface), nrgba(aurora.bg)); aurora.surface.A == 255 || !near(got, want) {
		t.Errorf("Aurora's surface in pywal's colours is %+v, showing %v, want %v", aurora.surface, got, want)
	}
}

// TestGlassOf finds a translucent colour that shows as the colour asked
// for over the ground.
func TestGlassOf(t *testing.T) {
	ground := color.NRGBA{0x0d, 0x11, 0x17, 255}
	for _, c := range []color.NRGBA{{0x18, 0x1c, 0x22, 255}, {0xff, 0xff, 0xff, 255}, {0, 0, 0, 255}, ground, {0x40, 0x08, 0x10, 255}} {
		g := glassOf(c, ground, 20)
		if g.A < 20 || !near(over(g, ground), c) {
			t.Errorf("glass of %v: %v, showing %v", c, g, over(g, ground))
		}
	}
}

// TestAppearanceOfColorFile gives the title bar the light or dark of a
// colour file with one set of colours, though the theme follows the
// computer, and wears the theme of that mode; a file with both leaves it
// to the computer.
func TestAppearanceOfColorFile(t *testing.T) {
	var got []mygo.ThemeSource
	setAppearance = func(source mygo.ThemeSource) { got = append(got, source) }
	defer func() { setAppearance = func(mygo.ThemeSource) {} }()
	s := newTestShell(t)
	for _, c := range []struct {
		file   string
		follow bool
		want   mygo.ThemeSource
		theme  string
	}{
		{"pywal-light.json", true, mygo.ThemeLight, "daylight"},
		{"pywal-3.8.json", true, mygo.ThemeDark, "mix"},
		{"pywal-light.json", false, mygo.ThemeLight, "mix"},
		{"matugen-4.2.json", true, mygo.ThemeSystem, "mix"},
	} {
		got = nil
		s.set.FollowSystem, s.set.Theme, s.set.LightTheme, s.set.DarkTheme = c.follow, "mix", "daylight", "mix"
		s.set.ColorSource = settings.ColorsFile
		s.colorFile = app.ColorFileStatus{Colors: testColors(t, c.file)}
		s.restyle()
		if len(got) != 1 || got[0] != c.want {
			t.Errorf("%s, following the computer %v: the appearance went %v, want %s", c.file, c.follow, got, c.want)
		}
		if th, _ := s.activeTheme(); th.ID != c.theme {
			t.Errorf("%s, following the computer %v: wearing %s, want %s", c.file, c.follow, th.ID, c.theme)
		}
	}
}

// TestGroundTone tones the cover's colour on Greenroom's and Glasshouse's
// grounds as their own grounds have it, and as a colour file's ground does.
func TestGroundTone(t *testing.T) {
	s := newTestShell(t)
	for theme, want := range map[string]toning{"greenroom": washTone, "glasshouse": coverTone} {
		set := s.set
		set.FollowSystem, set.Theme = false, theme
		if got := groundTone(newLook(set, s.inv.Themes, true, "", colorfile.Colors{}).dark); got != want {
			t.Errorf("%s's own ground is toned %v, want %v", theme, got, want)
		}
	}
	if got := groundTone(fileLook(s, "greenroom", testColors(t, "pywal-light.json")).dark); got != coverTone {
		t.Errorf("Greenroom on pywal's light ground is toned %v", got)
	}
	if got := groundTone(fileLook(s, "glasshouse", testColors(t, "pywal-3.8.json")).dark); got != washTone {
		t.Errorf("Glasshouse on pywal's dark ground is toned %v", got)
	}
}
