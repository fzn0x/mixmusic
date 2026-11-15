package main

// The gap between a playlist's count and its rows (#625): its service
// counts items MixMusic can't list, and the settings may hide songs. Once
// the playlist is read, a muted line under its header says how many, and
// of what kind, so the count and the list agree. Any service fills
// media.Playlist.Unplayable, and the line tells the same for each.

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
)

// gapText says what the playlist leaves out of its count: the items of
// left, which can't play here, and hidden songs, which the settings hide.
// It's "" when nothing is left out.
func gapText(noun string, left media.Unplayable, hidden int) string {
	var parts []string
	if n := left.Total(); n > 0 {
		var kinds []string
		if left.Local > 0 {
			kinds = append(kinds, plural(left.Local, "local file"))
		}
		if left.Episodes > 0 {
			kinds = append(kinds, plural(left.Episodes, "episode"))
		}
		if left.Gone > 0 {
			kinds = append(kinds, plural(left.Gone, "removed or unavailable item", "removed or unavailable items"))
		}
		parts = append(parts, thousands(n)+" of this "+noun+"'s items can't play here: "+joinAnd(kinds)+".")
	}
	if hidden > 0 {
		parts = append(parts, plural(hidden, "explicit song")+" "+map[bool]string{true: "is", false: "are"}[hidden == 1]+" hidden by your settings.")
	}
	return strings.Join(parts, " ")
}

// joinAnd lists words as "a, b and c".
func joinAnd(words []string) string {
	if len(words) < 2 {
		return strings.Join(words, "")
	}
	return strings.Join(words[:len(words)-1], ", ") + " and " + words[len(words)-1]
}

// gapLine shows gapText under the header of the playlist r reads, once all
// of it is read: not while it loads, nor after it failed or was cut, which
// the line of readLine tells. shown are the tracks the page lists.
func (s *shell) gapLine(c *ui.Context, r *playlistRead, shown []media.Track, noun string) {
	if r.reading || r.err != nil || r.p.Cut {
		return
	}
	text := gapText(noun, r.p.Unplayable, len(r.p.Tracks)-len(shown))
	if text == "" {
		return
	}
	k := &s.lk
	ui.Text(c, text).FontSize(k.smallSize()).TextColor(k.muted).Margin(0, 0, 16).MaxWidth(640)
}

// shortReadAbove and shortReadShare tell when a YouTube Music playlist
// that was read to its end came up short (#627): its header counts more
// than shortReadAbove songs, and fewer than shortReadShare of them came.
// A smaller gap stays quiet, since the count may hold videos that are
// hidden or no longer available.
const (
	shortReadAbove = 100
	shortReadShare = 0.9
)

// readShort tells whether p, read to its end with no error and no cut,
// holds far fewer tracks than its service counts, so that the page can't
// look complete. Only YouTube Music is told: Spotify's count holds items
// MixMusic leaves out, which the line of gapText names.
func readShort(p media.Playlist) bool {
	return p.Source == media.SourceYouTube && !p.Cut && p.More == "" &&
		p.TrackCount > shortReadAbove && float64(len(p.Tracks)) < shortReadShare*float64(p.TrackCount)
}

// shortLine tells that r, a playlist read to its end, came up short, with
// Try again: it loads the playlist from its first page, since no next page
// is left to read on from.
func (s *shell) shortLine(c *ui.Context, r *playlistRead, noun string) {
	k := &s.lk
	text := "Only " + thousands(len(r.p.Tracks)) + " of this " + noun + "'s " + thousands(r.p.TrackCount) + " tracks came from YouTube Music."
	ui.Row(c).Gap(12).Margin(0, 0, 16).Children(func() {
		ui.Text(c, text).FontSize(k.smallSize()).TextColor(k.muted).MaxWidth(640)
		if s.button(c, "retry", "try again", false).Clicked() {
			s.refetch(strings.TrimSuffix(r.key, " read"))
		}
	})
}
