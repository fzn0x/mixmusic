package main

// Time lyrics to music videos, in Settings › Playback › Lyrics (#565): the
// lyrics of a music video that are its song's move to where the song
// starts in the video, which the app measures (internal/app's
// lyricsvideo.go), and show in place as they're timed.

import "github.com/egoist/mygo/ui"

func init() { addSettings(lyricsRows, 25, (*shell).timeVideosRow) }

// timeVideosRow switches the timing of music videos' lyrics to them.
func (s *shell) timeVideosRow(c *ui.Context) {
	s.row(c, "Time lyrics to music videos", "When a music video's lyrics are its song's, MixMusic finds where the song starts in the video, comparing the audio of the two from YouTube on this computer, and moves the lines there. Off, they keep the song's times, and show without them when the video is longer or shorter than the song.", func() {
		if s.switchOf(c.Key("lyrics-time-videos"), &s.set.LyricsTimeVideos).Label("Time lyrics to music videos").Changed() {
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
