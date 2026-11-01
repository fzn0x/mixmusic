package settings

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func TestDefaultIsClean(t *testing.T) {
	d := Default()
	a, _ := json.Marshal(d)
	b, _ := json.Marshal(d.Clean())
	if string(a) != string(b) {
		t.Errorf("Clean changed the defaults:\n%s\n%s", a, b)
	}
}

func TestCleanClampsAndRepairs(t *testing.T) {
	s := Settings{
		Theme:           "../../etc",
		Accent:          "red; background: url(x)",
		FontScale:       5,
		SpotifyPlayback: "nonsense",
		Crossfade:       60,
		Volume:          -1,
		Equalizer:       Equalizer{Gains: []float64{99, -99}},
		JSRuntime:       "python",
		EnabledPlugins:  []string{"sleep-timer", "sleep-timer", "Bad ID", "../x"},
		RegistryURL:     "http://insecure.example",
		YouTubeChannel:  "1234; x",
	}.Clean()
	if s.Theme != "mix" || s.Accent != "" || s.FontScale != 1 || s.SpotifyPlayback != Default().SpotifyPlayback {
		t.Errorf("appearance and playback not repaired: %+v", s)
	}
	if s.Crossfade != 12 || s.Volume != 0 || len(s.Equalizer.Gains) != len(Bands) {
		t.Errorf("ranges not clamped: %+v", s)
	}
	if s.JSRuntime != "" || len(s.EnabledPlugins) != 1 || s.RegistryURL != DefaultRegistryURL {
		t.Errorf("tools and add-ons not repaired: %+v", s)
	}
	if s.YouTubeChannel != "" {
		t.Errorf("a channel that is not a page ID was kept: %q", s.YouTubeChannel)
	}
	if c := (Settings{YouTubeChannel: " 112233445566778899 "}).Clean(); c.YouTubeChannel != "112233445566778899" {
		t.Errorf("a page ID was not kept: %q", c.YouTubeChannel)
	}
	if !Default().YouTubeHistory {
		t.Error("plays do not reach the YouTube history by default")
	}
}

func TestValidID(t *testing.T) {
	for id, want := range map[string]bool{"mix": true, "sleep-timer": true, "a1": true, "": false, "-x": false, "Mix": false, "a/b": false, "a.b": false} {
		if got := ValidID(id); got != want {
			t.Errorf("ValidID(%q) = %v", id, got)
		}
	}
}

func TestActiveTheme(t *testing.T) {
	s := Default()
	if got := s.ActiveTheme(true); got != "mix" {
		t.Errorf("not following the system, dark: %q", got)
	}
	s.Theme = "sakura"
	s.FollowSystem = true
	if got := s.ActiveTheme(true); got != "mix" {
		t.Errorf("dark appearance: %q", got)
	}
	if got := s.ActiveTheme(false); got != "daylight" {
		t.Errorf("light appearance: %q", got)
	}
	s.LightTheme, s.DarkTheme = "Not An ID", ""
	if c := s.Clean(); c.LightTheme != "daylight" || c.DarkTheme != "mix" {
		t.Errorf("cleaned to %q and %q", c.LightTheme, c.DarkTheme)
	}
}

func TestShortcutsClean(t *testing.T) {
	s := Default()
	s.Shortcuts = map[string]string{"player.toggle": "Mod+P", "player.mute": "", "": "K", "x": strings.Repeat("A", 65)}
	c := s.Clean()
	if c.Shortcuts["player.toggle"] != "Mod+P" {
		t.Errorf("chosen keys dropped: %v", c.Shortcuts)
	}
	if v, ok := c.Shortcuts["player.mute"]; !ok || v != "" {
		t.Errorf("a removed shortcut must stay removed: %v", c.Shortcuts)
	}
	if _, ok := c.Shortcuts[""]; ok || len(c.Shortcuts) != 2 {
		t.Errorf("invalid entries kept: %v", c.Shortcuts)
	}
	if (Settings{}).Clean().Shortcuts == nil {
		t.Error("Clean must give an empty map, not nil")
	}
}

