package audio

// Passing over parts of tracks (#548): the engine is told, by URL, the
// parts of a track to pass over, as the sponsor reads and intros
// SponsorBlock marks (SetSkips), and each deck of the track passes over
// them from its own position, on the audio thread, rather than on its time
// reports: a quantum of a part at most is heard. A deck that starts in a
// part, as one that Load, Preload or Crossfade start from the track's
// start, starts at its end, with no gap; one that reaches a part seeks
// past it; one that reaches a part that reaches the track's end ends
// there. Each part is passed over once a play, so that seeking back into
// one plays it.

import (
	"cmp"
	"slices"
	"sync/atomic"
	"time"
)

// Skip is a part of a track the engine passes over as it plays it.
type Skip struct {
	From, To time.Duration
	// Of is the length of the track the part was marked on, or 0 when it
	// isn't known: a track more than skipSlack longer or shorter passes
	// over none of it, as it would fall elsewhere in it.
	Of time.Duration
}

const (
	// skipSlack is how far the length of the track a part was marked on
	// may be from the track's own.
	skipSlack = 2 * time.Second
	// minSkip is the shortest part passed over: a shorter one plays.
	minSkip = time.Second
	// endSkip is how near the track's end a part ends that ends the track
	// where it starts.
	endSkip = time.Second
)

// Fits reports whether s, a part of a track of length, holds in it: it
// was marked on a track as long, within skipSlack, or the length of one or
// the other isn't known.
func (s Skip) Fits(length time.Duration) bool {
	return s.Of <= 0 || length <= 0 || (s.Of-length).Abs() <= skipSlack
}

// Spans returns the parts of skips a track of length passes over, in
// order: those that fit it, cut at its end when its length is known, of
// minSkip or longer, joined where they overlap or touch.
func Spans(skips []Skip, length time.Duration) []Skip {
	var out []Skip
	for _, s := range skips {
		if !s.Fits(length) {
			continue
		}
		s.From = max(s.From, 0)
		if length > 0 {
			s.To = min(s.To, length)
		}
		if s.To-s.From >= minSkip {
			out = append(out, Skip{From: s.From, To: s.To})
		}
	}
	slices.SortFunc(out, func(a, b Skip) int { return cmp.Compare(a.From, b.From) })
	joined := out[:0]
	for _, s := range out {
		if n := len(joined); n > 0 && s.From <= joined[n-1].To {
			joined[n-1].To = max(joined[n-1].To, s.To)
			continue
		}
		joined = append(joined, s)
	}
	if len(joined) == 0 {
		return nil
	}
	return joined
}

// Ends reports whether s, a part of a track of length, ends the track
// where it starts: it reaches within endSkip of the end.
func (s Skip) Ends(length time.Duration) bool { return length > 0 && s.To >= length-endSkip }

// skipPlan is what a deck passes over of its track, once it knows the
// track's length: its spans, in order. A new plan takes the place of the
// old, whose spans don't change but for passed.
type skipPlan struct{ spans []skipSpan }

// skipSpan is a part of a track a deck passes over, from frame from to
// frame to: ends is set for one that ends the track, and passed once the
// deck passed over it in the play.
type skipSpan struct {
	from, to int64
	ends     bool
	passed   atomic.Bool
}

// SkipEvents are the Events of an engine that hear of the parts it passes
// over, as the player's: Skipped tells that the track of url passed over
// the part from from to to, at its start, as it reached it, or at the end,
// which then ended it.
type SkipEvents interface {
	Skipped(url string, from, to time.Duration)
}

// SetSkips has the decks of url's track pass over skips as they play it,
// as Spans gives them of its length, from now on: those that play it now,
// and those that Load, Preload and Crossfade make of it later. A part a
// deck plays as skips come is passed over at once. Nil passes over
// nothing.
func (e *Engine) SetSkips(url string, skips []Skip) {
	skips = slices.Clone(skips)
	e.mu.Lock()
	if e.skips == nil {
		e.skips = map[string][]Skip{}
	}
	if len(skips) == 0 {
		delete(e.skips, url)
	} else {
		e.skips[url] = skips
	}
	// What the decks don't play is forgotten once many tracks were told
	// of: the player tells the next track's as it finds it.
	if len(e.skips) > maxSkipped {
		for u := range e.skips {
			if u != url && !e.plays(u) {
				delete(e.skips, u)
			}
		}
	}
	decks := e.decks
	e.mu.Unlock()
	for _, d := range decks {
		if d != nil && d.url == url {
			d.setSkips(skips)
		}
	}
}

