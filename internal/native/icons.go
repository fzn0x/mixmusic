package native

import "github.com/egoist/mygo/ui"

// MixMusic's icons, as the web interface had them: thin line glyphs with
// square ends on a 24-unit grid, drawn in the text colour.
var iconPaths = map[string]string{
	"home":         "M3.5 11 12 3.5l8.5 7.5M5.5 9.5V20h5v-6h3v6h5V9.5",
	"search":       "M10.5 17a6.5 6.5 0 1 0 0-13 6.5 6.5 0 0 0 0 13ZM15.2 15.2 20.5 20.5",
	"library":      "M4 20V4M8 20V4M12.5 20 11 4.5M16 20l3.5-15",
	"store":        "M3.5 3.5h7v7h-7zM13.5 3.5h7v7h-7zM3.5 13.5h7v7h-7zM13.5 13.5h7v7h-7z",
	"settings":     "M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6ZM10.1 5.4 10.6 3h2.8l.5 2.4 2.9 1.7 2.3-.8 1.4 2.4-1.8 1.6v3.4l1.8 1.6-1.4 2.4-2.3-.8-2.9 1.7-.5 2.4h-2.8l-.5-2.4-2.9-1.7-2.3.8-1.4-2.4 1.8-1.6v-3.4L3.5 8.7l1.4-2.4 2.3.8z",
	"stats":        "M4 20V12M9.33 20V6M14.67 20v-9M20 20V4M3 20.5h18",
	"play":         "M7 4.5v15l12.5-7.5z",
	"pause":        "M7 4.5v15M17 4.5v15",
	"next":         "M5.5 5v14l10-7zM18.5 5v14",
	"previous":     "M18.5 5v14l-10-7zM5.5 5v14",
	"shuffle":      "M3 7h3.5c3.5 0 4.5 10 8 10H21M17.5 13.5 21 17l-3.5 3.5M3 17h3.5c1.5 0 2.4-1.8 3.2-4M17.5 3.5 21 7l-3.5 3.5M13.3 9c.7-1.2 1.5-2 2.7-2h5",
	"repeat":       "M4 11V8.5A2.5 2.5 0 0 1 6.5 6H19M16 3l3 3-3 3M20 13v2.5a2.5 2.5 0 0 1-2.5 2.5H5M8 21l-3-3 3-3",
	"repeatOne":    "M4 11V8.5A2.5 2.5 0 0 1 6.5 6H19M16 3l3 3-3 3M20 13v2.5a2.5 2.5 0 0 1-2.5 2.5H5M8 21l-3-3 3-3M11.5 10.5 13 9.5v5",
	"heart":        "M12 20.5S3.5 15.2 3.5 9.2A4.7 4.7 0 0 1 12 6.6a4.7 4.7 0 0 1 8.5 2.6c0 6-8.5 11.3-8.5 11.3Z",
	"queue":        "M3.5 6h11M3.5 11h11M3.5 16h7M17 13.5v7l4.5-3.5z",
	"lyrics":       "M4.5 5h15v10.5H11l-4.5 4v-4h-2zM8 9h8M8 12h5",
	"volume":       "M4 9.5h3.5L12 5.5v13l-4.5-4H4zM15.5 9a4.5 4.5 0 0 1 0 6M18 6.5a8 8 0 0 1 0 11",
	"volumeMute":   "M4 9.5h3.5L12 5.5v13l-4.5-4H4zM16 9.5l5 5M21 9.5l-5 5",
	"plus":         "M12 4.5v15M4.5 12h15",
	"more":         "M5.5 12h.01M12 12h.01M18.5 12h.01",
	"close":        "M5.5 5.5l13 13M18.5 5.5l-13 13",
	"back":         "M14.5 5.5 8 12l6.5 6.5M8.5 12H20",
	"forward":      "M9.5 5.5 16 12l-6.5 6.5M15.5 12H4",
	"mini":         "M3.5 5h17v14h-17zM11.5 12h7v5h-7z",
	"check":        "M4.5 12.5 9.5 17.5 19.5 6.5",
	"chevronRight": "M9.5 5.5 16 12l-6.5 6.5",
	"chevronDown":  "M5.5 9.5 12 16l6.5-6.5",
	"external":     "M13.5 4h6.5v6.5M20 4l-9 9M17.5 14v6h-14V6.5h6",
	"download":     "M12 3.5v12M7 10.5l5 5 5-5M4 20.5h16",
	"trash":        "M4 6.5h16M9.5 6.5V4h5v2.5M6 6.5l1 14h10l1-14M10 10.5v6M14 10.5v6",
	"radio":        "M12 13.5a1.5 1.5 0 1 0 0-3 1.5 1.5 0 0 0 0 3ZM8 16a5.6 5.6 0 0 1 0-8M16 8a5.6 5.6 0 0 1 0 8M5 19a10 10 0 0 1 0-14M19 5a10 10 0 0 1 0 14",
	"equalizer":    "M5 4v16M12 4v16M19 4v16M2.5 14h5M9.5 8h5M16.5 16h5",
	"palette":      "M12 3.5a8.5 8.5 0 0 0 0 17c1.2 0 1.8-.8 1.8-1.7 0-1.4-1.2-1.7-1.2-3 0-1 .8-1.6 1.8-1.6H17a3.5 3.5 0 0 0 3.5-3.5c0-4-3.8-7.2-8.5-7.2ZM7.5 11.5h.01M9.5 7.5h.01M14.5 7.5h.01",
	"puzzle":       "M9.5 4.5a2 2 0 1 1 4 0V6H18v4.5h-1.5a2 2 0 1 0 0 4H18V19h-4.5v-1.5a2 2 0 1 0-4 0V19H5v-4.5h1.5a2 2 0 1 0 0-4H5V6h4.5z",
	"user":         "M12 12a4 4 0 1 0 0-8 4 4 0 0 0 0 8ZM4.5 20.5c0-4 3.4-6.5 7.5-6.5s7.5 2.5 7.5 6.5",
	"devices":      "M3.5 5h12v9h-12zM6.5 19h6M9.5 14v5M17.5 9h3v11h-3z",
	"refresh":      "M19.5 12a7.5 7.5 0 1 1-2.2-5.3M19.5 4v4.5H15",
	"folder":       "M3.5 6.5v13h17v-11h-9L9.5 6.5z",
	"alert":        "M12 3.5 21 19.5H3zM12 10v4.5M12 17h.01",
	"compass":      "M12 20.5a8.5 8.5 0 1 0 0-17 8.5 8.5 0 0 0 0 17ZM15.5 8.5l-2 5-5 2 2-5z",
	"disc":         "M12 20.5a8.5 8.5 0 1 0 0-17 8.5 8.5 0 0 0 0 17ZM12 14a2 2 0 1 0 0-4 2 2 0 0 0 0 4Z",
	"mic":          "M9 4.5h6v9H9zM5.5 11.5a6.5 6.5 0 0 0 13 0M12 18v2.5",
	"link":         "M10 14a4 4 0 0 0 5.7 0l3-3a4 4 0 0 0-5.7-5.7l-1 1M14 10a4 4 0 0 0-5.7 0l-3 3a4 4 0 0 0 5.7 5.7l1-1",
	"edit":         "M4 20h4l11-11-4-4L4 16zM13 7l4 4",
	"sidebar":      "M3.5 4.5h17v15h-17zM9 4.5v15",
}

// icons are iconPaths drawn as lines, and solidIcons as filled shapes, as
// the web interface's play glyph and a liked heart.
var icons, solidIcons = makeIcons()

func makeIcons() (lines, solid map[string]*ui.SVG) {
	lines, solid = map[string]*ui.SVG{}, map[string]*ui.SVG{}
	for name, d := range iconPaths {
		lines[name] = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="square" stroke-linejoin="miter"><path d="` + d + `"/></svg>`))
		solid[name] = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" stroke="currentColor" stroke-width="1.6" stroke-linecap="square" stroke-linejoin="miter"><path d="` + d + `"/></svg>`))
	}
	return lines, solid
}

// icon returns the icon name, drawn as the web interface draws it: play
// is solid.
func icon(name string) *ui.SVG {
	if name == "play" {
		return solidIcons[name]
	}
	if s, ok := icons[name]; ok {
		return s
	}
	return icons["puzzle"]
}
