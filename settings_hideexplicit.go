package main

import (
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/locales"
)

func init() { addSettings(explicitRows, 20, (*shell).hideExplicitRow) }

// hideExplicitDescription says what Hide explicit content does.
func hideExplicitDescription() string { return locales.ExplicitHideDesc() }

// hideExplicitRow hides explicit content, in Settings › Playback ›
// Explicit content (explicit.go).
func (s *shell) hideExplicitRow(c *ui.Context) {
	s.row(c, locales.ExplicitHideTitle(), hideExplicitDescription(), func() {
		if s.switchOf(c.Key("hide-explicit"), &s.set.HideExplicit).Label(locales.ExplicitHideTitle()).Changed() {
			s.save()
		}
	}, "parental", "children", "kids", "family", "clean", "mature", "advisory")
}
