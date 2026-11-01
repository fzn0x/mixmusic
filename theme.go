package main

import (
	"image/color"
	"runtime"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/addons"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// look is how the interface looks: a theme's tokens, which themes keep in
// the native interface, and the design's measures, scaled by the text
// size and the density of the settings.
type look struct {
	theme ui.Theme
	dark  bool
	// systemDark is the computer's appearance, which the themes that
	// follow it go by.
	systemDark bool

	bg, surface, surface2, surface3, border ui.Color
	text, muted, accent, accentText         ui.Color
	danger, success, youtube, spotify       ui.Color
	tiles                                   [4]ui.Color
	tileText                                ui.Color
	// The tints derived from the tokens.
	hover, press, rule, outline, scrim ui.Color
	// raised is the face of the menus that float over the page: surface-2
	// over the ground, solid where the theme's surfaces show the page
	// through, as Aurora's glass does.
	raised ui.Color

	radius, radiusSm float32
	body, display    string
	scale            float32
	compact          bool
	// fx are the effects of the built-in themes.
	fx effects

	// The shell's measures; rail is the unfolded rail's width, gap the gap
	// between covers, and cover their size on the shelves.
	row, rail, panel, player, gutter float32
	gap, cover                       float32
	// The layout tokens' styles (layout.go): pill rounds buttons, chips
	// and the search box fully; tonal fills controls rather than outlining
	// them; cards sets covers in filled cards; floating floats the player
	// bar over the window's foot.
	pill, tonal, cards, floating bool
	headings                     headingStyle
	motion                       motionStyle
	// selected fills what is chosen, as the current page and the track
	// that plays, and selectedText and selectedSub ink it: the accent, or,
	// with tonal controls, a quieter fill. tonalFill fills the secondary
	// buttons of tonal controls, and tonalInk inks them.
	selected, selectedText, selectedSub ui.Color
	tonalFill, tonalInk                 ui.Color
}

// fallbackTheme is the Mix theme's tokens, for a theme that lacks some.
var fallbackTheme = map[string]string{
	"bg": "#000000", "surface": "#0b0b0b", "surface-2": "#181818", "surface-3": "#272727",
	"border": "rgba(255, 255, 255, 0.16)", "text": "#ffffff", "text-muted": "#a3a3a3",
	"accent": "#00bde3", "accent-text": "#000000", "danger": "#ff5a4f", "success": "#a4e600",
	"youtube": "#ff4040", "spotify": "#1ed760", "tile-1": "#00bde3", "tile-2": "#e6007e",
	"tile-3": "#a4e600", "tile-4": "#ffb000", "tile-text": "#000000",
}

// newLook is the look of the active theme of s, among themes; dark tells
// whether the computer is in dark mode, for themes that follow it, and
// artwork is the colour of the artwork playing, "" for none.
func newLook(s settings.Settings, themes []addons.Theme, dark bool, artwork string) look {
	id := s.ActiveTheme(dark)
	var th addons.Theme
	for _, t := range themes {
		if t.ID == id {
			th = t
			break
		}
	}
	var fx effects
	if th.Builtin {
		fx = themeEffects(th, s.ThemeOptions[th.ID])
	}
	// The accent: the artwork's, the user's, or the theme's.
	pick := s.Accent
	if s.DynamicAccent && artwork != "" {
		pick = artwork
	}
	// The Material themes make all their colours of it, as their seed.
	var mat *materialColors
	if fx.material {
		seed := materialSeed
		if a, ok := parseColor(pick); ok && pick != "" {
			seed = a
		}
		mat = materialOf(seed, th.Scheme != "light")
	}
	tok := func(name string) color.NRGBA {
		if c, ok := mat.token(name); ok {
			return c
		}
		if c, ok := parseColor(th.Colors[name]); ok {
			return c
		}
		c, _ := parseColor(fallbackTheme[name])
		return c
	}
	bg := tok("bg")
	bg.A = 255
	text, accent := tok("text"), tok("accent")
	accentText := tok("accent-text")
	// Other themes take it as it reads on the ground: at 3:1, or, where
	// controls are tonal, which set it as small text on their fills, at
	// 4.5:1 on the panels' surface, which has less contrast with it.
	if a, ok := parseColor(pick); ok && pick != "" && mat == nil {
		if th.Layout["controls"] == "tonal" {
			a = legibleOn(a, over(tok("surface"), bg), 4.5)
		} else {
			a = readableOn(a, bg)
		}
		accent, accentText = a, inkOn(a)
	}
	k := look{dark: th.Scheme != "light", systemDark: dark, scale: float32(s.FontScale), compact: s.Density == settings.DensityCompact}
	if th.ID == "" {
		k.dark = true
	}
	if k.scale <= 0 {
		k.scale = 1
	}
	k.bg, k.surface, k.surface2, k.surface3 = col(bg), col(tok("surface")), col(tok("surface-2")), col(tok("surface-3"))
	k.border, k.text, k.muted = col(tok("border")), col(text), col(tok("text-muted"))
	k.accent, k.accentText = col(accent), col(accentText)
	k.danger, k.success, k.youtube, k.spotify = col(tok("danger")), col(tok("success")), col(tok("youtube")), col(tok("spotify"))
	for i := range k.tiles {
		k.tiles[i] = col(tok("tile-" + strconv.Itoa(i+1)))
	}
	k.tileText = col(tok("tile-text"))
	if th.Builtin {
		k.fx = fx
		if th.ID == "mix" && optionValue(th, s.ThemeOptions[th.ID], "white-tiles") == "true" {
			// Classic white ink on tiles, for those who prefer it to contrast.
			k.tileText = ui.RGB(255, 255, 255)
		}
	}
	k.hover = col(alpha(text, 0.07))
	k.press = col(alpha(text, 0.13))
	k.rule = col(alpha(accent, 0.45))
	k.outline = col(alpha(text, 0.85))
	k.scrim = col(alpha(bg, 0.72))
	k.raised = col(over(tok("surface-2"), bg))
	k.radius = cssLength(th.Radius)
	k.body = fontFamily(th.Fonts.Body)
	k.display = fontFamily(th.Fonts.Display)
	if th.Fonts.Display == "" {
		k.display = k.body
	}
	k.layoutOf(th)
	k.selected, k.selectedText, k.selectedSub = k.accent, k.accentText, k.accentText.Alpha(0.78)
	k.tonalFill, k.tonalInk = col(over(tok("surface-3"), bg)), k.text
	switch {
	case k.tonal && mat != nil:
		// Material's secondary container.
		k.selected, k.selectedText, k.selectedSub = col(mat.selected), col(mat.selectedText), col(mat.selectedText)
		k.tonalFill, k.tonalInk = k.selected, k.selectedText
	case k.tonal:
		// The highest surface, with the accent's ink, as it reads there.
		k.selected, k.selectedText, k.selectedSub = k.tonalFill, col(legibleOn(nrgba(k.accent), nrgba(k.tonalFill), 4.5)), k.muted
	}

	t := *ui.DarkTheme()
	if !k.dark {
		t = *ui.LightTheme()
	}
	t.Dark = k.dark
	t.Background, t.Surface = k.bg, k.surface
	t.SurfaceHover, t.SurfacePressed = col(over(alpha(text, 0.07), bg)), col(over(alpha(text, 0.13), bg))
	t.Border, t.Text, t.TextMuted = k.border, k.text, k.muted
	t.Accent, t.AccentText = k.accent, k.accentText
	t.AccentHover = col(mix(accent, text, 0.14))
	t.AccentPressed = col(mix(accent, bg, 0.14))
	t.Danger, t.Success, t.Warning = k.danger, k.success, col(tok("tile-4"))
	t.Selection, t.Focus = col(alpha(accent, 0.35)), k.accent
	// Toasts are inverted, an ink block with ground-coloured text.
	t.Inverse, t.InverseText = k.text, k.bg
	t.Scrollbar = col(tok("surface-3"))
	t.Radius = k.radiusSm
	t.FontSize = k.size(0.9375)
	t.Font = k.body
	k.theme = t
	return k
}

// size is rem rem of text, scaled by the text size.
func (k *look) size(rem float32) float32 { return rem * 16 * k.scale }

// The type scale.
func (k *look) bodySize() float32     { return k.size(0.9375) }
func (k *look) smallSize() float32    { return k.size(0.8125) }
func (k *look) captionSize() float32  { return k.size(0.75) }
func (k *look) subtitleSize() float32 { return k.size(1.375) }
func (k *look) titleSize() float32    { return k.size(2.125) }
func (k *look) pivotSize() float32    { return k.size(1.75) }

// tile is the colour of the tile of tone i, which rotate.
func (k *look) tile(i int) ui.Color { return k.tiles[((i%4)+4)%4] }

func col(c color.NRGBA) ui.Color { return ui.Color{R: c.R, G: c.G, B: c.B, A: c.A} }

// cssLength reads a length in pixels, as "12px", "0.75rem" or "0", a rem
// or an em being 16 pixels.
func cssLength(s string) float32 {
	s, unit := strings.TrimSpace(s), float64(1)
	for _, u := range []string{"rem", "em"} {
		if strings.HasSuffix(s, u) {
			s, unit = strings.TrimSuffix(s, u), 16
			break
		}
	}
	v, err := strconv.ParseFloat(strings.TrimSuffix(s, "px"), 32)
	if err != nil || v < 0 {
		return 0
	}
	return float32(v * unit)
}

// fontFamily picks the first family of a CSS font stack that MixMusic or
// the system has, or the system's font.
func fontFamily(stack string) string {
	have := map[string]bool{}
	for _, f := range ui.FontFamilies() {
		have[strings.ToLower(f.Name)] = true
	}
	for _, name := range strings.Split(stack, ",") {
		name = strings.Trim(strings.TrimSpace(name), `'"`)
		switch strings.ToLower(name) {
		case "":
			continue
		case "selawik":
			return face
		case "system-ui", "sans-serif", "-apple-system":
			return ""
		case "monospace", "ui-monospace":
			return monospace()
		}
		if have[strings.ToLower(name)] {
			return name
		}
	}
	if stack == "" {
		return face
	}
	return ""
}

// monospace is the system's monospaced family.
func monospace() string {
	switch runtime.GOOS {
	case "windows":
		return "Consolas"
	case "darwin":
		return "Menlo"
	}
	return "DejaVu Sans Mono"
}
