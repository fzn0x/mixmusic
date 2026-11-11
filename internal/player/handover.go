package player

import (
	"slices"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// TakeOver replaces the queue with tracks, which another computer handed
// over in the order they play there (remote control, #353), and plays the
// one at i from at, or holds it there paused unless playing, as one the
// user chose. Shuffle and repeat become the other's: a queue shuffled
// there keeps its order here, which turning shuffle off keeps too, as its
// own order stayed there. The tracks before i count as played.
func (p *Player) TakeOver(tracks []media.Track, i int, at time.Duration, playing, shuffle bool, repeat Repeat) {
	tracks = slices.Clone(tracks)
	p.do(func() {
		if len(tracks) == 0 {
			return
		}
		items := p.items(tracks)
		i = min(max(i, 0), len(items)-1)
		items[i].Chosen, items[i].Skip = true, ""
		played := make(map[string]bool, i)
		for _, it := range items[:i] {
			played[it.UID] = true
		}
		p.played = played
		p.unshuffled = nil
		if shuffle {
			p.unshuffled = items
		}
		p.s.Shuffle = shuffle
		switch repeat {
		case RepeatAll, RepeatOne:
			p.s.Repeat = repeat
		default:
			p.s.Repeat = RepeatOff
		}
		if !playing {
			// What plays stops now, as the track handed over waits.
			p.pause()
		}
		// Nothing of the new queue is current until it plays, as for
		// playItems.
		p.s.Queue, p.s.Index = items, -1
		p.emit(QueueChange)
		p.playAt(i, max(at, 0), playing)
	})
}
