package player

import "slices"

// The queue's entries by their UIDs, as another computer names them
// (remote control, #353): what it saw a moment ago may have moved since,
// so an index would name another entry. Those that name no entry of the
// queue change nothing.

// PlayEntry plays the entry uid from its start, as one the user chose, as
// PlayAt does.
func (p *Player) PlayEntry(uid string) {
	p.do(func() {
		if i := p.indexOf(uid); i >= 0 {
			p.choose(i)
			p.playAt(i, 0, true)
		}
	})
}

// RemoveEntry takes the entry uid out of the queue, when it comes after
// the current one.
func (p *Player) RemoveEntry(uid string) {
	p.do(func() {
		i := p.indexOf(uid)
		if i < 0 || i <= p.s.Index {
			return
		}
		p.s.Queue = slices.Delete(slices.Clone(p.s.Queue), i, i+1)
		p.emit(QueueChange)
	})
}

// MoveEntry moves the entry uid to the place of the entry to, as dragging
// one onto the other in the queue does (Move): both come after the
// current one.
func (p *Player) MoveEntry(uid, to string) {
	p.do(func() {
		from, at := p.indexOf(uid), p.indexOf(to)
		if from < 0 || at < 0 || from == at || from <= p.s.Index || at <= p.s.Index {
			return
		}
		moved := p.s.Queue[from]
		q := slices.Delete(slices.Clone(p.s.Queue), from, from+1)
		p.s.Queue = slices.Insert(q, at, moved)
		p.emit(QueueChange)
	})
}