func TestDefaultSpotifyPlayback(t *testing.T) {
	for goos, want := range map[string]SpotifyPlayback{
		"windows": SpotifyPlaybackHere,
		"darwin":  SpotifyPlaybackConnect,
		"linux":   SpotifyPlaybackConnect,
	} {
		if got := defaultSpotifyPlayback(goos); got != want {
			t.Errorf("defaultSpotifyPlayback(%q) = %q, want %q", goos, got, want)
		}
	}
	if c := (Settings{SpotifyPlayback: SpotifyPlaybackHere}).Clean(); c.SpotifyPlayback != SpotifyPlaybackHere {
		t.Errorf("Clean dropped playback here: %q", c.SpotifyPlayback)
	}
}

// TestCleanCopies changes what Clean returned: the settings it cleaned
// stay as they were.
func TestCleanCopies(t *testing.T) {
	s := Default()
	s.ThemeOptions = map[string]map[string]string{"mix": {"texture": "dots"}}
	s.Shortcuts = map[string]string{"player.toggle": "Space"}
	s.EnabledPlugins = []string{"spectrum"}
	c := s.Clean()
	c.Equalizer.Gains[0] = 6
	c.ThemeOptions["mix"]["texture"] = "none"
	c.Shortcuts["player.toggle"] = "K"
	c.EnabledPlugins[0] = "other"
	if s.Equalizer.Gains[0] != 0 || s.ThemeOptions["mix"]["texture"] != "dots" || s.Shortcuts["player.toggle"] != "Space" || s.EnabledPlugins[0] != "spectrum" {
		t.Errorf("changing the cleaned settings changed the settings: %v %v %v %v", s.Equalizer.Gains, s.ThemeOptions, s.Shortcuts, s.EnabledPlugins)
	}
}

// TestRegistryURL moves settings off the web interface's registry, which
// earlier versions kept as theirs.
func TestRegistryURL(t *testing.T) {
	if got := (Settings{RegistryURL: WebRegistryURL}).Clean().RegistryURL; got != DefaultRegistryURL {
		t.Errorf("the web interface's registry became %q", got)
	}
	const own = "https://example.com/my-registry.json"
	if got := (Settings{RegistryURL: own}).Clean().RegistryURL; got != own {
		t.Errorf("a registry of the user's became %q", got)
	}
}

// TestDiscordPresence shows what plays on Discord in new installs and in
// the settings of earlier versions, where it was off until the user set it
// up, unless they set it up and turned it off.
func TestDiscordPresence(t *testing.T) {
	if d := Default(); !d.DiscordPresence || d.DiscordApp() != DefaultDiscordAppID {
		t.Errorf("a new install shows on Discord %v, as %q", d.DiscordPresence, d.DiscordApp())
	}
	const own = "123456789012345678"
	for _, c := range []struct {
		file string
		want bool
	}{
		{`{}`, true},
		{`{"discordPresence":false}`, true},
		{`{"discordPresence":false,"discordAppId":"  "}`, true},
		{`{"discordPresence":true,"discordAppId":"` + own + `"}`, true},
		{`{"discordPresence":false,"discordAppId":"` + own + `"}`, false},
		{`{"showOnDiscord":false}`, false},
		{`{"showOnDiscord":true,"discordPresence":false,"discordAppId":"` + own + `"}`, true},
	} {
		s := Default()
		if err := json.Unmarshal([]byte(c.file), &s); err != nil {
			t.Fatal(err)
		}
		if s.DiscordPresence != c.want {
			t.Errorf("%s shows on Discord %v, not %v", c.file, s.DiscordPresence, c.want)
		}
	}
	if got := (Settings{DiscordAppID: own}).DiscordApp(); got != own {
		t.Errorf("the user's application became %q", got)
	}
}

