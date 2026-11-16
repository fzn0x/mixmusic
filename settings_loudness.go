package main

import (
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/locales"
)

func init() { addSettings(soundRows, 30, (*shell).loudnessRow) }

// loudnessRow turns loud tracks down to one level, in Settings › Playback
// › Sound.
func (s *shell) loudnessRow(c *ui.Context) {
	even := locales.LoudnessEvenDesc()
	if s.set.SpotifyPlayback == settings.SpotifyPlaybackConnect || s.set.SpotifyPlayback == settings.SpotifyPlaybackHere && spotifyPlaysHere {
		even = locales.LoudnessEvenDescSpotify()
	}
	s.row(c, locales.LoudnessEven(), even, func() {
		if s.switchOf(c.Key("loudness"), &s.set.Loudness).Label(locales.LoudnessEven()).Changed() {
			s.save()
		}
	})
}

func init() { addSettings(soundRows, 40, (*shell).levellingRow) }

// levellingRow squeezes loud and quiet moments closer together, in
// Settings › Playback › Sound.
func (s *shell) levellingRow(c *ui.Context) {
	s.row(c, locales.LoudnessLevelling(), locales.LoudnessLevellingDesc(), func() {
		if s.switchOf(c.Key("normalize"), &s.set.Normalize).Label(locales.LoudnessLevelling()).Changed() {
			s.save()
		}
	}, "normalize loudness")
}
