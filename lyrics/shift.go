package lyrics

import "time"

// Shifted returns synced lyrics with each time moved by d, the times of
// their words too, as a song's lyrics are moved to where the song starts
// in its music video (#565): a time moved before 0:00 is 0:00. Plain
// lyrics, which have no times, are as they are.
func (l Lyrics) Shifted(d time.Duration) Lyrics {
	if !l.Synced {
		return l
	}
	by := int(d.Milliseconds())
	lines := make([]Line, len(l.Lines))
	for i, line := range l.Lines {
		line.TimeMs = max(0, line.TimeMs+by)
		if line.Words != nil {
			words := make([]Word, len(line.Words))
			for j, w := range line.Words {
				w.StartMs, w.EndMs = max(0, w.StartMs+by), max(0, w.EndMs+by)
				words[j] = w
			}
			line.Words = words
		}
		lines[i] = line
	}
	l.Lines = lines
	return l
}