// TestShowInTitle names what plays in the window's title in new installs
// and in the settings of earlier versions, which don't have the key.
func TestShowInTitle(t *testing.T) {
	for file, want := range map[string]bool{`{}`: true, `{"theme":"mix"}`: true, `{"showInTitle":false}`: false, `{"showInTitle":true}`: true} {
		s := Default()
		if err := json.Unmarshal([]byte(file), &s); err != nil {
			t.Fatal(err)
		}
		if s.Clean().ShowInTitle != want {
			t.Errorf("%s shows what plays in the title %v, not %v", file, s.ShowInTitle, want)
		}
	}
}

// TestCleanPanel shows the queue for settings from before the panel, and
// keeps the user's choice.
func TestCleanPanel(t *testing.T) {
	for in, want := range map[string]string{"": "queue", "bogus": "queue", "queue": "queue", "lyrics": "lyrics", "none": "none"} {
		if got := (Settings{Panel: in}).Clean().Panel; got != want {
			t.Errorf("Clean of the panel %q gives %q, not %q", in, got, want)
		}
	}
}

// TestLoudness has Even loudness on in new installs, and in the settings of
// earlier versions, which don't have it, until the user turns it off.
func TestLoudness(t *testing.T) {
	for file, want := range map[string]bool{
		`{}`:                                  true,
		`{"normalize":true}`:                  true,
		`{"loudness":false}`:                  false,
		`{"loudness":false,"normalize":true}`: false,
	} {
		s := Default()
		if err := json.Unmarshal([]byte(file), &s); err != nil {
			t.Fatal(err)
		}
		if s = s.Clean(); s.Loudness != want {
			t.Errorf("%s: Even loudness %v, not %v", file, s.Loudness, want)
		}
	}
}

// TestZoom keeps a zoom of the steps, takes the nearest step for another
// in range, and 100% for one outside them, as settings from before it.
func TestZoom(t *testing.T) {
	for _, c := range []struct{ in, want float64 }{
		{1.25, 1.25}, {2, 2}, {0.8, 0.8}, {1.3, 1.25}, {1.2, 1.25}, {0.85, 0.8},
		{0, 1}, {0.5, 1}, {3, 1}, {math.NaN(), 1},
	} {
		s := Default()
		s.Zoom = c.in
		if got := s.Clean().Zoom; got != c.want {
			t.Errorf("a zoom of %v reads as %v, want %v", c.in, got, c.want)
		}
	}
}

// TestColorSource starts with the theme's colours, drops a source it
// doesn't know and a path too long to be one, and keeps a pasted path
// without its spaces and quotes.
func TestColorSource(t *testing.T) {
	if d := Default(); d.ColorSource != ColorsTheme || d.ColorFile != "" {
		t.Errorf("a new install reads a colour file: %q %q", d.ColorSource, d.ColorFile)
	}
	for _, source := range []ColorSource{ColorsTheme, ColorsPywal, ColorsMatugen, ColorsFile} {
		if got := (Settings{ColorSource: source}).Clean().ColorSource; got != source {
			t.Errorf("Clean made %q %q", source, got)
		}
	}
	if got := (Settings{ColorSource: "wallust"}).Clean().ColorSource; got != ColorsTheme {
		t.Errorf("an unknown source stayed: %q", got)
	}
	for in, want := range map[string]string{
		"  ~/.cache/wal/colors.json ":    "~/.cache/wal/colors.json",
		`"C:\Users\me\colors.json"`:      `C:\Users\me\colors.json`,
		` " /home/me/my colours.json " `: "/home/me/my colours.json",
		`"`:                              `"`,
		"/" + strings.Repeat("a", 5000):  "",
		"/" + strings.Repeat("a", 4095):  "/" + strings.Repeat("a", 4095),
		`/home/me/"quoted" name.json`:    `/home/me/"quoted" name.json`,
		"":                               "",
	} {
		if got := (Settings{ColorFile: in}).Clean().ColorFile; got != want {
			t.Errorf("Clean made the colour file %q %q, want %q", in, got, want)
		}
	}
}
