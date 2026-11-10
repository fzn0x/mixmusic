package app

import "github.com/fzn0x/mixmusic/internal/media"

// Matched returns the YouTube Music track MixMusic matched to t, without
// searching: t itself for a YouTube Music track, and for a Spotify track
// the match found when it played or downloaded, if any. Sharing a song
// links to both of its pages with it.
func (p *Player) Matched(t media.Track) (media.Track, bool) {
	return p.a.matcher.Matched(t)
}
