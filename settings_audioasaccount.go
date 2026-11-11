package main

import "github.com/egoist/mygo/ui"

func init() { addSettings(soundRows, 15, (*shell).audioAsAccountRow) }

// audioAsAccountTitle names the switch that asks for YouTube Music's audio
// as the user's account, for Premium's higher bitrate (#471).
const audioAsAccountTitle = "Play audio as your account"

// audioAsAccountDescription tells what the switch brings and what it
// risks, in Settings › Playback › Sound.
const audioAsAccountDescription = "For YouTube Music Premium's higher bitrate, MixMusic asks for each track's audio as your " +
	"YouTube Music account. YouTube may flag an account used this way, and tracks may start more slowly. " +
	"A track it can't get this way plays as before."

// audioAsAccountRow asks for the audio of YouTube Music's tracks as the
// user's account, in Settings › Playback › Sound, beside Quality.
func (s *shell) audioAsAccountRow(c *ui.Context) {
	description := audioAsAccountDescription
	if !s.accounts().YouTube.SignedIn {
		description += " Sign in to YouTube Music in accounts first."
	}
	s.row(c, audioAsAccountTitle, description, func() {
		if s.switchOf(c.Key("audio-as-account"), &s.set.AudioAsAccount).Label(audioAsAccountTitle).Changed() {
			s.save()
		}
	}, "premium", "bitrate", "signed in", "youtube music")
}
