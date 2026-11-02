// Package colorfile reads the colours of a file that a tool makes of the
// wallpaper, for MixMusic to wear over the theme's: pywal's colors.json,
// matugen's JSON, and MixMusic's own tokens, as its matugen template and a
// theme.json write them. It tells them apart by what they hold.
package colorfile

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/fzn0x/mixmusic/internal/addons"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// MaxSize is the largest colour file MixMusic reads, in bytes.
const MaxSize = 1 << 20

// Format is the kind of a colour file.
type Format string

const (
	// FormatPywal is pywal's colors.json: its special colours, background,
	// foreground and cursor, and its 16 colours, color0 to color15.
	FormatPywal Format = "pywal"
	// FormatMaterial is matugen's JSON: Material colour roles, such as
	// surface, on_surface and primary.
	FormatMaterial Format = "material"
	// FormatTokens is MixMusic's theme tokens, such as bg, text and accent:
	// what its matugen template writes, or the colors of a theme.json.
	FormatTokens Format = "tokens"
)

// Colors are the colours of a file, by the names the file gives them.
// Their maps are never changed once made.
type Colors struct {
	Format Format
	// Modes holds the colours by mode: "" for a file with one set, "dark"
	// and "light" for one with a set of each.
	Modes map[string]map[string]string
	// Scheme is "dark" or "light" where the file says which it is, "" where
	// it doesn't.
	Scheme string
}

// Empty tells whether c holds no colours.
func (c Colors) Empty() bool { return len(c.Modes) == 0 }

// OneMode tells whether the file has a single set of colours, as pywal's
// and a theme.json's, which decides whether MixMusic is light or dark.
func (c Colors) OneMode() bool {
	_, dark := c.Modes["dark"]
	_, light := c.Modes["light"]
	return !c.Empty() && !(dark && light)
}

// Mode returns the colours of a mode, "dark" or "light": the file's of
// that mode, or its one set.
func (c Colors) Mode(mode string) map[string]string {
	if m, ok := c.Modes[mode]; ok {
		return m
	}
	if m, ok := c.Modes[""]; ok {
		return m
	}
	for _, other := range []string{"dark", "light"} {
		if m, ok := c.Modes[other]; ok {
			return m
		}
	}
	return nil
}

var (
	// ErrEmpty is a file with nothing in it, as a file is for a moment
	// while pywal and matugen write it again.
	ErrEmpty = errors.New("it's empty")
	// ErrPartial is a file that ends partway through its JSON, as one does
	// that is being written.
	ErrPartial = errors.New("it ends partway through")
	// ErrUnknown is a file that holds no colours MixMusic knows.
	ErrUnknown = errors.New("it holds no colours MixMusic knows. MixMusic reads pywal's colors.json, matugen's JSON, what MixMusic's matugen template writes, and the colors of a theme.json")
	// ErrFolder is a path of a folder.
	ErrFolder = errors.New("it's a folder")
	// ErrTooBig is a file over MaxSize.
	ErrTooBig = errors.New("it's over 1 MB")
)

// Read reads the colours of the file at path, and returns its bytes too.
// A file that isn't there gives an error that errors.Is fs.ErrNotExist.
func Read(path string) (Colors, []byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return Colors{}, nil, plain(err)
	}
	defer f.Close()
	if info, err := f.Stat(); err != nil {
		return Colors{}, nil, plain(err)
	} else if info.IsDir() {
		return Colors{}, nil, ErrFolder
	}
	data, err := io.ReadAll(io.LimitReader(f, MaxSize+1))
	if err != nil {
		return Colors{}, nil, plain(err)
	}
	if len(data) > MaxSize {
		return Colors{}, nil, ErrTooBig
	}
	c, err := Parse(data)
	return c, data, err
}

// plain is err without the path, which the settings show beside it.
func plain(err error) error {
	var pe *fs.PathError
	switch {
	case errors.Is(err, fs.ErrPermission):
		return fmt.Errorf("MixMusic may not read it: %w", fs.ErrPermission)
	case errors.As(err, &pe):
		return pe.Err
	}
	return err
}

