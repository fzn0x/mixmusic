package library

import (
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// sameSlack is how far apart two places of an episode can be and still be
// the same, besides a percent of its length: YouTube Music tells a place
// to the percent, and heard MixMusic's own a report or so before it
// stopped.
const sameSlack = 30 * time.Second

// toldOutranks reports whether an episode starts where its service last
// saw the user, as told when t was read (Episode.ToldAt), rather than at
// pos, the place MixMusic keeps of its own. The service's place is taken
// when it is newer: it changed since MixMusic kept pos, which remembers the
// service's place of then in its track, and was read after. A place heard
// on a phone, further on or back, is such a change; one that didn't change
// is MixMusic's own, or one it chose not to start from before. When
// MixMusic never saw the service's place, which is newer can't be told, so
// the one further along is taken: a place partway is never undone by an
// older one. Either way, a place within sameSlack and a percent of
// MixMusic's own is that one, which keeps MixMusic's, exact to the second.
// An episode played counts as at its end.
func toldOutranks(pos Position, t media.Track) bool {
	told := t.Episode
	if told == nil || told.ToldAt.IsZero() {
		return false
	}
	length := millis(t.DurationMs)
	if length <= 0 {
		length = millis(pos.DurationMs)
	}
	slack := sameSlack + length/100
	there, here := placeOf(told.ResumeMs, told.Played, length), placeOf(pos.PositionMs, pos.Played, length)
	if told.Played == pos.Played && near(there, here, slack) {
		return false
	}
	if seen := pos.Track.Episode; seen != nil && !seen.ToldAt.IsZero() {
		moved := told.Played != seen.Played || !near(millis(told.ResumeMs), millis(seen.ResumeMs), length/100)
		return moved && told.ToldAt.After(seen.ToldAt)
	}
	return there > here+slack || told.Played && !pos.Played
}

// placeOf is where an episode of length was left: at ms, or at its end
// once played.
func placeOf(at int, played bool, length time.Duration) time.Duration {
	if played {
		return max(length, millis(at))
	}
	return millis(at)
}

// near reports whether two places are within slack of each other.
func near(a, b, slack time.Duration) bool { return a-b <= slack && b-a <= slack }

func millis(n int) time.Duration { return time.Duration(n) * time.Millisecond }
