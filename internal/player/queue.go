package player

import (
	"context"
	"math/rand/v2"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/fzn0x/mixmusic/internal/media"
)

// The queue's slice is never changed in place: State shares it.

// PlayTracks replaces the queue with tracks, and plays the one at start.
func (p *Player) PlayTracks(tracks []media.Track, start int) {
	tracks = slices.Clone(tracks)
	p.do(func() { p.playTracks(tracks, start) })
}

// PlayNext puts tracks right after the current one.
func (p *Player) PlayNext(tracks []media.Track) {
	tracks = slices.Clone(tracks)
	p.do(func() {
		items := p.items(tracks)
		if p.unshuffled != nil {
			// In the queue's own order too, they follow the current entry.
			at := 0
			if cur, ok := p.s.Current(); ok {
				at = indexIn(p.unshuffled, cur.UID) + 1
			}
			p.unshuffled = slices.Insert(slices.Clone(p.unshuffled), at, items...)
		}
		p.s.Queue = slices.Insert(slices.Clone(p.s.Queue), p.s.Index+1, items...)
		p.emit(QueueChange)
		if p.s.Index < 0 {
			p.playAt(0, 0, true)
		}
	})
}

// Enqueue adds tracks at the end of the queue, before the tracks of the
// radio.
func (p *Player) Enqueue(tracks []media.Track) {
	tracks = slices.Clone(tracks)
	p.do(func() {
		items := p.items(tracks)
		if u := p.unshuffled; u != nil {
			// In the queue's own order too.
			cur, _ := p.s.Current()
			p.unshuffled = slices.Insert(slices.Clone(u), enqueueAt(u, indexIn(u, cur.UID)), items...)
		}
		q := p.s.Queue
		p.s.Queue = slices.Insert(slices.Clone(q), enqueueAt(q, p.s.Index), items...)
		p.emit(QueueChange)
	})
}

// enqueueAt is where tracks added to q go: at its end, before the entries
// of the radio after the one at cur.
func enqueueAt(q []Item, cur int) int {
	at := len(q)
	for at > cur+1 && q[at-1].Radio {
		at--
	}
	return at
}

// indexIn is the index in q of the entry uid, or -1.
func indexIn(q []Item, uid string) int {
	if uid == "" {
		return -1
	}
	return slices.IndexFunc(q, func(it Item) bool { return it.UID == uid })
}

// RemoveAt takes the entry at i out of the queue, but the current one.
func (p *Player) RemoveAt(i int) {
	p.do(func() {
		if i == p.s.Index || i < 0 || i >= len(p.s.Queue) {
			return
		}
		p.s.Queue = slices.Delete(slices.Clone(p.s.Queue), i, i+1)
		if i < p.s.Index {
			p.s.Index--
		}
		p.emit(QueueChange)
	})
}

// Move moves the entry at from to to.
func (p *Player) Move(from, to int) {
	p.do(func() {
		q := p.s.Queue
		if from == to || from < 0 || from >= len(q) {
			return
		}
		cur, hasCur := p.s.Current()
		moved := q[from]
		q = slices.Delete(slices.Clone(q), from, from+1)
		q = slices.Insert(q, min(max(to, 0), len(q)), moved)
		p.s.Queue = q
		if hasCur {
			p.s.Index = p.indexOf(cur.UID)
		}
		p.emit(QueueChange)
	})
}

// ClearUpcoming empties the queue after the current track.
func (p *Player) ClearUpcoming() {
	p.do(func() {
		p.s.Queue = slices.Clone(p.s.Queue[:p.s.Index+1])
		p.emit(QueueChange)
	})
}

// SetShuffle keeps the current track and what played before it, and
// shuffles the rest of the queue after it; off, it puts the queue back in
// its own order.
func (p *Player) SetShuffle(on bool) { p.do(func() { p.setShuffle(on) }) }

// ToggleShuffle shuffles or unshuffles.
func (p *Player) ToggleShuffle() { p.do(func() { p.setShuffle(!p.s.Shuffle) }) }

// CycleRepeat repeats the queue, then the track, then nothing.
func (p *Player) CycleRepeat() {
	p.do(func() {
		switch p.s.Repeat {
		case RepeatOff:
			p.s.Repeat = RepeatAll
		case RepeatAll:
			p.s.Repeat = RepeatOne
		default:
			p.s.Repeat = RepeatOff
		}
		p.report()
		p.emit(StateChange)
	})
}

func (p *Player) items(tracks []media.Track) []Item {
	items := make([]Item, len(tracks))
	for i, t := range tracks {
		items[i] = p.item(t, false)
	}
	return items
}

func (p *Player) playTracks(tracks []media.Track, start int) {
	if len(tracks) == 0 {
		return
	}
	p.unshuffled = nil
	p.played = nil
	items := p.items(tracks)
	first := min(max(start, 0), len(items)-1)
	if p.s.Shuffle {
		p.unshuffled = items
		items = p.shuffledFrom(items, first)
		first = 0
	}
	p.s.Queue = items
	p.emit(QueueChange)
	p.playAt(first, 0, true)
}