// Parse reads the colours of a colour file's content.
func Parse(data []byte) (Colors, error) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	if len(bytes.TrimSpace(data)) == 0 {
		return Colors{}, ErrEmpty
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		// pywal 3.3.0 on Windows writes the wallpaper's path with its
		// backslashes as they are, which JSON doesn't allow.
		if pairs, ok := pywalPairs(data); ok {
			if c, err := pywalColors(pairs); err == nil {
				return c, nil
			}
		}
		return Colors{}, syntaxError(data, err)
	}
	if special, colors := object(top["special"]), object(top["colors"]); special != nil && colors != nil && colors["color0"] != nil {
		pairs := map[string]string{}
		read := func(m map[string]json.RawMessage, where, name string) error {
			raw, ok := m[name]
			if !ok {
				return nil
			}
			var v string
			if json.Unmarshal(raw, &v) != nil {
				return fmt.Errorf("%s.%s isn't a colour", where, name)
			}
			pairs[name] = v
			return nil
		}
		for _, name := range []string{"background", "foreground", "cursor"} {
			if err := read(special, "special", name); err != nil {
				return Colors{}, err
			}
		}
		for i := range 16 {
			if err := read(colors, "colors", "color"+strconv.Itoa(i)); err != nil {
				return Colors{}, err
			}
		}
		return pywalColors(pairs)
	}
	if c, ok, err := tokens(top); ok || err != nil {
		return c, err
	}
	if c, ok, err := matugen(top); ok || err != nil {
		return c, err
	}
	return Colors{}, ErrUnknown
}

// syntaxError tells where the JSON of data goes wrong.
func syntaxError(data []byte, err error) error {
	var se *json.SyntaxError
	var te *json.UnmarshalTypeError
	switch {
	case errors.As(err, &se) && se.Offset >= int64(len(bytes.TrimRight(data, " \t\r\n"))):
		return ErrPartial
	case errors.As(err, &se):
		line := 1 + bytes.Count(data[:min(se.Offset, int64(len(data)))], []byte("\n"))
		return fmt.Errorf("line %d: %v", line, se)
	case errors.As(err, &te):
		return ErrUnknown
	}
	return err
}

// object is the JSON object of raw, or nil.
func object(raw json.RawMessage) map[string]json.RawMessage {
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return nil
	}
	return m
}

// pywalPair is a colour of pywal's colors.json, found without reading it as
// JSON.
var pywalPair = regexp.MustCompile(`"(background|foreground|cursor|color\d{1,2})"\s*:\s*"([^"\\]*)"`)

// pywalPairs finds the colours of a colors.json that isn't valid JSON, as
// pywal 3.3.0 writes on Windows. A file cut short, as one being written,
// gives none: it doesn't end as colors.json does.
func pywalPairs(data []byte) (map[string]string, bool) {
	text := string(data)
	if !strings.Contains(text, `"special"`) || !strings.Contains(text, `"color0"`) || !strings.HasSuffix(strings.TrimSpace(text), "}") {
		return nil, false
	}
	pairs := map[string]string{}
	for _, m := range pywalPair.FindAllStringSubmatch(text, -1) {
		pairs[m[1]] = m[2]
	}
	return pairs, len(pairs) > 0
}

// pywalColors checks the colours of pywal's colors.json: it needs the
// background, the foreground and color1 to color6.
func pywalColors(pairs map[string]string) (Colors, error) {
	set := map[string]string{}
	for name, v := range pairs {
		where := "colors."
		if name == "background" || name == "foreground" || name == "cursor" {
			where = "special."
		}
		c, ok := colorValue(v)
		if !ok {
			return Colors{}, fmt.Errorf("%s%s isn't a colour: %q", where, name, v)
		}
		set[name] = c
	}
	for _, name := range []string{"background", "foreground", "color1", "color2", "color3", "color4", "color5", "color6"} {
		if _, ok := set[name]; !ok {
			where := "colors."
			if name == "background" || name == "foreground" {
				where = "special."
			}
			return Colors{}, fmt.Errorf("pywal's colors.json needs %s%s", where, name)
		}
	}
	return Colors{Format: FormatPywal, Modes: map[string]map[string]string{"": set}}, nil
}

