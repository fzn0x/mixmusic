// Package settings defines the user's preferences and their defaults.
package settings

import (
	"cmp"
	"encoding/json"
	"maps"
	"math"
	"regexp"
	"runtime"
	"slices"
	"strings"
)

// SpotifyPlayback chooses how Spotify tracks play.
type SpotifyPlayback string

const (
	// SpotifyPlaybackMatch plays Spotify tracks through MixMusic's own
	// player, from their recording on YouTube Music.
	SpotifyPlaybackMatch SpotifyPlayback = "match"
	// SpotifyPlaybackConnect plays Spotify tracks on a Spotify Connect
	// device, which needs Spotify Premium.
	SpotifyPlaybackConnect SpotifyPlayback = "connect"
	// SpotifyPlaybackHere plays Spotify tracks in MixMusic, which Spotify's
	// Web Playback SDK makes a Spotify Connect device of its own. It needs
	// Spotify Premium, and no Spotify app.
	SpotifyPlaybackHere SpotifyPlayback = "here"
)

// NativeAudio reports whether MixMusic's audio engine plays on this
// system: 64-bit Windows, macOS and Linux.
const NativeAudio = (runtime.GOOS == "windows" || runtime.GOOS == "darwin" || runtime.GOOS == "linux") &&
	(runtime.GOARCH == "amd64" || runtime.GOARCH == "arm64")

// Density chooses how much room the interface gives each element.
type Density string

const (
	DensityComfortable Density = "comfortable"
	DensityCompact     Density = "compact"
)

// Quality chooses between the best audio and smaller streams.
type Quality string

const (
	QualityBest  Quality = "best"
	QualitySaver Quality = "saver"
)

// Bands are the center frequencies of the equalizer's bands, in hertz.
var Bands = []int{32, 64, 125, 250, 500, 1000, 2000, 4000, 8000, 16000}

// Equalizer shapes the sound of MixMusic's own player.
type Equalizer struct {
	Enabled bool   `json:"enabled"`
	Preset  string `json:"preset"`
	// Gains are in decibels, one per band, from -12 to 12.
	Gains []float64 `json:"gains"`
	// Preamp is in decibels, from -12 to 12.
	Preamp float64 `json:"preamp"`
}

// Settings are the user's preferences.
type Settings struct {
	// Appearance.
	Theme string `json:"theme"`
	// FollowSystem shows LightTheme while the computer is in light mode
	// and DarkTheme while it is in dark mode, in place of Theme.
	FollowSystem bool   `json:"followSystem"`
	LightTheme   string `json:"lightTheme"`
	DarkTheme    string `json:"darkTheme"`
	// ThemeOptions holds the values of each theme's options, by theme ID
	// and option ID.
	ThemeOptions map[string]map[string]string `json:"themeOptions"`
	// Accent replaces the theme's accent color, unless empty.
	Accent string `json:"accent"`
	// DynamicAccent takes the accent color from the artwork playing.
	DynamicAccent bool    `json:"dynamicAccent"`
	Density       Density `json:"density"`
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
	// LyricsPlain shows synchronized lyrics as plain text.
	LyricsPlain bool `json:"lyricsPlain"`
	// SkippedYTDLP is the release of yt-dlp the home page told of, which
	// the user put off: it doesn't tell of it again.
	SkippedYTDLP string `json:"skippedYtdlp"`

	// Playback.
	SpotifyPlayback SpotifyPlayback `json:"spotifyPlayback"`
	// SpotifyDevice is the Spotify Connect device to play on; the active
	// one when empty.
	SpotifyDevice string  `json:"spotifyDevice"`
	Quality       Quality `json:"quality"`
	// Crossfade overlaps tracks by this many seconds, from 0 to 12.
	Crossfade int `json:"crossfade"`
	// Normalize runs Volume levelling, a compressor that squeezes loud and
	// quiet moments closer together as they play.
	Normalize bool `json:"normalize"`
	// Loudness turns tracks louder than -14 LKFS down to it, from YouTube's
	// measure of each: Even loudness.
	Loudness bool `json:"loudness"`
	// Autoplay continues with similar tracks when the queue ends.
	Autoplay bool `json:"autoplay"`
	// SkipDisliked passes over the tracks the user disliked as the queue
	// moves on, and leaves them out of radios.
	SkipDisliked bool      `json:"skipDisliked"`
	Equalizer    Equalizer `json:"equalizer"`
	// Volume is from 0 to 1.
	Volume float64 `json:"volume"`
	// CacheSizeMB bounds the audio cache; 0 turns it off.
	CacheSizeMB int `json:"cacheSizeMb"`

	// Accounts.
	SpotifyClientID string `json:"spotifyClientId"`

	// Scrobbling sends what plays to Last.fm, once the user connected it.
	Scrobbling bool `json:"scrobbling"`
	// YouTubeChannel is the brand channel the user uses YouTube Music as,
	// by its page ID; "" is the account itself.
	YouTubeChannel string `json:"youtubeChannel"`
	// YouTubeHistory adds what the user plays to their history on YouTube
	// Music, once they signed in, for its recommendations.
	YouTubeHistory bool `json:"youtubeHistory"`

	// The desktop.
	CloseToTray   bool `json:"closeToTray"`
	Notifications bool `json:"notifications"`
	// ShowInTitle names what plays in the main window's title.
	ShowInTitle bool `json:"showInTitle"`
	MediaKeys   bool `json:"mediaKeys"`
	// DiscordPresence shows what plays on the user's Discord profile, as
	// the Discord application of DiscordAppID, or MixMusic's own when it
	// is empty.
	DiscordPresence bool   `json:"showOnDiscord"`
	DiscordAppID    string `json:"discordAppId"`

	// Tools.
	YTDLPPath string `json:"ytdlpPath"`
	JSRuntime string `json:"jsRuntime"`

	// Shortcuts replaces the keys of commands, by command ID; an empty
	// value takes a command's shortcut away.
	Shortcuts map[string]string `json:"shortcuts"`

	// Add-ons.
	EnabledPlugins []string `json:"enabledPlugins"`
	// RegistryURL is the address of the community registry of themes and
	// plugins.
	RegistryURL string `json:"registryUrl"`
}

