package audio

// The next track on the very next sample (#600): a track that Preload
// loaded on the idle deck, armed by its Cue's Follow, follows the one the
// active deck plays as that one ends, without a crossfade. As the active
// deck has played all it holds, its stretch's last frames too, or reaches
// the part that ends its track (skip.go), the audio thread fills the rest
// of that quantum from the armed deck, at full level, from its first
// frame, with no fade-in, and makes it the active deck: it allocates
// nothing and waits for nothing. The engine's loop then tells Followed in
// place of Playing(false) and Ended, and closes the deck that ended. A
// track that failed, a pause or a crossfade under way, follows nothing.

import "math"

// FollowEvents are the Events of an engine that hear of a track that
// followed the one before on its next sample, as Preload's Cue.Follow
// asks: Followed tells that the track of url plays now, in place of the
// Playing(false) and Ended the end of the one before would tell. An engine
// whose Events don't hear of it follows nothing.
type FollowEvents interface {
	Followed(url string)
}

// albumGlide is the time constant, in seconds, of the glide of a track
// that follows the one before at its loudness (Cue.Glide), to its own:
// the songs of one album don't step in level at their joins, and each
// plays at its own level a few seconds in.
const albumGlide = 1.0

// albumK is the share of the distance to its own loudness that such a
// track covers by a sample, as a smoother of albumGlide.
var albumK = 1 - math.Exp(-1/(albumGlide*Rate))

// deckRead is what a deck read of a quantum: the frames from from to n of
// it, which it holds at their place in the quantum, of r.
type deckRead struct {
	r       *ring
	from, n int
}

// played reports whether the deck played all of its track, of which r is
// the ring, its stretch's last frames too, rather than failing: on the
// audio thread, once it read.
func (d *deck) played(r *ring) bool {
	return r.done.Load() && r.readable() == 0 && !d.pending(r) && d.failure() == nil
}

// endsAtPart reports whether the deck holds still at the part that ends
// its track (skip.go), which ends it there: on the audio thread.
func (d *deck) endsAtPart() bool {
	if d.cut.Load() {
		return true
	}
	s := d.jump.Load()
	return s != nil && s.ends
}

// follow has the deck armed to follow d, the active deck, whose track
// ended at frame at of the quantum, play on from there, as the active
// deck, and reads its frames from there into reads. On the audio thread.
func (e *Engine) follow(d *deck, at int, reads *[2]deckRead) {
	next := e.takeOver(d)
	if next == nil {
		return
	}
	j := e.active
	r := next.ring.Load()
	reads[j] = deckRead{r: r, from: at, n: at + next.read(r, e.deckBuf[j][r.channels*at:r.channels*quantum])}
}

// takeOver makes the deck armed to follow d, the active deck, whose track
// ended, the active deck: at d's level, which it moves as d's does through
// the quantum, at full gain, from its own first frame. Gliding, it starts
// at the loudness d plays at, and glides to its own. The loop tells
// Followed, and closes d. It returns the deck that took over, or nil when
// none does: nothing is armed, a pause or a crossfade is under way, or the
// loop didn't tell of the last one yet. On the audio thread.
func (e *Engine) takeOver(d *deck) *deck {
	j := 1 - e.active
	next := e.decks[j]
	if !e.armed || next == nil || e.fading || e.paused || e.followed != nil || e.decks[e.active] != d {
		return nil
	}
	e.armed = false
	e.levels[j] = e.levels[e.active]
	next.gain = ramp{value: 1, target: 1}
	if e.glide && d.trimmed {
		next.trim = smoother{value: d.trim.value, target: d.trim.value, k: albumK}
		next.trimmed = true
	}
	e.decks[e.active] = nil
	e.active = j
	e.followed = d
	e.wake()
	return next
}

// disarm has the idle deck follow nothing, and returns the deck the active
// one followed while the loop hasn't told of it yet, for its caller to
// close: Load, Crossfade, Stop and Unload change what plays, which tells
// no Followed then. Its caller holds mu.
func (e *Engine) disarm() *deck {
	e.armed, e.glide = false, false
	d := e.followed
	e.followed = nil
	return d
}

// tellFollowed tells events that to's track plays now, as it followed
// from's, which ended: first the part that ended from's, if one did, and
// then Followed. It closes from. On the engine's loop.
func (e *Engine) tellFollowed(from, to *deck) {
	from.passOver()
	passes := from.passed()
	closeDecks([]*deck{from})
	if s, ok := e.events.(SkipEvents); ok {
		for _, p := range passes {
			s.Skipped(from.url, p.From, p.To)
		}
	}
	if f, ok := e.events.(FollowEvents); ok && to != nil {
		f.Followed(to.url)
	}
}
