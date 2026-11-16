package lyrics

import (
	"math"
	"time"
)

// Shifted returns synced lyrics with each time t moved to shift + t ×
// speed, the times of their words too, as a song's lyrics are moved to
// where the song plays in its music video (#565): from where it starts in
// it, at the speed the video plays it, 1 for the song's own (#748). A
// time moved before 0:00 is 0:00. Plain lyrics, which have no times, are
// as they are.
func (l Lyrics) Shifted(shift time.Duration, speed float64) Lyrics {
	if !l.Synced {
		return l
	}
	by := float64(shift.Milliseconds())
	at := func(ms int) int { return max(0, int(math.Round(by+float64(ms)*speed))) }
	lines := make([]Line, len(l.Lines))
	for i, line := range l.Lines {
		line.TimeMs = at(line.TimeMs)
		if line.Words != nil {
			words := make([]Word, len(line.Words))
			for j, w := range line.Words {
				w.StartMs, w.EndMs = at(w.StartMs), at(w.EndMs)
				words[j] = w
			}
			line.Words = words
		}
		lines[i] = line
	}
	l.Lines = lines
	return l
}
