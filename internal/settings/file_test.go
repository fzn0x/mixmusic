package settings

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func readTestdata(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// TestDecode reads settings files as versions of MixMusic and Windows
// PowerShell 5.1 wrote them.
func TestDecode(t *testing.T) {
	for _, c := range []struct {
		file  string
		check func(t *testing.T, s Settings)
	}{
		{"settings-0.5.5.json", func(t *testing.T, s Settings) {
			if s.Theme != "aurora" || s.Crossfade != 6 {
				t.Errorf("theme %q, crossfade %d", s.Theme, s.Crossfade)
			}
			// Keys of later versions take their defaults; Discord shows
			// what plays, as for every file of before 0.5.6 without an ID.
			if s.Zoom != 1 || !s.Loudness || !s.ShowInTitle || !s.DiscordPresence {
				t.Errorf("zoom %v, loudness %v, title %v, Discord %v", s.Zoom, s.Loudness, s.ShowInTitle, s.DiscordPresence)
			}
		}},
		// Out-File and > write UTF-16 with a byte order mark.
		{"powershell-utf16.json", func(t *testing.T, s Settings) {
			if s.Theme != "aurora" || s.Volume != 0.8 {
				t.Errorf("theme %q, volume %v", s.Theme, s.Volume)
			}
		}},
		// Set-Content -Encoding utf8 writes UTF-8 with a byte order mark.
		{"powershell-utf8-bom.json", func(t *testing.T, s Settings) {
			if s.Theme != "aurora" {
				t.Errorf("theme %q", s.Theme)
			}
		}},
		// Set-Content writes the system's code page, here Shift-JIS: the
		// file still reads, with replacement characters.
		{"powershell-codepage.json", func(t *testing.T, s Settings) {
			if s.Theme != "aurora" || !strings.HasSuffix(s.YTDLPPath, `\yt-dlp.exe`) || !strings.Contains(s.YTDLPPath, "\uFFFD") {
				t.Errorf("theme %q, yt-dlp %q", s.Theme, s.YTDLPPath)
			}
		}},
	} {
		t.Run(c.file, func(t *testing.T) {
			s, err := Decode(readTestdata(t, c.file))
			if err != nil {
				t.Fatal(err)
			}
			c.check(t, s)
		})
	}
}

// TestDecodeCleans reads a file as MixMusic does at start: missing keys
// take their defaults, and values out of range go back in range.
func TestDecodeCleans(t *testing.T) {
	s, err := Decode([]byte(`{"fontScale": 2, "crossfade": 60, "volume": 0.5, "shortcuts": {"player.next": "Mod+N"}}`))
	if err != nil {
		t.Fatal(err)
	}
	want := Default()
	want.Volume = 0.5
	want.Shortcuts = map[string]string{"player.next": "Mod+N"}
	want.Crossfade = 12
	if !reflect.DeepEqual(s, want.Clean()) {
		t.Errorf("Decode = %+v\nwant %+v", s, want.Clean())
	}
}

func TestDecodeErrors(t *testing.T) {
	for _, c := range []struct {
		name, text string
		line, col  int
		key, msg   string
	}{
		{"trailing comma", string(readTestdata(t, "trailing-comma.json")), 4, 1, "", "a comma before } has nothing after it"},
		{"trailing comma in a list", `{"enabledPlugins": ["a",]}`, 1, 25, "", "a comma before ] has nothing after it"},
		{"missing comma", "{\n  \"theme\": \"aurora\"\n  \"crossfade\": 6\n}", 3, 3, "", "a comma is missing before this"},
		{"missing comma after a number", `{"crossfade": 6 "theme": "aurora"}`, 1, 17, "", "a comma is missing before this"},
		{"comment", "{\n  // the theme\n  \"theme\": \"aurora\"\n}", 2, 3, "", "JSON has no comments"},
		{"single quotes", `{'theme': 'aurora'}`, 1, 2, "", "JSON takes double quotes"},
		{"key without quotes", `{theme: "aurora"}`, 1, 2, "", "a key takes double quotes"},
		{"text without quotes", `{"theme": aurora}`, 1, 11, "", "text takes double quotes"},
		{"misspelled word", `{"autoplay": tru}`, 1, 17, "", "text takes double quotes; only true, false and null go without"},
		{"missing colon", `{"theme" "aurora"}`, 1, 10, "", "a colon is missing after the key"},
		{"missing value", `{"theme": }`, 1, 11, "", "a value is missing before this"},
		{"backslash", `{"ytdlpPath": "C:\Users\me\yt-dlp.exe"}`, 1, 18, "", `a \ inside quotes is written \\`},
		{"line break in quotes", "{\"theme\": \"aur\nora\"}", 1, 15, "", "a line break inside quotes"},
		{"more after the end", `{"theme": "aurora"} {}`, 1, 21, "", "there is more after the last }"},
		{"truncated", "{\n  \"theme\": \"aurora\",\n  \"crossfade\": 6", 3, 17, "", "the file ends before its last }"},
		{"truncated in a string", `{"theme": "auro`, 1, 16, "", "the file ends before its last }"},
		{"empty", "", 0, 0, "", "it is empty"},
		{"blank", " \r\n\t", 0, 0, "", "it is empty"},
		{"null", "null", 1, 1, "", "it must hold one JSON object, from { to }"},
		{"list", "\n []", 2, 2, "", "it must hold one JSON object, from { to }"},
		{"wrong type", string(readTestdata(t, "wrong-type.json")), 0, 0, "volume", "volume should be a number"},
		{"wrong type in a list", `{"equalizer": {"gains": [0, "loud"]}}`, 0, 0, "equalizer.gains.1", "equalizer.gains.1 should be a number"},
		{"wrong type of a shortcut", `{"shortcuts": {"player.next": 1}}`, 0, 0, "shortcuts.player.next", "shortcuts.player.next should be text in quotes"},
		{"whole number", `{"crossfade": 2.5}`, 0, 0, "crossfade", "crossfade should be a whole number"},
		{"switch", `{"autoplay": "yes"}`, 0, 0, "autoplay", "autoplay should be true or false"},
		{"list of plugins", `{"enabledPlugins": "lyrics"}`, 0, 0, "enabledPlugins", "enabledPlugins should be a list in [ ]"},
	} {
		t.Run(c.name, func(t *testing.T) {
			s, err := Decode([]byte(c.text))
			var fe *FileError
			if !errors.As(err, &fe) {
				t.Fatalf("Decode = %v, want a FileError", err)
			}
			if fe.Line != c.line || fe.Column != c.col || fe.Key != c.key || fe.Msg != c.msg {
				t.Errorf("error = %+v, want line %d, column %d, key %q: %q", *fe, c.line, c.col, c.key, c.msg)
			}
			// No part of a file that doesn't read applies.
			if !reflect.DeepEqual(s, Settings{}) {
				t.Errorf("Decode gave settings too: %+v", s)
			}
			if strings.Contains(err.Error(), "plain") {
				t.Errorf("the error names the type it decodes into: %v", err)
			}
		})
	}
	err := &FileError{Line: 4, Column: 1, Msg: "a comma before } has nothing after it"}
	if got := err.Error(); got != "line 4, column 1: a comma before } has nothing after it" {
		t.Errorf("Error() = %q", got)
	}
}

func TestText(t *testing.T) {
	for _, c := range []struct {
		name string
		data []byte
		want string
		ok   bool
	}{
		{"UTF-8", []byte(`{"a": "é"}`), `{"a": "é"}`, true},
		{"UTF-8 with a BOM", []byte("\xEF\xBB\xBF{}"), "{}", true},
		{"UTF-16 LE", []byte("\xFF\xFE{\x00\xE9\x00}\x00"), "{é}", true},
		{"UTF-16 BE", []byte("\xFE\xFF\x00{\x00\xE9\x00}"), "{é}", true},
		{"UTF-16 cut short", []byte("\xFF\xFE{\x00}"), "{", false},
		{"code page", []byte("{\"a\": \"\x91\xBE\"}"), "{\"a\": \"\x91\xBE\"}", false},
	} {
		got, ok := Text(c.data)
		if string(got) != c.want || ok != c.ok {
			t.Errorf("%s: Text = %q, %v; want %q, %v", c.name, got, ok, c.want, c.ok)
		}
	}
}

func TestInspect(t *testing.T) {
	for _, c := range []struct {
		name              string
		text              string
		notUTF8           bool
		unknown, adjusted []string
	}{
		{"as kept", `{"theme": "aurora", "volume": 0.50, "crossfade": 2}`, false, nil, nil},
		{"a typo", `{"crosfade": 6, "theme": "aurora"}`, false, []string{"crosfade"}, nil},
		{"the key of before 0.5.6", `{"discordPresence": true}`, false, nil, nil},
		{"keys in another case", `{"Theme": "aurora", "FONTSCALE": 1.1}`, false, nil, nil},
		{"out of range", `{"fontScale": 2, "crossfade": 60, "theme": "Not A Theme"}`, false, nil, []string{"crossfade", "fontScale", "theme"}},
		{"nulls", `{"theme": null, "equalizer": null}`, false, nil, nil},
		{"part of the equalizer", `{"equalizer": {"enabled": true}}`, false, nil, nil},
		{"a gain out of range", `{"equalizer": {"gains": [20, 0, 0, 0, 0, 0, 0, 0, 0, 0]}}`, false, nil, []string{"equalizer.gains"}},
		{"too few gains", `{"equalizer": {"gains": [1]}}`, false, nil, []string{"equalizer.gains"}},
		{"a typo in the equalizer", `{"equalizer": {"enabld": true}}`, false, []string{"equalizer.enabld"}, nil},
		{"theme options", `{"themeOptions": {"mix": {"grain": "on"}}}`, false, nil, nil},
		{"shortcuts", `{"shortcuts": {"player.next": "Mod+N"}}`, false, nil, nil},
		{"plugins twice", `{"enabledPlugins": ["lyrics", "lyrics"]}`, false, nil, []string{"enabledPlugins"}},
		// The file and the settings both read the byte that isn't UTF-8 as
		// a replacement character.
		{"code page", "{\"ytdlpPath\": \"C:\\\\\x91\\\\yt-dlp\"}", true, nil, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			s, err := Decode([]byte(c.text))
			if err != nil {
				t.Fatal(err)
			}
			r := Inspect([]byte(c.text), s)
			if r.NotUTF8 != c.notUTF8 || !slices.Equal(r.Unknown, c.unknown) || !slices.Equal(r.Adjusted, c.adjusted) {
				t.Errorf("Inspect = %+v, want not UTF-8 %v, unknown %q, adjusted %q", r, c.notUTF8, c.unknown, c.adjusted)
			}
			if r.Empty() != (!c.notUTF8 && c.unknown == nil && c.adjusted == nil) {
				t.Errorf("Empty() = %v", r.Empty())
			}
		})
	}
	r := Inspect(readTestdata(t, "powershell-codepage.json"), Default())
	if !r.NotUTF8 {
		t.Error("the code page's file reads as UTF-8")
	}
}
