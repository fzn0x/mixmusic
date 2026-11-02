package settings

import (
	"maps"
	"math"
	"regexp"
	"slices"
)

// Density chooses how much room the interface gives each element.
type Density string

const (
	DensityComfortable Density = "comfortable"
	DensityCompact     Density = "compact"
)

// Layout chooses the layout design of the interface: its shapes, its
// density, its covers, its player bar, its headings and its motion. The
// theme sets its colours and faces.
type Layout string

const (
	// LayoutBlend has Expressive's rounded shapes and springs, and
	// Crate's dense rows, big covers and bold headings.
	LayoutBlend Layout = "blend"
	// LayoutExpressive is playful and colourful: tonal fills, big rounded
	// shapes, a floating player bar and springy motion.
	LayoutExpressive Layout = "expressive"
	// LayoutCrate is restrained: dense rows, big covers in cards and bold
	// tight headings, so that the covers do the work.
	LayoutCrate Layout = "crate"
	// LayoutTiles is MixMusic's first layout: flat tiles, giant light
	// lowercase words and outlined boxes.
	LayoutTiles Layout = "tiles"
)

// Layouts are the layout designs, in the order the settings list them.
var Layouts = []Layout{LayoutBlend, LayoutExpressive, LayoutCrate, LayoutTiles}

// ColorSource chooses where the colours come from: the theme, or a file
// that pywal or matugen make of the wallpaper, or one of the user's.
type ColorSource string

const (
	// ColorsTheme wears the theme's own colours.
	ColorsTheme ColorSource = ""
	// ColorsPywal reads pywal's colors.json, in pywal's cache folder.
	ColorsPywal ColorSource = "pywal"
	// ColorsMatugen reads matugen.json in the data folder, which
	// MixMusic's matugen template writes.
	ColorsMatugen ColorSource = "matugen"
	// ColorsFile reads the file of Settings.ColorFile.
	ColorsFile ColorSource = "file"
)

// maxColorFile bounds the length of Settings.ColorFile, in bytes.
const maxColorFile = 4096

// colorRe matches a colour of the accent, as "#ff8800" or "#f80".
var colorRe = regexp.MustCompile(`^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

// Appearance is how the interface looks: Settings › Appearance, and the
// sidebar and side panel as the user leaves them.
type Appearance struct {
	// Theme is the ID of the theme the interface wears, unless it follows
	// the computer.
	Theme string `json:"theme"`
	// FollowSystem shows LightTheme while the computer is in light mode
	// and DarkTheme while it is in dark mode, in place of Theme.
	FollowSystem bool `json:"followSystem"`
	// LightTheme is the ID of the theme of light mode, as FollowSystem
	// shows it.
	LightTheme string `json:"lightTheme"`
	// DarkTheme is the ID of the theme of dark mode, as FollowSystem
	// shows it.
	DarkTheme string `json:"darkTheme"`
	// ThemeOptions holds the values of each theme's options, by theme ID
	// and option ID.
	ThemeOptions map[string]map[string]string `json:"themeOptions"`
	// Accent replaces the theme's accent color, unless empty.
	Accent string `json:"accent"`
	// DynamicAccent takes the accent color from the artwork playing.
	DynamicAccent bool `json:"dynamicAccent"`
	// ColorSource takes the colours from a file, over the theme's, which
	// keeps its type, corners and effects.
	ColorSource ColorSource `json:"colorSource"`
	// ColorFile is the file of ColorsFile: a full path, or one that starts
	// with ~.
	ColorFile string `json:"colorFile"`
	// Density is how much room the interface gives each element.
	Density Density `json:"density"`
	// Layout is the layout design; settings from before it have Blend's.
	Layout Layout `json:"layout"`
	// FontScale scales the text, from 0.85 to 1.3.
	FontScale float64 `json:"fontScale"`
	// Zoom scales the whole interface, one of ZoomSteps: the text, which
	// FontScale scales too, and everything else.
	Zoom float64 `json:"zoom"`
	// SidebarCollapsed shows the sidebar as icons only.
	SidebarCollapsed bool `json:"sidebarCollapsed"`
	// Panel is the side panel beside the pages: "queue", "lyrics" or
	// "none". Settings from before it show the queue.
	Panel string `json:"panel"`
}

// defaultAppearance is how a new install looks.
func defaultAppearance() Appearance {
	var a Appearance
	a.Theme = "mix"
	a.LightTheme = "daylight"
	a.DarkTheme = "mix"
	a.ThemeOptions = map[string]map[string]string{}
	a.DynamicAccent = true
	a.Density = DensityComfortable
	a.Layout = LayoutBlend
	a.Panel = "queue"
	a.FontScale = 1
	a.Zoom = 1
	return a
}

// clean is a with every value in its range, and d's in place of values
// that are missing or invalid.
func (a Appearance) clean(d Appearance) Appearance {
	if !ValidID(a.Theme) {
		a.Theme = d.Theme
	}
	if !ValidID(a.LightTheme) {
		a.LightTheme = d.LightTheme
	}
	if !ValidID(a.DarkTheme) {
		a.DarkTheme = d.DarkTheme
	}
	options := make(map[string]map[string]string, len(a.ThemeOptions))
	for id, values := range a.ThemeOptions {
		options[id] = maps.Clone(values)
	}
	a.ThemeOptions = options
	if a.Accent != "" && !colorRe.MatchString(a.Accent) {
		a.Accent = ""
	}
	switch a.ColorSource {
	case ColorsTheme, ColorsPywal, ColorsMatugen, ColorsFile:
	default:
		a.ColorSource = ColorsTheme
	}
	a.ColorFile = CleanPath(a.ColorFile)
	if len(a.ColorFile) > maxColorFile {
		a.ColorFile = ""
	}
	if a.Panel != "lyrics" && a.Panel != "none" {
		a.Panel = d.Panel
	}
	if a.Density != DensityCompact {
		a.Density = DensityComfortable
	}
	if !slices.Contains(Layouts, a.Layout) {
		a.Layout = d.Layout
	}
	if a.FontScale < 0.85 || a.FontScale > 1.3 {
		a.FontScale = d.FontScale
	}
	a.Zoom = zoomStep(a.Zoom, d.Zoom)
	return a
}

// ActiveTheme returns the ID of the theme to show: Theme, or the light or
// dark one of the computer's appearance when the theme follows it.
func (a Appearance) ActiveTheme(dark bool) string {
	switch {
	case !a.FollowSystem:
		return a.Theme
	case dark:
		return a.DarkTheme
	default:
		return a.LightTheme
	}
}

// ZoomSteps are the zooms of the interface, from 80% to 200%.
var ZoomSteps = []float64{0.8, 0.9, 1, 1.1, 1.25, 1.5, 1.75, 2}

// zoomStep is the step of ZoomSteps nearest z, or def when z is outside
// them, as in settings from before the zoom.
func zoomStep(z, def float64) float64 {
	if !(z >= ZoomSteps[0] && z <= ZoomSteps[len(ZoomSteps)-1]) {
		return def
	}
	near := ZoomSteps[0]
	for _, step := range ZoomSteps {
		if math.Abs(step-z) < math.Abs(near-z) {
			near = step
		}
	}
	return near
}