// tokens reads MixMusic's theme tokens: the dark and light sets of its
// matugen template, the colors of a theme.json, or tokens at the top.
func tokens(top map[string]json.RawMessage) (Colors, bool, error) {
	isTokens := func(m map[string]json.RawMessage) bool { return m != nil && m["bg"] != nil }
	dark, light := object(top["dark"]), object(top["light"])
	// modes holds the sets by mode, and where says where each is, for the
	// errors.
	modes := map[string]map[string]json.RawMessage{}
	where := map[string]string{"dark": "dark.", "light": "light.", "": ""}
	switch {
	case isTokens(dark) || isTokens(light):
		if isTokens(dark) {
			modes["dark"] = dark
		}
		if isTokens(light) {
			modes["light"] = light
		}
	case isTokens(object(top["colors"])):
		modes[""], where[""] = object(top["colors"]), "colors."
	case isTokens(top):
		modes[""] = top
	default:
		return Colors{}, false, nil
	}
	c := Colors{Format: FormatTokens, Modes: map[string]map[string]string{}}
	if modes[""] != nil {
		c.Scheme = scheme(top)
	}
	for mode, m := range modes {
		where := where[mode]
		set := map[string]string{}
		for _, key := range addons.ColorKeys {
			raw, ok := m[key]
			if !ok {
				continue
			}
			var s string
			if json.Unmarshal(raw, &s) != nil {
				return Colors{}, true, fmt.Errorf("%s%s isn't a colour", where, key)
			}
			v, ok := colorValue(s)
			if !ok {
				return Colors{}, true, fmt.Errorf("%s%s isn't a colour: %q", where, key, s)
			}
			set[key] = v
		}
		if set["text"] == "" {
			return Colors{}, true, fmt.Errorf("%stext is missing, the ink on bg", where)
		}
		c.Modes[mode] = set
	}
	return c, true, nil
}

// materialRoles are the Material colour roles MixMusic wears.
var materialRoles = []string{
	"background", "on_background", "surface", "on_surface", "on_surface_variant",
	"surface_container_low", "surface_container", "surface_container_high", "surface_container_highest",
	"outline", "outline_variant", "primary", "on_primary", "secondary", "tertiary", "error", "source_color",
}

// matugen reads the JSON matugen prints with --json: in 2.x, its colors
// hold a dark and a light set of roles; in 3.x, each role holds its dark,
// light and default colour; in 4.x, each of those is {"color": ...}, unless
// --old-json-output writes them as 3.x did.
func matugen(top map[string]json.RawMessage) (Colors, bool, error) {
	colors := object(top["colors"])
	if colors == nil {
		return Colors{}, false, nil
	}
	modes := map[string]map[string]json.RawMessage{}
	if dark, light := object(colors["dark"]), object(colors["light"]); dark != nil && light != nil && dark["primary"] != nil {
		modes["dark"], modes["light"] = dark, light
	} else if primary := object(colors["primary"]); primary != nil && primary["dark"] != nil {
		for _, mode := range []string{"dark", "light"} {
			m := map[string]json.RawMessage{}
			for role, raw := range colors {
				if v := object(raw)[mode]; v != nil {
					m[role] = v
				}
			}
			modes[mode] = m
		}
	} else {
		return Colors{}, false, nil
	}
	c := Colors{Format: FormatMaterial, Modes: map[string]map[string]string{}, Scheme: scheme(top)}
	for mode, m := range modes {
		set := map[string]string{}
		for _, role := range materialRoles {
			raw, ok := m[role]
			if !ok {
				continue
			}
			var s string
			if json.Unmarshal(raw, &s) != nil {
				// 4.x's {"color": ...}
				var entry struct {
					Color *string `json:"color"`
				}
				if json.Unmarshal(raw, &entry) != nil || entry.Color == nil {
					return Colors{}, true, fmt.Errorf("colors.%s.%s isn't a colour", role, mode)
				}
				s = *entry.Color
			}
			v, ok := colorValue(s)
			if !ok {
				return Colors{}, true, fmt.Errorf("colors.%s.%s isn't a colour: %q", role, mode, s)
			}
			set[role] = v
		}
		if set["primary"] == "" || set["surface"] == "" && set["background"] == "" {
			return Colors{}, true, fmt.Errorf("matugen's colours need primary and surface, in %s", mode)
		}
		c.Modes[mode] = set
	}
	return c, true, nil
}

// scheme is the mode a file says its colours are: a theme's scheme, or
// matugen's mode.
func scheme(top map[string]json.RawMessage) string {
	for _, key := range []string{"scheme", "mode"} {
		var s string
		if json.Unmarshal(top[key], &s) == nil {
			if s = strings.ToLower(s); s == "dark" || s == "light" {
				return s
			}
		}
	}
	var dark bool
	if json.Unmarshal(top["is_dark_mode"], &dark) == nil {
		if dark {
			return "dark"
		}
		return "light"
	}
	return ""
}