// DefaultRegistryURL is the community registry in MixMusic's repository.
const DefaultRegistryURL = "https://raw.githubusercontent.com/fzn0x/mixmusic/main/community/registry-2.json"

// WebRegistryURL is the registry of the add-ons of the web interface,
// which the versions of MixMusic before its own interface read, and kept
// in their settings.
const WebRegistryURL = "https://raw.githubusercontent.com/fzn0x/mixmusic/main/community/registry.json"

// DefaultDiscordAppID is MixMusic's own Discord application, whose name
// Discord shows beside what plays when the user set no other.
const DefaultDiscordAppID = "1562074343691788378"

// Default returns the settings of a new install.
func Default() Settings {
	return Settings{
		Theme:           "mix",
		LightTheme:      "daylight",
		DarkTheme:       "mix",
		ThemeOptions:    map[string]map[string]string{},
		DynamicAccent:   true,
		Density:         DensityComfortable,
		Panel:           "queue",
		FontScale:       1,
		Zoom:            1,
		SpotifyPlayback: defaultSpotifyPlayback(runtime.GOOS),
		Quality:         QualityBest,
		Loudness:        true,
		Autoplay:        true,
		SkipDisliked:    true,
		Equalizer:       Equalizer{Preset: "flat", Gains: make([]float64, len(Bands))},
		Volume:          0.8,
		CacheSizeMB:     1024,
		Scrobbling:      true,
		YouTubeHistory:  true,
		CloseToTray:     false,
		Notifications:   true,
		ShowInTitle:     true,
		MediaKeys:       true,
		DiscordPresence: true,
		Shortcuts:       map[string]string{},
		EnabledPlugins:  []string{},
		RegistryURL:     DefaultRegistryURL,
	}
}

// defaultSpotifyPlayback is how Spotify tracks play in a new install on
// goos: in MixMusic where its webview, WebView2, decrypts Spotify's audio,
// else on Spotify Connect.
func defaultSpotifyPlayback(goos string) SpotifyPlayback {
	if goos == "windows" {
		return SpotifyPlaybackHere
	}
	return SpotifyPlaybackConnect
}

