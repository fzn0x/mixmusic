package library

// PlayCounts returns how many times each track was played, by its ID, as
// the history has them: the last 1,000 plays, episodes too. A track never
// played isn't there.
func (l *Library) PlayCounts() map[string]int {
	counts := map[string]int{}
	for _, p := range l.file.Get().History {
		counts[p.Track.ID]++
	}
	return counts
}