func (p *Player) setShuffle(on bool) {
	if on == p.s.Shuffle {
		return
	}
	cur, hasCur := p.s.Current()
	q := p.s.Queue
	if on {
		p.unshuffled = q
		p.s.Queue = p.shuffledFrom(q, p.s.Index)
	} else if p.unshuffled != nil {
		present := map[string]bool{}
		for _, it := range q {
			present[it.UID] = true
		}
		var order []Item
		known := map[string]bool{}
		for _, it := range p.unshuffled {
			if present[it.UID] {
				order = append(order, it)
				known[it.UID] = true
			}
		}
		for _, it := range q {
			if !known[it.UID] {
				order = append(order, it)
			}
		}
		p.s.Queue = order
		p.unshuffled = nil
	}
	p.s.Shuffle = on
	if hasCur {
		p.s.Index = p.indexOf(cur.UID)
	}
	p.emit(QueueChange)
	p.report()
}

// shuffledFrom is q shuffled around its entry at, or -1: the entries
// before it that played stay before it, in their order, and the rest of q
// follows it in random order, the entries before it that haven't played
// included, as a list started in its middle leaves them. The user's own
// entries come first, then the radio's, so that what they add still goes
// before the radio's.
func (p *Player) shuffledFrom(q []Item, at int) []Item {
	out := make([]Item, 0, len(q))
	var own, radio []Item
	for i, it := range q {
		switch {
		case i == at:
		case i < at && p.played[it.UID]:
			out = append(out, it)
		case it.Radio:
			radio = append(radio, it)
		default:
			own = append(own, it)
		}
	}
	if at >= 0 && at < len(q) {
		out = append(out, q[at])
	}
	shuffle(own)
	shuffle(radio)
	out = append(out, own...)
	return append(out, radio...)
}

func shuffle(items []Item) {
	rand.Shuffle(len(items), func(i, j int) { items[i], items[j] = items[j], items[i] })
}

// markPlayed notes that the entry uid became current.
func (p *Player) markPlayed(uid string) {
	if p.played == nil {
		p.played = map[string]bool{}
	}
	p.played[uid] = true
}

// extendWithRadio adds tracks like the current one to the queue, and then
// tells whether it added any.
func (p *Player) extendWithRadio(then func(added bool)) {
	seed, ok := p.s.Current()
	if !ok {
		then(false)
		return
	}
	await(p, func(ctx context.Context) ([]media.Track, error) { return p.svc.Radio(ctx, seed.Track) }, func(found []media.Track, err error) {
		if err != nil {
			then(false)
			return
		}
		queued := make([]media.Track, len(p.s.Queue))
		for i, it := range p.s.Queue {
			queued[i] = it.Track
		}
		tracks := RadioTracks(found, queued)
		tracks = tracks[:min(len(tracks), radioMax)]
		if len(tracks) == 0 {
			then(false)
			return
		}
		items := make([]Item, len(tracks))
		for i, t := range tracks {
			items[i] = p.item(t, true)
		}
		p.s.Queue = append(slices.Clone(p.s.Queue), items...)
		if p.unshuffled != nil {
			p.unshuffled = append(slices.Clone(p.unshuffled), items...)
		}
		p.emit(QueueChange)
		then(true)
	})
}

// uploadMark matches what marks an upload in a title, in brackets:
// official video, lyrics, audio and the like.
var uploadMark = regexp.MustCompile(`(?i)[(\[][^)\]]*\b(?:official|video|audio|lyrics?|visuali[sz]er|mv|hd|4k)\b[^)\]]*[)\]]`)

// SongKey names a song across its uploads and versions: its title without
// what marks an upload, and its first artist, in letters and digits.
func SongKey(t media.Track) string {
	artist := ""
	if len(t.Artists) > 0 {
		artist = t.Artists[0].Name
	}
	return plain(uploadMark.ReplaceAllString(t.Title, "")) + "|" + plain(artist)
}

// plain keeps the letters and digits of s, in lower case.
func plain(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			return r
		}
		return -1
	}, strings.ToLower(s))
}

// RadioTracks keeps, of what a radio suggests, the songs not queued yet:
// neither the same upload nor another of the same song. Of a song offered
// as both, it keeps the song rather than its music video.
func RadioTracks(found, queued []media.Track) []media.Track {
	ids := map[string]bool{}
	songs := map[string]bool{}
	for _, t := range queued {
		ids[t.ID] = true
		songs[SongKey(t)] = true
	}
	keep := make([]bool, len(found))
	for _, video := range []bool{false, true} {
		for i, t := range found {
			if t.Video != video {
				continue
			}
			key := SongKey(t)
			if ids[t.ID] || songs[key] {
				continue
			}
			ids[t.ID], songs[key], keep[i] = true, true, true
		}
	}
	var out []media.Track
	for i, t := range found {
		if keep[i] {
			out = append(out, t)
		}
	}
	return out
}
