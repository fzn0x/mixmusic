package addons

import (
	"strings"
	"testing"
)

const validTheme = `{
	"id": "test-theme", "name": "Test", "version": "1.0.0", "author": "Someone", "scheme": "dark",
	"colors": {"bg": "#000", "surface": "#111111", "surface-2": "#222", "surface-3": "#333", "border": "rgba(255, 255, 255, 0.1)",
		"text": "#fff", "text-muted": "#aaa", "accent": "hsl(260 90% 65%)", "accent-text": "#fff",
		"danger": "#f44", "success": "#4f4", "youtube": "#f00", "spotify": "#1db954",
		"tile-1": "#00bde3", "tile-2": "#e6007e", "tile-3": "#a4e600", "tile-4": "#ffb000", "tile-text": "#000"},
	"fonts": {"body": "Inter, system-ui, sans-serif"},
	"radius": "12px",
	"preview": "preview.png",
	"options": [
		{"id": "glass", "label": "Frosted panels", "type": "toggle", "default": "true"},
		{"id": "layout", "label": "Layout", "type": "select", "default": "wide", "choices": [{"value": "wide", "label": "Wide"}, {"value": "narrow", "label": "Narrow"}]},
		{"id": "blur", "label": "Blur", "type": "range", "default": "12", "min": 0, "max": 40, "unit": "px"}
	],
	"files": ["theme.json", "preview.png"]
}`

func TestParseThemeValid(t *testing.T) {
	th, err := ParseTheme([]byte(validTheme))
	if err != nil {
		t.Fatal(err)
	}
	if th.ID != "test-theme" || len(th.Options) != 3 {
		t.Errorf("theme = %+v", th)
	}
}

func TestParseThemeRejects(t *testing.T) {
	for name, edit := range map[string][2]string{
		"bad id":           {`"id": "test-theme"`, `"id": "../evil"`},
		"missing color":    {`"spotify": "#1db954"`, `"spotifyy": "#1db954"`},
		"css injection":    {`"bg": "#000"`, `"bg": "#000; } body { display: none"`},
		"bad font":         {`"Inter, system-ui, sans-serif"`, `"Inter; } * { color: red"`},
		"preview escape":   {`"preview": "preview.png"`, `"preview": "../../preview.png"`},
		"unlisted preview": {`"files": ["theme.json", "preview.png"]`, `"files": ["theme.json"]`},
		"bad option type":  {`"type": "toggle"`, `"type": "slider"`},
		"default not in":   {`"default": "wide"`, `"default": "huge"`},
		"bad version":      {`"version": "1.0.0"`, `"version": "v1"`},
		"bad scheme":       {`"scheme": "dark"`, `"scheme": "dim"`},
		"unknown unit":     {`"unit": "px"`, `"unit": "vw; x"`},
		"radius injection": {`"radius": "12px"`, `"radius": "12px; color: red"`},
	} {
		bad := strings.Replace(validTheme, edit[0], edit[1], 1)
		if bad == validTheme {
			t.Fatalf("%s: the edit does not apply", name)
		}
		if _, err := ParseTheme([]byte(bad)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestParsePlugin(t *testing.T) {
	good := `{"id": "sleep-timer", "name": "Sleep Timer", "version": "1.2.0", "author": "Someone", "main": "index.js",
		"permissions": ["playback"], "files": ["plugin.json", "index.js"]}`
	if _, err := ParsePlugin([]byte(good)); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{
		strings.Replace(good, `"main": "index.js"`, `"main": "../index.js"`, 1),
		strings.Replace(good, `"main": "index.js"`, `"main": "index.ts"`, 1),
		strings.Replace(good, `["playback"]`, `["root"]`, 1),
		strings.Replace(good, `"files": ["plugin.json", "index.js"]`, `"files": ["plugin.json"]`, 1),
	} {
		if _, err := ParsePlugin([]byte(bad)); err == nil {
			t.Errorf("accepted %s", bad)
		}
	}
}

func TestValidFile(t *testing.T) {
	for name, want := range map[string]bool{
		"theme.css": true, "img/preview.png": true, "a-b_c.js": true,
		"": false, "../x": false, "/abs": false, "a//b": false, "./a": false, ".hidden": false, `a\b`: false, "a/../b": false,
	} {
		if got := ValidFile(name); got != want {
			t.Errorf("ValidFile(%q) = %v", name, got)
		}
	}
}
