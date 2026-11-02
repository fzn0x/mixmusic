package main

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

func init() { addSettings(soundRows, 20, (*shell).crossfadeRow) }

// crossfadeRow overlaps the tracks, in Settings › Playback › Sound.
func (s *shell) crossfadeRow(c *ui.Context) {
	desc := "Off: tracks follow each other."
	if s.set.Crossfade > 0 {
		desc = fmt.Sprintf("Tracks overlap by %d seconds.", s.set.Crossfade)
	}
	s.row(c, "Crossfade", desc, func() {
		v := float64(s.set.Crossfade)
		if ui.StepSlider(c.Key("crossfade"), &v, 0, 12, 1).Width(200).Label("Crossfade").Changed() {
			s.set.Crossfade = int(v + 0.5)
			s.save()
		}
	})
}
