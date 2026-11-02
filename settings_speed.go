package main

import (
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/settings"
)

// speedSettings sets the speed of what MixMusic plays itself, at its own
// pitch: songs, for now.
func (s *shell) speedSettings(c *ui.Context) {
	s.group(c, "Speed", func() {
		s.speedRow(c, "song-speed", "Songs", "Speed of songs", &s.set.SongSpeed)
		// Spotify's players have no speed to set, and a live stream has
		// nothing ahead of its live edge to play faster.
		s.hint(c, "Speed changes what MixMusic plays itself: YouTube Music, and Spotify tracks played from it. Tracks Spotify plays, in MixMusic or on Spotify Connect, keep their own speed. So do live streams, which play as they're broadcast.")
	})
}

// speedRow sets a speed, from settings.MinSpeed to settings.MaxSpeed by
// settings.SpeedStep, and offers the tracks' own while another is set. The
// search of the settings finds it by the words of a speed, at any.
func (s *shell) speedRow(c *ui.Context, key, title, label string, v *float64) {
	desc := "Their own speed."
	if *v != 1 {
		desc = speedLabel(*v) + ", at their own pitch."
	}
	s.row(c, title, desc, func() {
		ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
			// Left of the slider, which stays where it is as it comes.
			if *v != 1 && s.link(c, key+"-normal", "normal") {
				*v = 1
				s.save()
			}
			x := *v
			if ui.Slider(c.Key(key), &x, settings.MinSpeed, settings.MaxSpeed).Step(settings.SpeedStep).Width(200).Label(label).Changed() {
				*v = settings.CleanSpeed(x)
				s.save()
			}
		})
	}, "faster slower tempo rate pitch")
}
