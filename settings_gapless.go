package main

import (
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/locales"
)

func init() { addSettings(soundRows, 25, (*shell).gaplessAlbumsRow) }

// gaplessAlbumsRow plays the songs of one album straight on, without a
// crossfade, in Settings › Playback › Sound, right under Crossfade, while
// Crossfade is on (#601).
func (s *shell) gaplessAlbumsRow(c *ui.Context) {
	if s.set.Crossfade == 0 {
		return
	}
	s.row(c, locales.CrossfadeGapless(), locales.CrossfadeGaplessDesc(), func() {
		if s.switchOf(c.Key("gapless-albums"), &s.set.GaplessAlbums).Label(locales.CrossfadeGapless()).Changed() {
			s.save()
		}
	})
}
