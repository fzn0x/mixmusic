package main

import "github.com/egoist/mygo/ui"

func init() { addSettings("playback", 42, (*shell).movingCoverSettings) }

// movingCoverSettings play the covers that move, in Settings › Playback,
// beside the picture of music videos (movingcover.go).
func (s *shell) movingCoverSettings(c *ui.Context) {
	s.group(c, "Moving covers", func() {
		const desc = "A GIF cover, of a mix or of music on this computer, moves on the now playing page, pausing with the music, and on the mix's page. Elsewhere, and while the system asks for less motion, it stays still."
		s.row(c, "Show moving covers", desc, func() {
			if s.switchOf(c.Key("moving-covers"), &s.set.MovingCovers).Label("Show moving covers").Changed() {
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
