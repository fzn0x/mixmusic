package main

import "github.com/egoist/mygo/ui"

func init() { addSettings(soundRows, 70, (*shell).skipAIRow) }

// skipAIRow skips the tracks YouTube labels as made with AI, in Settings ›
// Playback › Sound.
func (s *shell) skipAIRow(c *ui.Context) {
	s.row(c, "Skip songs made with AI", "Skips the tracks YouTube labels as made with AI. YouTube labels only some of them, so others still play.", func() {
		if s.switchOf(c.Key("skip-ai"), &s.set.SkipAI).Label("Skip songs made with AI").Changed() {
			s.save()
		}
	})
}
