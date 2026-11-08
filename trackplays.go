package main

import (
	"cmp"

	"github.com/fzn0x/mixmusic/internal/media"
)

// playCounter counts how many times each track was played in MixMusic,
// by its ID, from the history, which the library's version tells the
// changes of: app.Collection.
type playCounter interface {
	Version() uint64
	PlayCounts() map[string]int
}

// playsVersion is the version of the library a list sorted by sort shows
// the plays of: the library's for the sort by plays, whose order follows
// the plays as they're counted, and 0 for the others, which don't.
func playsVersion(sort string, plays playCounter) uint64 {
	if sort != "plays" || plays == nil {
		return 0
	}
	return plays.Version()
}

// playCounts are the plays of each track, by its ID, for the sort by
// plays, and nil for the others, which count none.
func playCounts(sort string, plays playCounter) map[string]int {
	if sort != "plays" || plays == nil {
		return nil
	}
	return plays.PlayCounts()
}

// byPlays orders tracks by how many times they were played, the most
// played first; tracks never played come last.
func byPlays(plays map[string]int) func(a, b media.Track) int {
	return func(a, b media.Track) int { return cmp.Compare(plays[b.ID], plays[a.ID]) }
}
