package native

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/settings"
)

// TestSettingsViewed shows every section of the settings, in both of the
// ways the themes go, and in each way Spotify plays: showing them changes
// nothing.
func TestSettingsViewed(t *testing.T) {
	s := newTestShell(t)
	tt := tester(t, s)
	for _, follow := range []bool{false, true} {
		for _, mode := range []settings.SpotifyPlayback{settings.SpotifyPlaybackHere, settings.SpotifyPlaybackConnect, settings.SpotifyPlaybackMatch} {
			st := s.svc.Preferences.Get()
			st.FollowSystem, st.SpotifyPlayback, st.Accent = follow, mode, "#123456"
			if _, err := s.svc.Preferences.Set(st); err != nil {
				t.Fatal(err)
			}
			s.refresh()
			before, _ := json.Marshal(s.svc.Preferences.Get())
			for _, tab := range settingsTabs {
				s.navigate("/settings/" + tab)
				s.drain()
				tt.Frame()
				tt.Advance(time.Second)
			}
			if after, _ := json.Marshal(s.svc.Preferences.Get()); string(after) != string(before) {
				t.Errorf("showing the settings changed them, following the computer %v, Spotify %s:\n%s\n%s", follow, mode, before, after)
			}
		}
	}
}

// TestThemeOptionsViewed shows the options of each built-in theme: showing
// them changes nothing.
func TestThemeOptionsViewed(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 1800)
	for _, theme := range []string{"mix", "aurora", "sakura", "terminal"} {
		st := s.svc.Preferences.Get()
		st.FollowSystem, st.Theme = false, theme
		if _, err := s.svc.Preferences.Set(st); err != nil {
			t.Fatal(err)
		}
		s.refresh()
		before, _ := json.Marshal(s.svc.Preferences.Get())
		show(s, tt, "/settings/appearance")
		if !tt.HasText(theme + " options") {
			t.Errorf("the settings don't show the options of %s", theme)
		}
		if after, _ := json.Marshal(s.svc.Preferences.Get()); string(after) != string(before) {
			t.Errorf("showing the options of %s changed the settings:\n%s\n%s", theme, before, after)
		}
		show(s, tt, "/")
	}
}

func TestAccentLink(t *testing.T) {
	s := newTestShell(t)
	st := s.svc.Preferences.Get()
	st.Accent = "#ff0000"
	if _, err := s.svc.Preferences.Set(st); err != nil {
		t.Fatal(err)
	}
	s.refresh()
	// The link is under the fold of a window of the usual size, past the
	// themes' cards.
	tt := testerOf(t, s, 1280, 2200)
	show(s, tt, "/settings/appearance")
	click(t, s, tt, "use the theme's")
	if got := s.svc.Preferences.Get().Accent; got != "" {
		t.Errorf("the theme's accent left the accent %q", got)
	}
}

// TestColoursKept closes the window, or quits, before a colour well saves
// the accent it changed: the settings keep it at once.
func TestColoursKept(t *testing.T) {
	s := newTestShell(t)
	s.wells["accent"] = &colorWell{edits: 1}
	s.set.Accent = "#123456"
	s.keepColors()
	if got := s.svc.Preferences.Get().Accent; got != "#123456" {
		t.Fatalf("the settings keep the accent %q", got)
	}
	if s.wells["accent"].edits != 0 {
		t.Error("the well still waits to save")
	}
	// With nothing waiting, nothing is saved.
	st := s.svc.Preferences.Get()
	st.Accent = "#abcdef"
	if _, err := s.svc.Preferences.Set(st); err != nil {
		t.Fatal(err)
	}
	s.keepColors()
	if got := s.svc.Preferences.Get().Accent; got != "#abcdef" {
		t.Errorf("keeping with nothing waiting saved the accent %q", got)
	}
}

// TestPluginSettings enables a plugin, which asks first, disables it, and
// removes it, which asks too.
func TestPluginSettings(t *testing.T) {
	s := newTestShell(t)
	dir := filepath.Join(s.svc.System.Info().DataDir, "addons", "plugins", "hello")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string]string{
		"plugin.json": `{"id": "hello", "name": "Hello", "version": "1.0.0", "author": "A", "main": "index.js", "permissions": ["playback"]}`,
		"index.js":    "export default () => {}",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s.svc.Addons.Reload()
	s.refresh()
	enabled := func() bool { return slices.Contains(s.svc.Preferences.Get().EnabledPlugins, "hello") }
	tt := tester(t, s)
	show(s, tt, "/settings/plugins")

	click(t, s, tt, "Enable Hello")
	if enabled() {
		t.Fatal("the plugin was enabled without asking")
	}
	if !tt.HasText("It asks for: playback.") {
		t.Errorf("the question doesn't tell what the plugin asks for: %q", tt.Texts())
	}
	click(t, s, tt, "Enable")
	if !enabled() {
		t.Fatal("the plugin wasn't enabled")
	}
	click(t, s, tt, "Enable Hello")
	if enabled() {
		t.Fatal("the plugin wasn't disabled")
	}

	click(t, s, tt, "Remove Hello")
	if len(s.svc.Addons.List().Plugins) != 1 {
		t.Fatal("the plugin was removed without asking")
	}
	click(t, s, tt, "Remove")
	if n := len(s.svc.Addons.List().Plugins); n != 0 {
		t.Errorf("%d plugins are left", n)
	}
}

