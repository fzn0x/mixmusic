package main

import (
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/locales"
)

// The row of Settings › Playback › Skip parts of videos that leaves the
// parts skipped out of exports too (#735): of copies of Opus, which the
// remux cuts, while M4A copies keep them.

func init() { addSettings(partRows, 35, (*shell).partExportsRow) }

// partExportsRow leaves the parts skipped out of exports, while parts are
// skipped.
func (s *shell) partExportsRow(c *ui.Context) {
	if !s.set.SkipParts.On {
		return
	}
	s.row(c, locales.SkipPartsExportsTitle(), locales.SkipPartsExportsAbout(), func() {
		if s.switchOf(c.Key("part-exports"), &s.set.SkipParts.Exports).Label(locales.SkipPartsExportsTitle()).Changed() {
			s.save()
		}
	}, "sponsorblock export copy cut")
}
