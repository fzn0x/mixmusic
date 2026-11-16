package main

import (
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/locales"
)

func init() { addSettings(colorTypeRows, 70, (*shell).wavyProgressRow) }

// wavyProgressRow draws the played part of the seek bar as a wave, in
// Settings › Appearance › Colour and type.
func (s *shell) wavyProgressRow(c *ui.Context) {
	s.row(c, locales.WavyProgressTitle(), locales.WavyProgressDescription(), func() {
		if s.switchOf(c.Key("wavy-progress"), &s.set.WavyProgress).Label(locales.WavyProgressTitle()).Changed() {
			s.save()
		}
	}, "seek bar", "squiggly", "wave")
}
