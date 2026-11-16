package main

import (
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/locales"
)

func init() { addSettings(soundRows, 15, (*shell).audioAsAccountRow) }

// audioAsAccountTitle names the switch that asks for YouTube Music's audio
// as the user's account, for Premium's higher bitrate (#471).
func audioAsAccountTitle() string { return locales.AudioAsAccountTitle() }

// audioAsAccountDescription tells what the switch brings and what it
// risks, in Settings › Playback › Sound.
func audioAsAccountDescription() string { return locales.AudioAsAccountDesc() }

// audioAsAccountRow asks for the audio of YouTube Music's tracks as the
// user's account, in Settings › Playback › Sound, beside Quality.
func (s *shell) audioAsAccountRow(c *ui.Context) {
	description := audioAsAccountDescription()
	if !s.accounts().YouTube.SignedIn {
		description = locales.AudioAsAccountSignIn()
	}
	s.row(c, audioAsAccountTitle(), description, func() {
		if s.switchOf(c.Key("audio-as-account"), &s.set.AudioAsAccount).Label(audioAsAccountTitle()).Changed() {
			s.save()
		}
	}, "premium", "bitrate", "signed in", "youtube music")
}
