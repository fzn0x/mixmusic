package main

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/spotify"
	"github.com/fzn0x/mixmusic/internal/tools"
	"github.com/fzn0x/mixmusic/internal/youtube"
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
	// The palette's keys, as the user would type them.
	palette := "ctrl+k"
	if runtime.GOOS == "darwin" {
		palette = "cmd+k"
	}
	for _, c := range []struct {
		q         string
		want, not []string
	}{
		{"equalizer", []string{"Equalizer", "Preset", "pre dB", "Spotify notes"}, []string{"Crossfade"}},
		{"bass", []string{"Preset"}, []string{"Crossfade", "pre dB"}},
		{"preamp", []string{"Equalizer", "pre dB"}, []string{"Preset", "Crossfade"}},
		{palette, []string{"Command palette"}, []string{"Like what plays"}},
		{"normalize", []string{"Volume levelling"}, []string{"Crossfade"}},
		{"startup", []string{"Start at login"}, []string{"Close to the tray"}},
		{"version", []string{"Version", "This is the newest version"}, []string{"Crossfade"}},
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

// paneTexts are the texts the settings at path show but those the home
// page shows too: the rail's, the top bar's, the queue's and the player
// bar's.
func paneTexts(t *testing.T, s *shell, path string) []string {
	t.Helper()
	tt := testerOf(t, s, 1280, 9000)
	show(s, tt, "/")
	base := tt.Texts()
	show(s, tt, path)
	var out []string
	for _, x := range tt.Texts() {
		if i := slices.Index(base, x); i >= 0 {
			base = slices.Delete(base, i, i+1)
			continue
		}
		out = append(out, x)
	}
	return out
}

// TestSettingsFindNothingElse searches the settings for what nothing holds,
// in every state of the accounts, the tools and the add-ons that shows
// something more: nothing shows but the empty line, as every block of the
// sections goes through the search. A block that doesn't shows here.
func TestSettingsFindNothingElse(t *testing.T) {
	for _, c := range []struct {
		name string
		set  func(s *shell)
	}{
		{"signed out", func(s *shell) {}},
		{"refused", func(s *shell) {
			s.loads["accounts"] = &loaded[app.AccountsStatus]{done: true, value: app.AccountsStatus{
				Spotify: app.SpotifyAccount{SignedIn: true, Error: "refused"},
				YouTube: app.YouTubeAccount{SignedIn: true, Error: "expired"},
				Lastfm:  app.LastfmAccount{Connected: true, Expired: true, Name: "someone"},
			}}
		}},
		{"signed in", func(s *shell) {
			s.loads["accounts"] = &loaded[app.AccountsStatus]{done: true, value: app.AccountsStatus{
				Spotify: app.SpotifyAccount{SignedIn: true},
				YouTube: app.YouTubeAccount{SignedIn: true},
				Lastfm:  app.LastfmAccount{Connected: true, Name: "someone"},
			}}
			s.loads["channels"] = &loaded[[]youtube.AccountChannel]{done: true, value: []youtube.AccountChannel{
				{Name: "Me", Current: true}, {Name: "My band", PageID: "b"},
			}}
			s.set.SpotifyPlayback = settings.SpotifyPlaybackHere
		}},
		{"on spotify connect", func(s *shell) {
			s.set.SpotifyPlayback, s.set.FollowSystem, s.set.DiscordPresence = settings.SpotifyPlaybackConnect, true, true
			s.loads["devices"] = &loaded[[]spotify.Device]{done: true}
			s.licenseOpen = 0
		}},
		{"looking for the tools", func(s *shell) { s.loads["tools"] = &loaded[tools.Status]{} }},
		{"add-ons that didn't load", func(s *shell) {
			for _, kind := range []string{"themes", "plugins"} {
				if err := os.MkdirAll(filepath.Join(s.svc.System.Info().DataDir, "addons", kind, "broken"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			s.svc.Addons.Reload()
			s.refresh()
			if len(s.inv.Problems) != 2 {
				t.Fatalf("the add-ons' problems are %+v", s.inv.Problems)
			}
		}},
	} {
		s := newTestShell(t)
		c.set(s)
		for _, x := range paneTexts(t, s, settingsPath("", "zzzz")) {
			switch {
			case x == "settings", x == "Search the settings", x == "zzzz", x == "No setting matches “zzzz”.", slices.Contains(settingsTabs, x):
			default:
				t.Errorf("%s, a search that found nothing shows %q", c.name, x)
			}
		}
	}
}

// TestSettingsFindForms searches for a sign-in's button, which shows with
// the fields it needs.
func TestSettingsFindForms(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 6000)
	show(s, tt, settingsPath("", "sign in"))
	showsOnly(t, tt, "searching sign in", []string{"Client ID", "sign in to spotify", "sign in to youtube music"}, nil)
	show(s, tt, settingsPath("", "connect"))
	showsOnly(t, tt, "searching connect", []string{"API key", "Shared secret", "connect to last.fm"}, nil)

	// The button reads the Client ID's draft, which its row made.
	s.drafts["client-id"] = &draft{text: "0123456789abcdef0123456789abcdef"}
	show(s, tt, settingsPath("", "redirect uri"))
	if !tt.HasText("Client ID") || !tt.HasText("sign in to spotify") {
		t.Errorf("searching redirect uri shows %q", tt.Texts())
	}
}

// TestSettingsFindSpellings finds the settings spelled either way, at every
// letter typed.
func TestSettingsFindSpellings(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 6000)
	for _, c := range []struct{ word, finds string }{
		{"colour", "Accent colour"}, {"color", "Accent colour"},
		{"equaliser", "Shape the sound of YouTube Music with ten bands and a preamp."},
		{"equalizer", "Shape the sound of YouTube Music with ten bands and a preamp."},
		{"leveling", "Volume levelling"}, {"levelling", "Volume levelling"},
		{"licence", "libopus, which decodes Opus audio"},
		{"normalise", "Volume levelling"},
	} {
		for n := 4; n <= len(c.word); n++ {
			q := c.word[:n]
			show(s, tt, settingsPath("", q))
			if !tt.HasText(c.finds) {
				t.Errorf("%q doesn't find %q: %q", q, c.finds, tt.Texts())
			}
		}
	}
}

func TestSearchFold(t *testing.T) {
	for in, want := range map[string]string{
		"Accent colour":    "accent colour accent color",
		"Équalizer":        "equalizer equaliser",
		"Volume levelling": "volume levelling volume leveling",
		"Crossfade":        "crossfade",
	} {
		if got := searchFold(in); got != want {
			t.Errorf("searchFold(%q) = %q, not %q", in, got, want)
		}
	}
}
