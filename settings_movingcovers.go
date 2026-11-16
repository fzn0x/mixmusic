package main

import (
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/locales"
)

func init() { addSettings("playback", 42, (*shell).movingCoverSettings) }

// movingCoverSettings play the covers that move, in Settings › Playback,
// beside the picture of music videos (movingcover.go).
func (s *shell) movingCoverSettings(c *ui.Context) {
	s.group(c, locales.MovingCoversGroup(), func() {
		s.row(c, locales.MovingCoversShow(), locales.MovingCoversDesc(), func() {
			if s.switchOf(c.Key("moving-covers"), &s.set.MovingCovers).Label(locales.MovingCoversShow()).Changed() {
				s.save()
			}
		}, "gif animated animation")
	})
}

// toggleMovingCovers turns the covers that move on or off.
func (s *shell) toggleMovingCovers() {
	s.set.MovingCovers = !s.set.MovingCovers
	s.save()
}
