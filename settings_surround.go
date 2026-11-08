package main

import "github.com/egoist/mygo/ui"

func init() { addSettings(soundRows, 50, (*shell).surroundRow) }

// surroundRow plays the 5.1 surround mix of the tracks that have one, in
// Settings › Playback › Sound.
func (s *shell) surroundRow(c *ui.Context) {
	s.row(c, "Play 5.1 surround when a track has it", surroundDescription, func() {
		if s.switchOf(c.Key("surround"), &s.set.Surround).Label("Play 5.1 surround when a track has it").Changed() {
			s.save()
		}
	})
}

// surroundDescription tells what 5.1 surround does, in Settings › Playback.
const surroundDescription = "Some tracks have a 5.1 surround mix beside their stereo one. It plays on six speakers where your output has them, " +
	"else mixed down to stereo. The 5.1 button by the player turns it on and off too."
