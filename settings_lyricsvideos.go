package main

// Time lyrics to music videos, in Settings › Playback › Lyrics (#565): the
// lyrics of a music video that are its song's move to where the song
// starts in the video, which the app measures (internal/app's
// lyricsvideo.go), and show in place as they're timed.

import (
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/locales"
)

func init() { addSettings(lyricsRows, 25, (*shell).timeVideosRow) }

// timeVideosRow switches the timing of music videos' lyrics to them.
func (s *shell) timeVideosRow(c *ui.Context) {
	s.row(c, locales.LyricsTimeVideosTitle(), locales.LyricsTimeVideosDescription(), func() {
		if s.switchOf(c.Key("lyrics-time-videos"), &s.set.LyricsTimeVideos).Label(locales.LyricsTimeVideosTitle()).Changed() {
			s.save()
			s.lyricsAgain()
		}
	}, "music video", "sync", "offset", "intro")
}

// followLyricsTiming has the lyrics of a music video looked up again as
// the app timed them to it, or gave up, until the window closes: the
// lyrics shown switch in place.
func (s *shell) followLyricsTiming() func() {
	return s.a.OnLyricsTimed(func(trackID string) { s.update(func() { s.lyricsTimed(trackID) }) })
}

// lyricsTimed looks the lyrics of the track trackID up again, keeping
// those shown until the new ones come.
func (s *shell) lyricsTimed(trackID string) { s.refetch("lyrics:" + trackID) }
