package main

import (
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/locales"
)

func init() { addSettings(soundRows, 70, (*shell).skipAIRow) }

// skipAIRow skips the tracks YouTube labels as made with AI, in Settings ›
// Playback › Sound.
func (s *shell) skipAIRow(c *ui.Context) {
	s.row(c, locales.SkipAITitle(), locales.SkipAIDesc(), func() {
		if s.switchOf(c.Key("skip-ai"), &s.set.SkipAI).Label(locales.SkipAITitle()).Changed() {
			s.save()
		}
	})
}
