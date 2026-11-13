package main

import (
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/locales"
)

func init() { addSettings(soundRows, 50, (*shell).surroundRow) }

// surroundRow plays the 5.1 surround mix of the tracks that have one, in
// Settings › Playback › Sound.
func (s *shell) surroundRow(c *ui.Context) {
	s.row(c, locales.SurroundTitle(), surroundDescription(), func() {
		if s.switchOf(c.Key("surround"), &s.set.Surround).Label(locales.SurroundTitle()).Changed() {
			s.save()
		}
	})
}

// surroundDescription tells what 5.1 surround does, in Settings › Playback.
func surroundDescription() string { return locales.SurroundDescription() }