// maxSkipped is how many tracks' parts the engine keeps.
const maxSkipped = 8

// plays reports whether a deck has url's track. Its caller holds mu.
func (e *Engine) plays(url string) bool {
	for _, d := range e.decks {
		if d != nil && d.url == url {
			return true
		}
	}
	return false
}

// setSkips has the deck pass over skips, from now on.
func (d *deck) setSkips(skips []Skip) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.skips = skips
	d.replan()
}

// replan makes the deck's plan of what it passes over again, of its skips
// and its track's length, once it read that: the spans passed over in the
// play stay so. Its caller holds mu.
func (d *deck) replan() {
	if !d.ready {
		return
	}
	spans := Spans(d.skips, d.duration)
	if len(spans) == 0 {
		d.plan.Store(nil)
		return
	}
	// The download from the start goes on past the parts, for the cache.
	if w, ok := d.src.(Whole); ok {
		w.KeepWhole()
	}
	old := d.plan.Load()
	p := &skipPlan{spans: make([]skipSpan, len(spans))}
	for i, s := range spans {
		sp := &p.spans[i]
		sp.from, sp.to, sp.ends = frames(s.From), frames(s.To), s.Ends(d.duration)
		if old != nil {
			for j := range old.spans {
				if o := &old.spans[j]; o.from == sp.from && o.to == sp.to {
					sp.passed.Store(o.passed.Load())
				}
			}
		}
	}
	d.plan.Store(p)
}

// passStart returns where the deck starts its track, of start: the end of
// a part it starts in, which it passed over then, unless the part ends the
// track, which it cuts at once. Its caller holds mu, once the deck read the
// track's length.
func (d *deck) passStart(start time.Duration) time.Duration {
	p := d.plan.Load()
	if p == nil {
		return start
	}
	at := frames(start)
	for i := range p.spans {
		s := &p.spans[i]
		if at < s.from || at >= s.to || !s.passed.CompareAndSwap(false, true) {
			continue
		}
		d.passes = append(d.passes, Skip{From: duration(s.from), To: duration(s.to)})
		if s.ends {
			d.cut.Store(true)
			return start
		}
		return duration(s.to)
	}
	return start
}

// holdsStill reports whether the deck holds still, silent, rather than play a
// part it passes over: as it reached one, until the engine's loop moved it
// past (passOver), or as it reached the one that ends its track. On the
// audio thread.
func (d *deck) holdsStill() bool {
	if d.cut.Load() || d.jump.Load() != nil {
		return true
	}
	p := d.plan.Load()
	if p == nil {
		return false
	}
	at := d.heard()
	for i := range p.spans {
		s := &p.spans[i]
		if at < s.from || at >= s.to || s.passed.Load() {
			continue
		}
		s.passed.Store(true)
		d.jump.Store(s)
		return true
	}
	return false
}

// passOver moves the deck past the part it reached, if any: to its end,
// or, for the part that ends the track, nowhere, cut there. On the
// engine's loop.
func (d *deck) passOver() {
	s := d.jump.Load()
	if s == nil {
		return
	}
	d.mu.Lock()
	d.passes = append(d.passes, Skip{From: duration(s.from), To: duration(s.to)})
	d.mu.Unlock()
	// The deck holds until it is cut, or the seek's ring is there.
	if s.ends {
		d.cut.Store(true)
		d.jump.CompareAndSwap(s, nil)
		return
	}
	d.seek(duration(s.to))
}

// passed returns the parts the deck passed over since it was last asked.
func (d *deck) passed() []Skip {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := d.passes
	d.passes = nil
	return out
}

// replay has the deck pass over its parts again, as a new play: a seek
// after its track ended starts one.
func (d *deck) replay() {
	p := d.plan.Load()
	if p == nil {
		return
	}
	np := &skipPlan{spans: make([]skipSpan, len(p.spans))}
	for i := range p.spans {
		np.spans[i].from, np.spans[i].to, np.spans[i].ends = p.spans[i].from, p.spans[i].to, p.spans[i].ends
	}
	d.plan.Store(np)
}

// fresh reports whether the deck holds its track as it was made to: not
// sought, and nothing of it played, as Preload leaves it.
func (d *deck) fresh() bool { return !d.sought.Load() && d.ring.Load().read.Load() == 0 }
