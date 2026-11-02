package main

import (
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/settings"
)

func init() { addSettings(soundRows, 30, (*shell).loudnessRow) }

// loudnessRow turns loud tracks down to one level, in Settings › Playback
// › Sound.
func (s *shell) loudnessRow(c *ui.Context) {
	even := "Turns loud tracks down to one level, from YouTube's measure of each, so none jumps out. Quiet recordings keep their level."
	if s.set.SpotifyPlayback == settings.SpotifyPlaybackConnect || s.set.SpotifyPlayback == settings.SpotifyPlaybackHere && spotifyPlaysHere {
		even += " Spotify plays its own tracks at its own loudness."
	}
	s.row(c, "Even loudness", even, func() {
		if s.switchOf(c.Key("loudness"), &s.set.Loudness).Label("Even loudness").Changed() {
			s.save()
		}
	})
}

func init() { addSettings(soundRows, 40, (*shell).levellingRow) }

// levellingRow squeezes loud and quiet moments closer together, in
// Settings › Playback › Sound.
func (s *shell) levellingRow(c *ui.Context) {
	s.row(c, "Volume levelling", "Squeezes loud and quiet moments closer together as they play, and lifts quiet recordings.", func() {
		if s.switchOf(c.Key("normalize"), &s.set.Normalize).Label("Volume levelling").Changed() {
			s.save()
		}
	}, "normalize loudness")
}
