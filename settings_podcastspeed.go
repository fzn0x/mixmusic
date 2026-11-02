package main

import "github.com/egoist/mygo/ui"

func init() { addSettings(speedRows, 20, (*shell).podcastSpeedRow) }

// podcastSpeedRow sets the speed of podcasts' episodes, beside the songs'.
func (s *shell) podcastSpeedRow(c *ui.Context) {
	s.speedRow(c, "podcast-speed", "Podcasts", "Speed of podcasts", &s.set.PodcastSpeed, "episodes spoken word")
}
