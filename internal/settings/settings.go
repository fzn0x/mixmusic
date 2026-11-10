// Package settings defines the user's preferences and their defaults.
package settings

import (
	"encoding/json"
	"regexp"
	"runtime"
	"strings"
)

// NativeAudio reports whether MixMusic's audio engine plays on this
// system: 64-bit Windows, macOS and Linux.
const NativeAudio = (runtime.GOOS == "windows" || runtime.GOOS == "darwin" || runtime.GOOS == "linux") &&
	(runtime.GOARCH == "amd64" || runtime.GOARCH == "arm64")

// Settings are the user's preferences, by section: each section's file
// holds its struct, its defaults and its part of Clean. The sections are
// embedded, so that their fields are the settings' own, in Go and in
// settings.json, whose keys come in the order of the sections: Lyrics's
// and Translation's are apart, as they were before there were sections.
//
// A new setting is a field of its section, with a doc comment of its own,
// and its key in testdata's default.json and full.json. No section has a
// MarshalJSON or UnmarshalJSON method, which Settings would take as its
// own: only Settings has one.
type Settings struct {
	Appearance
	Lyrics
	Home
	Playback
	Cache
	Translation
	Downloads
	LocalFiles
	Accounts
	Desktop
	Overlay
	Tools
	Keys
	Addons
}

// Default returns the settings of a new install.
func Default() Settings {
	var s Settings
	s.Appearance = defaultAppearance()
	s.Lyrics = defaultLyrics()
	s.Home = defaultHome()
	s.Playback = defaultPlayback()
	s.Cache = defaultCache()
	s.Downloads = defaultDownloads()
	s.LocalFiles = defaultLocalFiles()
	s.Accounts = defaultAccounts()
	s.Desktop = defaultDesktop()
	s.Overlay = defaultOverlay()
	s.Keys = defaultKeys()
	s.Addons = defaultAddons()
	return s
}

// idRe matches the ID of a theme or a plugin.
var idRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// UnmarshalJSON reads settings, those of the versions before 0.5.6 too,
// where Discord showed nothing until the user turned it on and pasted the
// ID of an application of theirs. Those settings show what plays now,
// unless the user had pasted an ID and turned it off.
func (s *Settings) UnmarshalJSON(data []byte) error {
	type plain Settings
	if err := json.Unmarshal(data, (*plain)(s)); err != nil {
		return err
	}
	s.readPaths()
	var keys struct {
		Before *bool `json:"discordPresence"`
		Now    *bool `json:"showOnDiscord"`
	}
	if json.Unmarshal(data, &keys) == nil && keys.Now == nil && keys.Before != nil {
		s.DiscordPresence = *keys.Before || strings.TrimSpace(s.DiscordAppID) == ""
	}
	return nil
}

// ValidID reports whether id can name a theme or a plugin: lowercase
// letters, digits and dashes.
func ValidID(id string) bool { return idRe.MatchString(id) }

// CleanPath is a path the user typed or pasted, without the spaces around
// it, or the double quotes Windows Explorer's Copy as path puts around it.
func CleanPath(p string) string {
	p = strings.TrimSpace(p)
	if len(p) >= 2 && p[0] == '"' && p[len(p)-1] == '"' {
		p = strings.TrimSpace(p[1 : len(p)-1])
	}
	return p
}

// Clean returns s with every value in its range, and the defaults in place
// of values that are missing or invalid. Its maps and slices are its own,
// which changing doesn't change s's.
func (s Settings) Clean() Settings {
	d := Default()
	s.Appearance = s.Appearance.clean(d.Appearance)
	s.Lyrics = s.Lyrics.clean()
	s.Home = s.Home.clean()
	s.Playback = s.Playback.clean(d.Playback)
	s.Cache = s.Cache.clean()
	s.Translation = s.Translation.clean()
	s.Downloads = s.Downloads.clean()
	s.LocalFiles = s.LocalFiles.clean()
	s.Accounts = s.Accounts.clean()
	s.Desktop = s.Desktop.clean()
	s.Overlay = s.Overlay.clean()
	s.Tools = s.Tools.clean()
	s.Keys = s.Keys.clean()
	s.Addons = s.Addons.clean(d.Addons)
	return s
}
