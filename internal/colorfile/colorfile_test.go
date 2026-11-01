package colorfile

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/fzn0x/mixmusic/internal/addons"
)

// The files of testdata are made by hand in the shapes pywal 3.3.0 and
// 3.8.15 write colors.json, and matugen 2.4.1, 3.1.0 and 4.2.0 print with
// --json: from their sources, as neither tool ran here.

func read(t *testing.T, name string) Colors {
	t.Helper()
	c, data, err := Read(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if len(data) == 0 {
		t.Fatalf("%s: no bytes", name)
	}
	return c
}

func TestPywal(t *testing.T) {
	for _, c := range []struct {
		file       string
		background string
		color8     string
	}{
		{"pywal-3.8.json", "#0d1117", "#5d6064"},
		{"pywal-light.json", "#eee9e3", "#8d847b"},
		// pywal 3.3.0 on Windows leaves the backslashes of the wallpaper's
		// path as they are, which JSON doesn't allow.
		{"pywal-3.3.0-windows.json", "#0d1117", "#5d6064"},
	} {
		got := read(t, c.file)
		if got.Format != FormatPywal || !got.OneMode() || got.Scheme != "" {
			t.Errorf("%s: %+v", c.file, got)
			continue
		}
		m := got.Mode("dark")
		if m["background"] != c.background || m["color8"] != c.color8 || len(m) != 19 {
			t.Errorf("%s: background %q, color8 %q, %d colours", c.file, m["background"], m["color8"], len(m))
		}
		if got.Mode("light")["foreground"] != m["foreground"] {
			t.Errorf("%s: the modes of a file with one set differ", c.file)
		}
	}
	data, err := os.ReadFile(filepath.Join("testdata", "pywal-3.3.0-windows.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(`C:\Users`)) {
		t.Fatal("the 3.3.0 file has lost its backslashes")
	}
}

func TestMatugen(t *testing.T) {
	for _, c := range []struct {
		file, scheme string
		// sourceDark is set where the file has source_color in the dark set.
		sourceDark bool
	}{
		{"matugen-4.2.json", "dark", true},
		{"matugen-3.1.json", "light", true},
		{"matugen-2.4.json", "", false},
	} {
		got := read(t, c.file)
		if got.Format != FormatMaterial || got.OneMode() || got.Scheme != c.scheme {
			t.Errorf("%s: format %q, one mode %v, scheme %q", c.file, got.Format, got.OneMode(), got.Scheme)
			continue
		}
		dark, light := got.Mode("dark"), got.Mode("light")
		if dark["surface"] != "#141218" || dark["primary"] != "#d0bcff" || light["surface"] != "#fef7ff" || light["on_primary"] != "#ffffff" {
			t.Errorf("%s: dark %v, light %v", c.file, dark, light)
		}
		if _, ok := dark["source_color"]; ok != c.sourceDark {
			t.Errorf("%s: source_color in the dark set: %v", c.file, ok)
		}
		if light["source_color"] != "#6750a4" {
			t.Errorf("%s: light source_color %q", c.file, light["source_color"])
		}
		// Custom colours of matugen's config, and the roles MixMusic
		// doesn't wear, aren't kept.
		for _, role := range []string{"red", "on_red", "scrim", "surface_tint"} {
			if _, ok := dark[role]; ok {
				t.Errorf("%s: kept %s", c.file, role)
			}
		}
	}
}

func TestTokens(t *testing.T) {
	tpl := read(t, "matugen-template.json")
	if tpl.Format != FormatTokens || tpl.OneMode() || len(tpl.Mode("dark")) != 15 || len(tpl.Mode("light")) != 15 {
		t.Errorf("the template's output: %+v", tpl)
	}
	if tpl.Mode("dark")["bg"] != "#141218" || tpl.Mode("light")["bg"] != "#fef7ff" {
		t.Errorf("the template's grounds: %q %q", tpl.Mode("dark")["bg"], tpl.Mode("light")["bg"])
	}
	theme := read(t, "theme.json")
	if theme.Format != FormatTokens || !theme.OneMode() || theme.Scheme != "light" || len(theme.Mode("")) != len(addons.ColorKeys) {
		t.Errorf("a theme.json: %+v", theme)
	}
	if theme.Mode("dark")["border"] != "rgba(30, 50, 42, 0.12)" {
		t.Errorf("a theme's border: %q", theme.Mode("dark")["border"])
	}
	flat, err := Parse([]byte(`{"bg": "#101010", "text": "#f0f0f0", "accent": "ff8800", "comment": "mine"}`))
	if err != nil || flat.Format != FormatTokens || flat.Scheme != "" || flat.Mode("")["accent"] != "#ff8800" || len(flat.Mode("")) != 3 {
		t.Errorf("tokens at the top: %+v, %v", flat, err)
	}
}

// TestBad gives an error for a file that holds no colours MixMusic can
// wear, which says why.
func TestBad(t *testing.T) {
	whole, err := os.ReadFile(filepath.Join("testdata", "pywal-3.8.json"))
	if err != nil {
		t.Fatal(err)
	}
	windows, err := os.ReadFile(filepath.Join("testdata", "pywal-3.3.0-windows.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name string
		data string
		want error
		says string
	}{
		{"empty", "", ErrEmpty, ""},
		{"blank", " \r\n\t", ErrEmpty, ""},
		{"byte order mark only", "\xef\xbb\xbf", ErrEmpty, ""},
		{"cut at 60%", string(whole[:len(whole)*6/10]), ErrPartial, ""},
		// The lenient read of 3.3.0's file never takes one cut short.
		{"3.3.0's cut at 80%", string(windows[:len(windows)*8/10]), nil, "line 2"},
		{"unknown names", `{"foo": "#ffffff", "colors": {"a": "#000000"}}`, ErrUnknown, ""},
		{"an array", `["#000000"]`, ErrUnknown, ""},
		{"a colour that isn't one", strings.Replace(string(whole), `"color3": "#7a8fa3"`, `"color3": "steelblue"`, 1), nil, `colors.color3 isn't a colour: "steelblue"`},
		{"a number for a colour", strings.Replace(string(whole), `"color3": "#7a8fa3"`, `"color3": 7`, 1), nil, "colors.color3 isn't a colour"},
		{"a pywal file without color4", strings.Replace(string(whole), `"color4": "#c2734b",`, ``, 1), nil, "pywal's colors.json needs colors.color4"},
		{"five hex digits", `{"bg": "#12345", "text": "#ffffff"}`, nil, `bg isn't a colour: "#12345"`},
		{"a theme's colour that isn't one", `{"scheme": "dark", "colors": {"bg": "#000000", "text": "white"}}`, nil, `colors.text isn't a colour: "white"`},
		{"tokens without text", `{"dark": {"bg": "#000000"}, "light": {"bg": "#ffffff", "text": "#000000"}}`, nil, "dark.text is missing"},
		{"matugen without primary", `{"colors": {"surface": {"dark": "#000000", "light": "#ffffff"}, "primary": {"dark": "#ffffff"}}}`, nil, "need primary and surface, in light"},
		{"a syntax error", "{\n  \"bg\": \"#000000\",\n  oops\n}", nil, "line 3:"},
	} {
		_, err := Parse([]byte(c.data))
		switch {
		case err == nil:
			t.Errorf("%s: no error", c.name)
		case c.want != nil && !errors.Is(err, c.want):
			t.Errorf("%s: %v, want %v", c.name, err, c.want)
		case c.says != "" && !strings.Contains(err.Error(), c.says):
			t.Errorf("%s: %q doesn't say %q", c.name, err, c.says)
		}
	}
}

func TestRead(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := Read(filepath.Join(dir, "colors.json")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a file that isn't there: %v", err)
	}
	if _, _, err := Read(dir); !errors.Is(err, ErrFolder) {
		t.Errorf("a folder: %v", err)
	}
	big := filepath.Join(dir, "big.json")
	if err := os.WriteFile(big, bytes.Repeat([]byte(" "), MaxSize+1), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Read(big); !errors.Is(err, ErrTooBig) {
		t.Errorf("a file over 1 MB: %v", err)
	}
	// One that is 1 MB at most is read.
	whole, err := os.ReadFile(filepath.Join("testdata", "pywal-3.8.json"))
	if err != nil {
		t.Fatal(err)
	}
	padded := append(bytes.Repeat([]byte(" "), MaxSize-len(whole)), whole...)
	if err := os.WriteFile(big, padded, 0o644); err != nil {
		t.Fatal(err)
	}
	if c, data, err := Read(big); err != nil || c.Format != FormatPywal || len(data) != MaxSize {
		t.Errorf("a file of 1 MB: %v, %d bytes", err, len(data))
	}
	// Errors don't name the path, which the settings show beside them.
	if _, _, err := Read(filepath.Join(dir, "nowhere", "colors.json")); err == nil || strings.Contains(err.Error(), dir) {
		t.Errorf("the error names the path: %v", err)
	}
}

func TestPywalPath(t *testing.T) {
	sep := string(filepath.Separator)
	home, xdg := sep+"home", sep+"xdg"
	walHome := filepath.Join(home, ".cache", "wal", "colors.json")
	walXDG := filepath.Join(xdg, "wal", "colors.json")
	for _, c := range []struct {
		name  string
		env   map[string]string
		there []string
		want  string
	}{
		{"PYWAL_CACHE_DIR", map[string]string{"PYWAL_CACHE_DIR": sep + "wal", "XDG_CACHE_HOME": xdg, "HOME": home}, []string{walXDG, walHome}, filepath.Join(sep+"wal", "colors.json")},
		{"HOME alone", map[string]string{"HOME": home}, nil, walHome},
		{"USERPROFILE alone, as on Windows", map[string]string{"USERPROFILE": home}, nil, walHome},
		{"XDG_CACHE_HOME, nothing there yet", map[string]string{"XDG_CACHE_HOME": xdg, "HOME": home}, nil, walXDG},
		{"XDG_CACHE_HOME, with pywal 3.3.0's file in ~/.cache", map[string]string{"XDG_CACHE_HOME": xdg, "HOME": home}, []string{walHome}, walHome},
		{"XDG_CACHE_HOME, with the file there", map[string]string{"XDG_CACHE_HOME": xdg, "HOME": home}, []string{walXDG}, walXDG},
		{"both, the newer pywal's first", map[string]string{"XDG_CACHE_HOME": xdg, "HOME": home}, []string{walHome, walXDG}, walXDG},
		{"nothing", map[string]string{}, nil, ""},
	} {
		got := PywalPath(func(k string) string { return c.env[k] }, func(p string) bool {
			for _, there := range c.there {
				if p == there {
					return true
				}
			}
			return false
		})
		if got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

func TestExpand(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	abs := filepath.Join(home, "my colours.json")
	for in, want := range map[string]string{
		"~/colors.json":   filepath.Join(home, "colors.json"),
		`~\colors.json`:   filepath.Join(home, "colors.json"),
		"~":               home,
		abs:               abs,
		` "` + abs + `" `: abs,
		filepath.Join(home, "x", "..", "colors.json"): filepath.Join(home, "colors.json"),
	} {
		if runtime.GOOS != "windows" && strings.Contains(in, `\`) {
			continue // a backslash is a letter of a name there
		}
		got, err := Expand(in)
		if err != nil || got != want {
			t.Errorf("Expand(%q) = %q, %v, want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"colors.json", "./colors.json", "~me/colors.json", ""} {
		if got, err := Expand(in); err == nil {
			t.Errorf("Expand(%q) = %q, with no error", in, got)
		}
	}
}

// TestMatugenTemplate renders MixMusic's template, every expression as a
// colour, into the tokens of a dark and a light set, and keeps matugen's
// delimiters, {{ and }}, to its expressions.
func TestMatugenTemplate(t *testing.T) {
	expr := regexp.MustCompile(`\{\{ colors\.[a-z_]+\.(dark|light)\.hex \}\}`)
	if n := len(expr.FindAllString(MatugenTemplate, -1)); n != 30 {
		t.Errorf("%d expressions, want 30", n)
	}
	rest := expr.ReplaceAllString(MatugenTemplate, "#123456")
	if strings.Contains(rest, "{{") || strings.Contains(rest, "}}") {
		t.Errorf("braces that matugen reads as an expression:\n%s", rest)
	}
	c, err := Parse([]byte(rest))
	if err != nil || c.Format != FormatTokens || len(c.Mode("dark")) != 15 || len(c.Mode("light")) != 15 {
		t.Fatalf("the rendered template: %+v, %v", c, err)
	}
	for _, key := range []string{"success", "youtube", "spotify"} {
		if _, ok := c.Mode("dark")[key]; ok {
			t.Errorf("the template sets %s, which stays the theme's", key)
		}
	}
	for _, mode := range []string{"dark", "light"} {
		if strings.Count(MatugenTemplate, "."+mode+".hex") != 15 {
			t.Errorf("the %s set doesn't take its colours from matugen's %s ones", mode, mode)
		}
	}
}

// TestMatugenTemplateDocs finds the template in docs/themes.md as
// MatugenTemplate has it, which Settings copies.
func TestMatugenTemplateDocs(t *testing.T) {
	doc, err := os.ReadFile(filepath.Join("..", "..", "docs", "themes.md"))
	if err != nil {
		t.Fatal(err)
	}
	doc = bytes.ReplaceAll(doc, []byte("\r\n"), []byte("\n"))
	if !bytes.Contains(doc, []byte("```json\n"+MatugenTemplate+"```\n")) {
		t.Error("docs/themes.md doesn't give the template as MatugenTemplate has it")
	}
}
