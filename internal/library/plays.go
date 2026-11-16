package library

import "maps"

// PlayCounts returns how many times each track was played, by its ID, as
// the play log has them: every play, episodes too. A track never played
// isn't there.
func (l *Library) PlayCounts() map[string]int {
	v := l.plays.view()
	return maps.Clone(memoized(&l.memo, l.stampOf(v), &l.memo.counts, struct{}{}, func() map[string]int { return playCounts(v) }))
}

// playCounts counts the plays of each track of v, by its ID.
func playCounts(v playsView) map[string]int {
	n := make([]int, len(v.tracks))
	for _, p := range v.plays {
		n[p.track]++
	}
	counts := map[string]int{}
	for i, c := range n {
		if c > 0 {
			counts[v.tracks[i].ID] += c
		}
	}
	return counts
}
