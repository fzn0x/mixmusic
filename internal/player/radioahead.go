package player

// Autoplay's radio found ahead (#595). While the last entry of the queue
// plays, the radio that carries on after it is asked for once the track
// played prepareAfter, as the next track's stream would be found, and its
// tracks join the queue then, as the radio's entries. The next track's
// stream and audio are then found ahead of time, and a crossfade fades
// into it, as into any other track. A move past the end of the queue while
// that radio is asked for waits for its answer, rather than ask again.

import (
	"context"

	"github.com/fzn0x/mixmusic/internal/media"
)

// radioAhead is autoplay's radio asked for ahead.
type radioAhead struct {
	// seed is the UID of the entry the radio starts from.
	seed string
	// asking is set until the radio's answer comes. then, when set, is the
	// move past the end of the queue that waits for it.
	asking bool
	then   func(added bool, err error)
	// added holds the UIDs of the entries the radio added.
	added []string
}

// radioFollows reports whether autoplay's radio of seed follows the
// current entry: nothing after it plays, the queue doesn't repeat,
// autoplay is on, and seed has a radio, as pastEnd asks.
func (p *Player) radioFollows(seed Item) bool {
	return p.s.Repeat == RepeatOff && p.s.nextIndex() < 0 && p.passing.radios < maxEmptyRadios &&
		!spoken(seed.Track) && seed.Track.Source != media.SourceFile && p.svc.Settings().Autoplay
}

// askRadioAhead asks for autoplay's radio while the last entry of the
// queue plays, once for each entry it starts from.
func (p *Player) askRadioAhead() {
	seed, ok := p.radioSeed()
	if !ok || p.radioAhead != nil && p.radioAhead.seed == seed.UID || !p.radioFollows(seed) {
		return
	}
	a := &radioAhead{seed: seed.UID, asking: true}
	p.radioAhead = a
	await(p, func(ctx context.Context) ([]media.Track, error) { return p.svc.Radio(ctx, seed.Track) }, func(found []media.Track, err error) {
		p.radioAheadFound(a, found, err)
	})
}

// waitRadioAhead has then, of a move past the end of the queue, wait for
// the radio of seed asked for ahead, and tells whether it does: no such
// radio is on its way, and the move asks for its own.
func (p *Player) waitRadioAhead(seed Item, then func(added bool, err error)) bool {
	a := p.radioAhead
	if a == nil || !a.asking || a.seed != seed.UID {
		return false
	}
	a.then = then
	return true
}

// radioAheadFound takes in the answer of the radio a asked for ahead. A
// move past the end of the queue that waits for it goes on with it, while
// the queue it was asked for is still there. Otherwise its tracks join the
// queue while the queue still carries on into it.
func (p *Player) radioAheadFound(a *radioAhead, found []media.Track, err error) {
	a.asking = false
	then := a.then
	a.then = nil
	if then != nil {
		if err == nil && p.indexOf(a.seed) >= 0 {
			then(p.addAhead(a, found))
			return
		}
		then(false, err)
		return
	}
	if err != nil || p.radioAhead != a {
		return // dropped meanwhile; one that failed is asked again at the end
	}
	if seed, ok := p.radioSeed(); ok && seed.UID == a.seed && p.radioFollows(seed) {
		_, _ = p.addAhead(a, found)
	}
}

// addAhead adds the tracks the radio a found to the queue, as addRadio
// does, and keeps the entries it added.
func (p *Player) addAhead(a *radioAhead, found []media.Track) (bool, error) {
	n := len(p.s.Queue)
	added, err := p.addRadio(found)
	if added {
		for _, it := range p.s.Queue[n:] {
			a.added = append(a.added, it.UID)
		}
	}
	return added, err
}
