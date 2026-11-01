package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// typeKeys types text into the focused input a letter at a time, a frame
// each, as a user does.
func typeKeys(s *shell, tt *ui.Tester, text string) {
	for _, r := range text {
		tt.Type(string(r))
		s.drain()
		tt.Frame()
	}
}

// boxFocused tells whether the box that searches the settings has the
// keyboard focus, rather than the page around it.
func boxFocused(tt *ui.Tester) bool {
	return tt.Focused("Search the settings") && !tt.Focused("settings")
}

// showsOnly fails unless the window shows every text of want, and none of
// not.
func showsOnly(t *testing.T, tt *ui.Tester, what string, want, not []string) {
	t.Helper()
	for _, w := range want {
		if !tt.HasText(w) {
			t.Errorf("%s doesn't show %q: %q", what, w, tt.Texts())
		}
	}
	for _, n := range not {
		if tt.HasText(n) {
			t.Errorf("%s shows %q", what, n)
		}
	}
}

// TestSettingsFind types in the box of the settings, which shows what
// every section holds of what was typed, and is the location's query;
// Escape clears it, and shows the section again.
func TestSettingsFind(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 2400)
	show(s, tt, "/settings/appearance")
	if boxFocused(tt) {
		t.Error("opening the settings focused the box")
	}
	click(t, s, tt, "Search the settings")
	typeKeys(s, tt, "crossfade")
	tt.Advance(time.Second)
	if got := s.router.Location(); got != "/settings/appearance?q=crossfade" {
		t.Errorf("the location is %q", got)
	}
	if !boxFocused(tt) {
		t.Error("the box lost the focus as the user typed")
	}
	showsOnly(t, tt, "searching crossfade", []string{"sound", "Crossfade"}, []string{"Volume levelling", "Follow the artwork", "Preset"})

	tt.Key(0, ui.KeyEscape)
	s.drain()
	tt.Frame()
	tt.Advance(time.Second)
	if got := s.router.Location(); got != "/settings/appearance" {
		t.Errorf("after Escape the location is %q", got)
	}
	showsOnly(t, tt, "the search cleared", []string{"Follow the artwork"}, []string{"Crossfade"})
	if s.find.query != "" {
		t.Errorf("the box holds %q", s.find.query)
	}
}

// TestSettingsFindMatches searches the settings for words that are in a
// group's title, a section's name, a row's title, description or words, a
// theme's name and nowhere.
func TestSettingsFindMatches(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 6000)
	for _, c := range []struct {
		q         string
		want, not []string
	}{
		{"equalizer", []string{"Equalizer", "Preset"}, []string{"Crossfade"}},
		{"desktop", []string{"Close to the tray", "Reset settings", "Start at login"}, []string{"Crossfade"}},
		{"TRAY", []string{"Close to the tray"}, []string{"Media keys", "Crossfade"}},
		{"aurora", []string{"Aurora", "Theme: Aurora"}, []string{"Mochi", "Crossfade"}},
		{"lévelling", []string{"Volume levelling"}, []string{"Crossfade"}},
		{"follow each other", []string{"Crossfade"}, []string{"Volume levelling"}},
		{"zzzz", []string{"No setting matches “zzzz”."}, []string{"Crossfade", "Close to the tray"}},
	} {
		show(s, tt, settingsPath("", c.q))
		showsOnly(t, tt, "searching "+c.q, c.want, c.not)
		show(s, tt, "/")
	}
}

// TestSettingsFindLocation opens the settings searched: the search is
// part of their location, which Back and Forward bring back, while the
// settings opened as usual show unsearched.
func TestSettingsFindLocation(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 2400)
	show(s, tt, settingsPath("playback", "tray"))
	if s.find.query != "tray" || !tt.HasText("Close to the tray") || tt.HasText("Crossfade") {
		t.Fatalf("the box holds %q; the page shows %q", s.find.query, tt.Texts())
	}
	show(s, tt, "/explore")
	click(t, s, tt, "Back")
	tt.Advance(time.Second)
	if got := s.router.Location(); got != "/settings/playback?q=tray" || s.find.query != "tray" {
		t.Errorf("Back went to %q, the box holding %q", got, s.find.query)
	}
	click(t, s, tt, "settings")
	tt.Advance(time.Second)
	if got := s.router.Location(); got != "/settings" || s.find.query != "" {
		t.Errorf("the rail's settings went to %q, the box holding %q", got, s.find.query)
	}
	if !tt.HasText("Follow the artwork") || tt.HasText("Close to the tray") {
		t.Errorf("the settings opened show %q", tt.Texts())
	}
}

// TestSettingsFindPivot leaves the search for the section chosen.
func TestSettingsFindPivot(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 2400)
	show(s, tt, settingsPath("appearance", "crossfade"))
	click(t, s, tt, "desktop")
	tt.Advance(time.Second)
	if got := s.router.Location(); got != "/settings/desktop" {
		t.Errorf("the pivot went to %q", got)
	}
	showsOnly(t, tt, "the pivot", []string{"Close to the tray"}, []string{"Crossfade"})
	if s.find.query != "" {
		t.Errorf("the box still holds %q", s.find.query)
	}
}

// TestSettingsFindKeyboard changes a setting the search found with the
// keyboard: Tab goes from the box into what it found.
func TestSettingsFindKeyboard(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 2400)
	show(s, tt, "/settings")
	click(t, s, tt, "Search the settings")
	typeKeys(s, tt, "levelling")
	before := s.svc.Preferences.Get().Normalize
	focused := func() bool { return tt.Focused("Volume levelling") && !tt.Focused("settings") }
	for range 3 {
		if focused() {
			break
		}
		tt.Key(0, ui.KeyTab)
		tt.Frame()
	}
	if !focused() {
		t.Fatal("Tab doesn't go from the box to the switch found")
	}
	tt.Key(0, ui.KeySpace)
	s.drain()
	tt.Frame()
	if s.svc.Preferences.Get().Normalize == before {
		t.Error("Space on the switch the search found changed nothing")
	}
	if s.p.State().Playing {
		t.Error("Space played")
	}
}

// TestSettingsFindEverything searches for what nearly every setting holds:
// every section builds at once, without two of their elements sharing a
// key, and a plugin installed is found.
func TestSettingsFindEverything(t *testing.T) {
	s := newTestShell(t)
	dir := filepath.Join(s.svc.System.Info().DataDir, "addons", "plugins", "hello")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string]string{
		"plugin.json": `{"id": "hello", "name": "Hello", "version": "1.0.0", "author": "A", "description": "Says hello", "main": "index.js"}`,
		"index.js":    "export default () => {}",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s.svc.Addons.Reload()
	s.refresh()
	tt := testerOf(t, s, 1280, 12000)
	for _, q := range []string{"e", "a", "s", "says hello"} {
		show(s, tt, settingsPath("playback", q))
		if !tt.HasText("Hello 1.0.0") {
			t.Errorf("searching %q doesn't find the plugin: %q", q, tt.Texts())
		}
	}
}
