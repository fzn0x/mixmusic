package library

// What the stats make of the play log holds until it changes (#545): the
// Stats page and Your year ask for their sums in each frame they show, and
// a list sorted by plays on each change of the library, while summing
// years of plays takes milliseconds. A change of the library counts too,
// for the top tracks' covers, which come from it.

import (
	"slices"
	"sync"
	"time"
)

// statsMemo holds what was made of the play log, and of the library, as
// they were at stamp.
type statsMemo struct {
	mu     sync.Mutex
	stamp  memoStamp
	stats  map[statsKey]Stats
	recaps map[recapKey]Recap
	years  map[*time.Location][]int
	counts map[struct{}]map[string]int
}

// memoStamp tells the play log and the library apart as they change.
type memoStamp struct{ plays, library uint64 }

// statsKey names the stats of the last days in a time zone. The days end
// now, which moves: the stats are counted again each minute.
type statsKey struct {
	days   int
	minute int64
	loc    *time.Location
}

// recapKey names the recap of a year in a time zone.
type recapKey struct {
	year int
	loc  *time.Location
}

// memoSize is how many results of a kind the memo holds; past it, it
// forgets them, as each minute makes new stats.
const memoSize = 16

// stampOf returns the stamp of the play log as v has it, and of the
// library as it is.
func (l *Library) stampOf(v playsView) memoStamp {
	return memoStamp{plays: v.changes, library: l.file.Version()}
}

// memoized returns what build made under key, in memos of m, while the
// stamp stays what it was then; else it builds it again, and keeps it.
func memoized[K comparable, V any](m *statsMemo, stamp memoStamp, memos *map[K]V, key K, build func() V) V {
	m.mu.Lock()
	if m.stamp != stamp {
		m.stamp, m.stats, m.recaps, m.years, m.counts = stamp, nil, nil, nil, nil
	}
	if v, ok := (*memos)[key]; ok {
		m.mu.Unlock()
		return v
	}
	m.mu.Unlock()
	v := build()
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stamp == stamp {
		if *memos == nil || len(*memos) >= memoSize {
			*memos = map[K]V{}
		}
		(*memos)[key] = v
	}
	return v
}

// clone returns s with slices of its own, which a caller may change.
func (s Stats) clone() Stats {
	s.TopTracks, s.TopArtists, s.Daily = slices.Clone(s.TopTracks), slices.Clone(s.TopArtists), slices.Clone(s.Daily)
	return s
}

// clone returns r with slices of its own, which a caller may change.
func (r Recap) clone() Recap {
	r.TopTracks, r.TopArtists, r.TopAlbums = slices.Clone(r.TopTracks), slices.Clone(r.TopArtists), slices.Clone(r.TopAlbums)
	return r
}