var (
	colorRe = regexp.MustCompile(`^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)
	idRe    = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
	// discordIDRe matches a Discord application ID, a number.
	discordIDRe = regexp.MustCompile(`^[0-9]{15,22}$`)
	// pageIDRe matches the page ID of a YouTube brand channel, a number.
	pageIDRe = regexp.MustCompile(`^[0-9]{1,32}$`)
)

// UnmarshalJSON reads settings, those of the versions before 0.5.6 too,
// where Discord showed nothing until the user turned it on and pasted the
// ID of an application of theirs. Those settings show what plays now,
// unless the user had pasted an ID and turned it off.
func (s *Settings) UnmarshalJSON(data []byte) error {
	type plain Settings
	if err := json.Unmarshal(data, (*plain)(s)); err != nil {
		return err
	}
	var keys struct {
		Before *bool `json:"discordPresence"`
		Now    *bool `json:"showOnDiscord"`
	}
	if json.Unmarshal(data, &keys) == nil && keys.Now == nil && keys.Before != nil {
		s.DiscordPresence = *keys.Before || strings.TrimSpace(s.DiscordAppID) == ""
	}
	return nil
}

// DiscordApp returns the ID of the Discord application that shows what
// plays: the user's, or MixMusic's own.
func (s Settings) DiscordApp() string { return cmp.Or(s.DiscordAppID, DefaultDiscordAppID) }

// ValidID reports whether id can name a theme or a plugin: lowercase
// letters, digits and dashes.
func ValidID(id string) bool { return idRe.MatchString(id) }

// ActiveTheme returns the ID of the theme to show: Theme, or the light or
// dark one of the computer's appearance when the theme follows it.
func (s Settings) ActiveTheme(dark bool) string {
	switch {
	case !s.FollowSystem:
		return s.Theme
	case dark:
		return s.DarkTheme
	default:
		return s.LightTheme
	}
}

// Clean returns s with every value in its range, and the defaults in place
// of values that are missing or invalid. Its maps and slices are its own,
// which changing doesn't change s's.
func (s Settings) Clean() Settings {
	d := Default()
	if !ValidID(s.Theme) {
		s.Theme = d.Theme
	}
	if !ValidID(s.LightTheme) {
		s.LightTheme = d.LightTheme
	}
	if !ValidID(s.DarkTheme) {
		s.DarkTheme = d.DarkTheme
	}
	options := make(map[string]map[string]string, len(s.ThemeOptions))
	for id, values := range s.ThemeOptions {
		options[id] = maps.Clone(values)
	}
	s.ThemeOptions = options
	if s.Accent != "" && !colorRe.MatchString(s.Accent) {
		s.Accent = ""
	}
	if s.Panel != "lyrics" && s.Panel != "none" {
		s.Panel = d.Panel
	}
	if s.Density != DensityCompact {
		s.Density = DensityComfortable
	}
	if s.FontScale < 0.85 || s.FontScale > 1.3 {
		s.FontScale = d.FontScale
	}
	s.Zoom = zoomStep(s.Zoom, d.Zoom)
	switch s.SpotifyPlayback {
	case SpotifyPlaybackMatch, SpotifyPlaybackConnect, SpotifyPlaybackHere:
	default:
		s.SpotifyPlayback = d.SpotifyPlayback
	}
	if s.Quality != QualitySaver {
		s.Quality = QualityBest
	}
	s.Crossfade = min(max(s.Crossfade, 0), 12)
	s.Volume = min(max(s.Volume, 0), 1)
	if len(s.Equalizer.Gains) != len(Bands) {
		s.Equalizer.Gains = make([]float64, len(Bands))
	} else {
		s.Equalizer.Gains = slices.Clone(s.Equalizer.Gains)
	}
	for i, g := range s.Equalizer.Gains {
		s.Equalizer.Gains[i] = min(max(g, -12), 12)
	}
	s.Equalizer.Preamp = min(max(s.Equalizer.Preamp, -12), 12)
	if s.Equalizer.Preset == "" {
		s.Equalizer.Preset = "custom"
	}
	s.CacheSizeMB = min(max(s.CacheSizeMB, 0), 64*1024)
	s.SpotifyClientID = strings.TrimSpace(s.SpotifyClientID)
	if s.DiscordAppID = strings.TrimSpace(s.DiscordAppID); !discordIDRe.MatchString(s.DiscordAppID) {
		s.DiscordAppID = ""
	}
	if s.YouTubeChannel = strings.TrimSpace(s.YouTubeChannel); !pageIDRe.MatchString(s.YouTubeChannel) {
		s.YouTubeChannel = ""
	}
	if !slices.Contains([]string{"", "deno", "node", "bun", "quickjs"}, s.JSRuntime) {
		s.JSRuntime = ""
	}
	shortcuts := map[string]string{}
	for id, keys := range s.Shortcuts {
		if id != "" && len(id) <= 128 && len(keys) <= 64 && len(shortcuts) < 500 {
			shortcuts[id] = keys
		}
	}
	s.Shortcuts = shortcuts
	plugins := []string{}
	for _, id := range s.EnabledPlugins {
		if ValidID(id) && !slices.Contains(plugins, id) {
			plugins = append(plugins, id)
		}
	}
	s.EnabledPlugins = plugins
	if !strings.HasPrefix(s.RegistryURL, "https://") || s.RegistryURL == WebRegistryURL {
		s.RegistryURL = d.RegistryURL
	}
	return s
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
