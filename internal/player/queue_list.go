package player

// Lists that play while they're read (#543): a long list, as all of an
// artist's songs, plays from its first part, and the rest of it joins the
// queue as it comes, where it would have been had it come at once.

import (
	"math/rand/v2"
	"slices"

	"github.com/fzn0x/mixmusic/internal/media"
)

// List is a list of tracks that plays while the rest of it is read:
// PlayList plays its first part, and Add adds each part after it.
type List struct {
	p *Player
	// entries are the UIDs of the entries of its tracks, which only the
	// player's goroutine reads and changes.
	entries map[string]bool
}

// PlayList plays tracks, the first part of a list, as PlayAll does, and
// returns the list, whose Add adds the rest of it.
func (p *Player) PlayList(tracks []media.Track, shuffled bool) *List {
	l := &List{p: p, entries: map[string]bool{}}
	tracks = slices.Clone(tracks)
	p.do(func() {
		for _, it := range p.playAll(tracks, shuffled) {
			l.entries[it.UID] = true
		}
	})
	return l
}

// Add adds tracks, the part of the list after those it was given, to the
// queue, and tells whether the list is still there: once another queue
// replaced it, or none of its entries is left, it adds nothing, and the
// rest of the list needn't be read. In the queue's own order the tracks
// follow the list's last entry, and the current one; shuffled, each goes
// at random among the entries after the current one, before the radio's,
// as though the list had been shuffled whole. It waits for the player.
func (l *List) Add(tracks []media.Track) bool {
	tracks = slices.Clone(tracks)
	kept := make(chan bool, 1)
	l.p.do(func() { kept <- l.add(tracks) })
	select {
	case ok := <-kept:
		return ok
	case <-l.p.ctx.Done():
		return false
	}
}

func (l *List) add(tracks []media.Track) bool {
	p := l.p
	q := p.s.Queue
	last := l.last(q)
	if last < 0 {
		return false
	}
	if len(tracks) == 0 {
		return true
	}
	items := p.items(tracks)
	for _, it := range items {
		l.entries[it.UID] = true
	}
	if u := p.unshuffled; u != nil {
		at := l.last(u) + 1
		if at == 0 {
			cur, _ := p.s.Current()
			at = enqueueAt(u, indexIn(u, cur.UID))
		}
		p.unshuffled = slices.Insert(slices.Clone(u), at, items...)
	}
	if p.s.Shuffle {
		from := min(max(p.s.Index, 0)+1, len(q))
		p.s.Queue = scatter(q, from, max(enqueueAt(q, p.s.Index), from), items)
	} else {
		p.s.Queue = slices.Insert(slices.Clone(q), max(last, p.s.Index)+1, items...)
	}
	p.emit(QueueChange)
	return true
}

// last is the index of the list's last entry in q, or -1.
func (l *List) last(q []Item) int {
	for i := len(q) - 1; i >= 0; i-- {
		if l.entries[q[i].UID] {
			return i
		}
	}
	return -1
}

// scatter returns q with items put at random among its entries from from
// to to, which keep their order: each before, between or after them.
func scatter(q []Item, from, to int, items []Item) []Item {
	items = slices.Clone(items)
	shuffle(items)
	mid := q[from:to]
	n := len(mid) + len(items)
	taken := make([]bool, n)
	for _, i := range rand.Perm(n)[:len(items)] {
		taken[i] = true
	}
	out := make([]Item, 0, len(q)+len(items))
	out = append(out, q[:from]...)
	for i, a, b := 0, 0, 0; i < n; i++ {
		if taken[i] {
			out = append(out, items[b])
			b++
		} else {
			out = append(out, mid[a])
			a++
		}
	}
	return append(out, q[to:]...)
}
