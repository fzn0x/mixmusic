package player

import (
	"slices"

	"github.com/fzn0x/mixmusic/internal/media"
)

// Retagged takes in what tracks are now, as after the tags of their files
// changed: the entries of the queue of the same IDs take them, as the
// queue shows and keeps them, in its own order too while it's shuffled.
// What plays goes on playing.
func (p *Player) Retagged(tracks []media.Track) {
	if len(tracks) == 0 {
		return
	}
	p.do(func() {
		by := make(map[string]media.Track, len(tracks))
		for _, t := range tracks {
			by[t.ID] = t
		}
		changed := false
		retag := func(items []Item) []Item {
			var out []Item
			for i, it := range items {
				if t, ok := by[it.Track.ID]; ok {
					if out == nil {
						out = slices.Clone(items)
					}
					out[i].Track = t
				}
			}
			if out == nil {
				return items
			}
			changed = true
			return out
		}
		p.s.Queue = retag(p.s.Queue)
		p.unshuffled = retag(p.unshuffled)
		if changed {
			p.emit(QueueChange)
		}
	})
}