// TestCacheOff turns the offline cache off, which its sizes start with.
func TestCacheOff(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 3200)
	show(s, tt, "/settings/playback")
	if !tt.HasText("1 GB at most.") {
		t.Fatalf("the cache's size isn't shown: %q", tt.Texts())
	}
	click(t, s, tt, "Cache size")
	tt.Key(0, ui.KeyHome)
	tt.Key(0, ui.KeyEnter)
	s.drain()
	tt.Frame()
	if got := s.svc.Preferences.Get().CacheSizeMB; got != 0 {
		t.Errorf("the cache keeps %d MB, not none", got)
	}
	if !tt.HasText("Off: MixMusic keeps no tracks.") {
		t.Errorf("the cache doesn't say it's off: %q", tt.Texts())
	}
}

// TestTitleSwitch turns what plays in the window's title off and on again
// from Settings › Desktop (#105). Its row's title carries the switch's
// words too, so the keyboard reaches the switch: focused, it has them, and
// the row's description doesn't.
func TestTitleSwitch(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 1800)
	show(s, tt, "/settings/desktop")
	const label = "Show what plays in the window title"
	const description = "The window's title names the track that plays, for the taskbar, the window switcher and screen readers."
	focused := func() bool { return tt.Focused(label) && !tt.Focused(description) }
	for i := 0; i < 100 && !focused(); i++ {
		tt.Key(0, ui.KeyTab)
	}
	if !focused() {
		t.Fatalf("Tab doesn't reach the switch: %q", tt.Texts())
	}
	for _, want := range []bool{false, true} {
		tt.Key(0, ui.KeySpace)
		s.drain()
		tt.Frame()
		if got := s.svc.Preferences.Get().ShowInTitle; got != want {
			t.Errorf("the switch left the title showing what plays %v, not %v", got, want)
		}
	}
}

func TestCacheSizeLabel(t *testing.T) {
	for mb, want := range map[int]string{0: "Off", 512: "512 MB", 1024: "1 GB", 16384: "16 GB", 1536: "1536 MB"} {
		if got := cacheSizeLabel(mb); got != want {
			t.Errorf("cacheSizeLabel(%d) = %q, not %q", mb, got, want)
		}
	}
}

// TestThemeCards says who made each theme: MixMusic, or its author.
func TestThemeCards(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 1600)
	show(s, tt, "/settings/appearance")
	if !tt.HasText("built in") {
		t.Errorf("the built-in themes don't say so: %q", tt.Texts())
	}
}

// TestAppearance gives the title bar and the system's menus the light or
// dark of the theme worn, and leaves them to the computer while the theme
// follows it.
func TestAppearance(t *testing.T) {
	var got []mygo.ThemeSource
	setAppearance = func(source mygo.ThemeSource) { got = append(got, source) }
	defer func() { setAppearance = func(mygo.ThemeSource) {} }()
	s := newTestShell(t)
	for _, c := range []struct {
		follow bool
		theme  string
		want   mygo.ThemeSource
	}{
		{false, "daylight", mygo.ThemeLight},
		{false, "mix", mygo.ThemeDark},
		{true, "daylight", mygo.ThemeSystem},
	} {
		got = nil
		s.set.FollowSystem, s.set.Theme = c.follow, c.theme
		s.restyle()
		if len(got) != 1 || got[0] != c.want {
			t.Errorf("wearing %s, following the computer %v, the appearance went %v, want %s", c.theme, c.follow, got, c.want)
		}
	}
}

// TestEvenLoudness turns Even loudness off, and tells how it differs from
// Volume levelling, and that Spotify's own tracks keep their loudness.
func TestEvenLoudness(t *testing.T) {
	const line = "Turns loud tracks down to one level, from YouTube's measure of each, so none jumps out. Quiet recordings keep their level."
	const spotify = " Spotify plays its own tracks at its own loudness."
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 3200)
	for _, c := range []struct {
		mode settings.SpotifyPlayback
		says string
	}{
		{settings.SpotifyPlaybackMatch, line},
		{settings.SpotifyPlaybackConnect, line + spotify},
	} {
		mode, says := c.mode, c.says
		st := s.svc.Preferences.Get()
		st.SpotifyPlayback = mode
		if _, err := s.svc.Preferences.Set(st); err != nil {
			t.Fatal(err)
		}
		s.refresh()
		show(s, tt, "/settings/playback")
		if !tt.HasText(says) || says == line && tt.HasText(spotify[1:]) {
			t.Errorf("with Spotify %s, the row doesn't say %q: %q", mode, says, tt.Texts())
		}
	}
	if !tt.HasText("Squeezes loud and quiet moments closer together as they play, and lifts quiet recordings.") {
		t.Error("Volume levelling doesn't say how it differs")
	}
	if !s.svc.Preferences.Get().Loudness {
		t.Fatal("Even loudness is off at first")
	}
	// The row's title shows "Even loudness" too: the switch is to its
	// right, beside the title or halfway down the row.
	title, _ := tt.Find("Even loudness")
	desc, _ := tt.Find(line + spotify)
	for x := title.X + title.W; x < 1280 && s.svc.Preferences.Get().Loudness; x += 6 {
		for _, y := range []float32{title.Y + title.H/2, (title.Y + desc.Y + desc.H) / 2} {
			if s.svc.Preferences.Get().Loudness {
				tt.ClickAt(x, y)
				s.drain()
				tt.Frame()
			}
		}
	}
	if s.svc.Preferences.Get().Loudness {
		t.Error("the switch didn't turn Even loudness off")
	}
}