var (
	// bareHex is a hex colour without its #, as matugen's strip format
	// writes it.
	bareHex = regexp.MustCompile(`^(?:[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)
	hexRe   = regexp.MustCompile(`^#(?:[0-9a-fA-F]{3,4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)
)

// colorValue is s as a colour a theme may set, or false.
func colorValue(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if bareHex.MatchString(s) {
		s = "#" + s
	}
	if strings.HasPrefix(s, "#") && !hexRe.MatchString(s) {
		return "", false
	}
	return s, addons.ValidColor(s)
}

// PywalPath is where pywal writes colors.json, of the environment getenv
// reads: $PYWAL_CACHE_DIR, else $XDG_CACHE_HOME/wal, where pywal writes
// since 3.3.1, else ~/.cache/wal, where 3.3.0 and older always write. Of
// the last two, it is the first that has a colors.json, or the first.
func PywalPath(getenv func(string) string, exists func(string) bool) string {
	if dir := getenv("PYWAL_CACHE_DIR"); dir != "" {
		return filepath.Join(dir, "colors.json")
	}
	var paths []string
	if dir := getenv("XDG_CACHE_HOME"); dir != "" {
		paths = append(paths, filepath.Join(dir, "wal", "colors.json"))
	}
	home := getenv("HOME")
	if home == "" {
		home = getenv("USERPROFILE")
	}
	if home != "" {
		paths = append(paths, filepath.Join(home, ".cache", "wal", "colors.json"))
	}
	for _, p := range paths {
		if exists(p) {
			return p
		}
	}
	if len(paths) == 0 {
		return ""
	}
	return paths[0]
}

// Expand is the full path of p, as the user typed or pasted it: ~ at its
// start is their home folder. A path that isn't full is refused.
func Expand(p string) (string, error) {
	p = settings.CleanPath(p)
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		p = filepath.Join(home, p[1:])
	}
	if !filepath.IsAbs(p) {
		root := "/"
		if runtime.GOOS == "windows" {
			root = `C:\`
		}
		return "", fmt.Errorf("it isn't a full path, which starts with ~ or %s", root)
	}
	return filepath.Clean(p), nil
}

// MatugenTemplate is MixMusic's template for matugen, which writes its
// colours for dark and light as MixMusic's tokens. Each brace of its JSON
// is on a line of its own: two together are how matugen's expressions
// start and end.
const MatugenTemplate = `{
  "dark":
  {
    "bg": "{{ colors.surface.dark.hex }}",
    "surface": "{{ colors.surface_container_low.dark.hex }}",
    "surface-2": "{{ colors.surface_container_high.dark.hex }}",
    "surface-3": "{{ colors.surface_container_highest.dark.hex }}",
    "border": "{{ colors.outline_variant.dark.hex }}",
    "text": "{{ colors.on_surface.dark.hex }}",
    "text-muted": "{{ colors.on_surface_variant.dark.hex }}",
    "accent": "{{ colors.primary.dark.hex }}",
    "accent-text": "{{ colors.on_primary.dark.hex }}",
    "danger": "{{ colors.error.dark.hex }}",
    "tile-1": "{{ colors.primary.dark.hex }}",
    "tile-2": "{{ colors.tertiary.dark.hex }}",
    "tile-3": "{{ colors.secondary.dark.hex }}",
    "tile-4": "{{ colors.source_color.dark.hex }}",
    "tile-text": "{{ colors.on_primary.dark.hex }}"
  },
  "light":
  {
    "bg": "{{ colors.surface.light.hex }}",
    "surface": "{{ colors.surface_container_low.light.hex }}",
    "surface-2": "{{ colors.surface_container_high.light.hex }}",
    "surface-3": "{{ colors.surface_container_highest.light.hex }}",
    "border": "{{ colors.outline_variant.light.hex }}",
    "text": "{{ colors.on_surface.light.hex }}",
    "text-muted": "{{ colors.on_surface_variant.light.hex }}",
    "accent": "{{ colors.primary.light.hex }}",
    "accent-text": "{{ colors.on_primary.light.hex }}",
    "danger": "{{ colors.error.light.hex }}",
    "tile-1": "{{ colors.primary.light.hex }}",
    "tile-2": "{{ colors.tertiary.light.hex }}",
    "tile-3": "{{ colors.secondary.light.hex }}",
    "tile-4": "{{ colors.source_color.light.hex }}",
    "tile-text": "{{ colors.on_primary.light.hex }}"
  }
}
`
