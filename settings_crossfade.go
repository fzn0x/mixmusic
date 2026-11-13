package main

import (
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/locales"
)

func init() { addSettings(soundRows, 20, (*shell).crossfadeRow) }

// crossfadeRow overlaps the tracks, in Settings › Playback › Sound.
func (s *shell) crossfadeRow(c *ui.Context) {
	desc := locales.CrossfadeOff()
	if s.set.Crossfade > 0 {
		desc = locales.CrossfadeOverlap(s.set.Crossfade)
	}
	s.row(c, locales.CrossfadeTitle(), desc, func() {
		v := float64(s.set.Crossfade)
		if ui.StepSlider(c.Key("crossfade"), &v, 0, 12, 1).Width(200).Label(locales.CrossfadeTitle()).Changed() {
			s.set.Crossfade = int(v + 0.5)
			s.save()
		}
	})
}
