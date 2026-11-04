package library

import (
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// told is the episode of an hour id as read when YouTube Music told,
// at when, that the user left it at the minute min, or played it.
func told(id string, when time.Time, min float64, played bool) media.Track {
	t := episode(id)
	t.Episode.ResumeMs = int(min * 60000)
	t.Episode.Played = played
	t.Episode.ToldAt = when
	return t
}

// TestToldPlace starts an episode where YouTube Music last saw the user
// when that is newer than MixMusic's own place: it changed since MixMusic
// kept its own. When MixMusic never saw it, the place further along wins.
// A place near MixMusic's own keeps MixMusic's, and a played one starts
// from the start.
func TestToldPlace(t *testing.T) {
	monday := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	tuesday := monday.AddDate(0, 0, 1)
	for _, c := range []struct {
		name string
		// keep is the place MixMusic keeps: the track it played, and the
		// minute it left it at, or -1 for none, or 60 for played.
		keep    media.Track
		keptMin float64
		// play is the track that plays now, as read.
		play media.Track
		want time.Duration
	}{
		{"no place here", media.Track{}, -1, told("yt:e", tuesday, 20, false), 20*time.Minute - rewind},
		{"no place here, played there", media.Track{}, -1, told("yt:e", tuesday, 0, true), 0},
		{"never seen, further there", episode("yt:e"), 30, told("yt:e", tuesday, 40, false), 40*time.Minute - rewind},
		{"never seen, behind there", episode("yt:e"), 30, told("yt:e", tuesday, 10, false), 30*time.Minute - rewind},
		{"never seen, played there", episode("yt:e"), 30, told("yt:e", tuesday, 0, true), 0},
		{"never seen, played here", episode("yt:e"), 60, told("yt:e", tuesday, 40, false), 0},
		{"moved back there", told("yt:e", monday, 10, false), 30, told("yt:e", tuesday, 5, false), 5*time.Minute - rewind},
		{"moved on there", told("yt:e", monday, 10, false), 30, told("yt:e", tuesday, 45, false), 45*time.Minute - rewind},
		{"played there since", told("yt:e", monday, 10, false), 30, told("yt:e", tuesday, 0, true), 0},
		{"unchanged there", told("yt:e", monday, 10, false), 30, told("yt:e", tuesday, 10, false), 30*time.Minute - rewind},
		{"our own report there", told("yt:e", monday, 10, false), 30, told("yt:e", tuesday, 30.5, false), 30*time.Minute - rewind},
		{"an older read", told("yt:e", tuesday, 40, false), 30, told("yt:e", monday, 5, false), 30*time.Minute - rewind},
		{"played here, unchanged there", told("yt:e", monday, 40, false), 60, told("yt:e", tuesday, 40, false), 0},
		{"Spotify's", episode("sp:e"), 30, func() media.Track { t := episode("sp:e"); t.Episode.ResumeMs = 3000000; return t }(), 30*time.Minute - rewind},
	} {
		p, _ := openPositions(t)
		switch {
		case c.keptMin >= 60:
			p.SetPlayed(c.keep, true)
		case c.keptMin >= 0:
			p.Keep(c.keep, time.Duration(c.keptMin*float64(time.Minute)), time.Hour)
		}
		if got := p.ResumeAt(c.play); got != c.want {
			t.Errorf("%s: ResumeAt = %v, want %v", c.name, got, c.want)
		}
	}
}
