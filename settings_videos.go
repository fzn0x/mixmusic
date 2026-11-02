package main

import "github.com/egoist/mygo/ui"

func init() { addSettings("playback", 40, (*shell).musicVideoSettings) }

// musicVideoSettings show the picture of music videos, in Settings ›
// Playback.
func (s *shell) musicVideoSettings(c *ui.Context) {
	s.group(c, "Music videos", func() {
		desc := "On the now playing page, a music video plays its picture in place of the cover, muted and in time with the sound. The picture takes about 3 MB a minute more, under 1 MB with data saver."
		problem := s.videoProblem()
		if problem != "" {
			desc = problem
		}
		s.row(c, "Show the video of music videos", desc, func() {
			if s.switchOf(c.Key("music-videos"), &s.set.MusicVideos).Label("Show the video of music videos").Disabled(problem != "").Changed() {
				s.save()
			}
		})
	})
}
