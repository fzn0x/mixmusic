package player

import "slices"

// PlayNextAt moves the entry at i of the queue right after the current one.
// It is one the user chose now, so it loses its radio mark and plays though
// the queue would pass over it, but for a track the settings hide as
// explicit. It does nothing for the current entry, or one already next.
func (p *Player) PlayNextAt(i int) {
	p.do(func() {
		q := p.s.Queue
		if i < 0 || i >= len(q) || i == p.s.Index {
			return
		}
		moved := q[i]
		moved.Radio, moved.Chosen, moved.Skip = false, true, p.keptSkip(moved.Track)
		cur, hasCur := p.s.Current()
		rest := slices.Delete(slices.Clone(q), i, i+1)
		at := 0
		if hasCur {
			at = indexIn(rest, cur.UID) + 1
		}
		p.s.Queue = slices.Insert(rest, at, moved)
		if hasCur {
			p.s.Index = p.indexOf(cur.UID)
		}
		if u := p.unshuffled; u != nil {
			// In the queue's own order too, it follows the current entry.
			own := slices.DeleteFunc(slices.Clone(u), func(it Item) bool { return it.UID == moved.UID })
			at := 0
			if hasCur {
				at = indexIn(own, cur.UID) + 1
			}
			p.unshuffled = slices.Insert(own, at, moved)
		}
		p.emit(QueueChange)
	})
}
