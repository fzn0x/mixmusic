package main

import (
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/locales"
)

func init() { addSettings("playback", 40, (*shell).musicVideoSettings) }

// musicVideoSettings show the picture of music videos, in Settings ›
// Playback.
func (s *shell) musicVideoSettings(c *ui.Context) {
	s.group(c, locales.VideosGroup(), func() {
		desc := locales.VideosDesc()
		problem := s.videoProblem()
		if problem != "" {
			desc = problem
		}
		s.row(c, locales.VideosShow(), desc, func() {
			if s.switchOf(c.Key("music-videos"), &s.set.MusicVideos).Label(locales.VideosShow()).Disabled(problem != "").Changed() {
				s.save()
			}
		})
	})
}
