package main

import "github.com/egoist/mygo/ui"

func init() { addSettings(colorTypeRows, 70, (*shell).wavyProgressRow) }

// wavyProgressRow draws the played part of the seek bar as a wave, in
// Settings › Appearance › Colour and type.
func (s *shell) wavyProgressRow(c *ui.Context) {
	s.row(c, "Wavy progress bar", wavyProgressDescription, func() {
		if s.switchOf(c.Key("wavy-progress"), &s.set.WavyProgress).Label("Wavy progress bar").Changed() {
			s.save()
		}
	}, "seek bar", "squiggly", "wave")
}

// wavyProgressDescription tells what the wavy progress bar does.
const wavyProgressDescription = "The played part of the seek bar is a gentle wave that moves while music plays and lies flat while it's paused. " +
	"It holds still while your system asks for less motion."
