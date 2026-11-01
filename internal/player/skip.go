package player

// Passing over entries as the queue moves on by itself: at the end of a
// track, on Next and Previous, round the queue on repeat all, into a radio
// autoplay adds, and from the start of a list played. An entry is passed
// over for a reason (Item.Skip): the user disliked its track, or YouTube
// labels it as made with AI, as the settings skip those. An entry the user
// chose (Item.Chosen) plays all the same.
//
// The reasons the player knows as an entry is made, or later, it keeps in
// Item.Skip. The label of AI the app may have to ask YouTube for: a move
// checks such an entry before it plays it (labels.go).

import (
	"errors"
	"fmt"
	"slices"

	"github.com/fzn0x/mixmusic/internal/media"
)

// maxEmptyRadios stops autoplay once that many radios in a row added
// nothing that played.
const maxEmptyRadios = 3

// passState is what the player keeps of passing over entries, on its
// goroutine.
type passState struct {
	// gate is the check of the entry a move reached, which the move waits
	// for.
	gate *labelGate
	// skipped holds the entries passed over since a track last played,
	// told in one notice once one plays.
	skipped []skippedEntry
	// ahead counts the checks of the entries after aheadOf, the current
	// entry, as it plays.
	aheadOf string
	ahead   int
	// lastPlayed is the track that last played, which a radio starts from
	// when no entry is current.
	lastPlayed *media.Track
	// radios counts the radios added since a track last played.
	radios int
	// toldPaused is set once the user was told that the checks of labels
	// pause, until a check answers again.
	toldPaused bool
	// movedTo is the load token of the entry the player last moved to by
	// itself, rather than the user chose.
	movedTo int
}

// skippedEntry is an entry passed over, and why.
type skippedEntry struct {
	uid, title, why string
	// spotify is set for a Spotify track, of which YouTube labels the
	// recording on YouTube Music.
	spotify bool
}

// move is the player moving on by itself, forward or back, past the
// entries the queue passes over.
type move struct {
	dir int
	// ended is set when the current track ended, rather than the user
	// skipping it.
	ended bool
	// paused is set for a move that holds the entry it reaches paused, as
	// leaving a track the user disliked while it was paused.
	paused bool
	// leaving is set for a move from a current entry the queue now passes
	// over, which stops once there's nothing else.
	leaving bool
	// from is the UID of the entry current as the move began, or "".
	from string
	// wrapped is set once the move went round the queue, on repeat all.
	wrapped bool
	// held is set once the move stopped what played.
	held bool
	// passed counts the entries the move passed over, and ai those of them
	// YouTube labels as made with AI.
	passed, ai int
}

// moveOn starts a move forward from the current entry, at the index it
// returns, or from the entry a move waits for the check of: Next, pressed
// meanwhile, moves on from that one.
func (p *Player) moveOn(ended bool) (*move, int) {
	m := &move{dir: 1, ended: ended}
	at := p.s.Index
	if cur, ok := p.s.Current(); ok {
		m.from = cur.UID
	}
	if g := p.passing.gate; g != nil && g.token == p.loadToken {
		if i := p.indexOf(g.uid); i >= 0 {
			at = i
		}
		m.from, m.held, m.wrapped, m.leaving = g.m.from, true, g.m.wrapped, g.m.leaving
	}
	return m, at
}

// moveBack starts a move back from the current entry.
func (p *Player) moveBack() *move {
	m := &move{dir: -1}
	if cur, ok := p.s.Current(); ok {
		m.from = cur.UID
	}
	return m
}

// skipOf tells why the queue passes over a new entry of t: the user
// disliked it, or the app knows YouTube labels it as made with AI, as the
// settings skip those; "" for one that plays, or that waits for its check.
func (p *Player) skipOf(t media.Track) string {
	if why := p.svc.Skip(t); why != "" {
		return why
	}
	return p.labelSkip(t)
}

// step goes on with the move m from the entry at i: it plays the first
// entry from there, in m's direction, that the queue doesn't pass over,
// and once there is none, goes round, adds a radio or stops.
func (p *Player) step(m *move, i int) {
	q := p.s.Queue
	for ; i >= 0 && i < len(q); i += m.dir {
		if q[i].Skip == "" {
			p.goTo(m, i)
			return
		}
		p.passOver(m, q[i], q[i].Skip)
	}
	if m.dir < 0 {
		switch {
		case p.s.Repeat == RepeatAll && !m.wrapped && len(q) > 1:
			m.wrapped = true
			p.step(m, len(q)-1)
		case m.held:
			// Nothing before it plays: the current track, which the move
			// stopped, starts over.
			p.playAt(p.s.Index, 0, true)
		default:
			p.seek(0)
			p.tellSkipped()
		}
		return
	}
	p.pastEnd(m)
}

// pastEnd goes on with the move m past the end of the queue: round to its
// start on repeat all, to a radio of the last entry it doesn't pass over,
// with autoplay, or it stops there.
func (p *Player) pastEnd(m *move) {
	if p.s.Repeat == RepeatAll && !m.wrapped {
		m.wrapped = true
		p.step(m, 0)
		return
	}
	if m.leaving && !m.held {
		// The track the queue now passes over stops.
		p.halt()
		m.held = true
	}
	switch {
	case p.s.Repeat == RepeatAll:
		// Round the whole queue, nothing played.
		p.stopMove(m, "The rest of the queue is "+m.what())
		return
	case !p.svc.Settings().Autoplay:
		p.stopMove(m, "The rest of the queue is "+m.what())
		return
	case p.passing.radios >= maxEmptyRadios:
		p.stopMove(m, "Autoplay stopped: three radios in a row held only "+m.what())
		return
	}
	seed, ok := p.radioSeed()
	if !ok && p.s.Index < 0 && p.passing.lastPlayed != nil && p.skipOf(*p.passing.lastPlayed) == "" {
		// What played last, before a list whose tracks were all passed
		// over, and none became current.
		seed, ok = Item{Track: *p.passing.lastPlayed}, true
	}
	if !ok {
		p.stopMove(m, "")
		return
	}
	token := p.loadToken
	p.extendWithRadio(seed, func(added bool, err error) {
		switch {
		case token != p.loadToken:
		case added:
			p.step(m, p.s.Index+1)
		case m.ended && p.radioOffline(err):
			p.waitRadio()
		case errors.Is(err, errOnlyLabeled):
			m.ai++
			p.stopMove(m, "Autoplay stopped: the radio held nothing new but "+m.what())
		default:
			p.stopMove(m, "Autoplay stopped: the radio held nothing new but "+m.what())
		}
	})
}

// goTo plays the entry at i, which the move m reached, unless its label,
// known or checked first, has it passed over. An entry the user chose
// plays.
func (p *Player) goTo(m *move, i int) {
	entry := p.s.Queue[i]
	label := LabelNone
	if !entry.Chosen && p.checks(entry.Track) {
		label = p.knownLabel(entry.Track)
	}
	switch label {
	case LabelAI:
		p.mark(entry.Track.ID)
		p.passOver(m, entry, SkipMadeWithAI)
		p.step(m, i+m.dir)
		return
	case LabelUnknown:
		p.check(m, i)
		return
	case LabelPaused:
		p.tellPaused()
	}
	p.playAt(i, 0, !m.paused)
	p.passing.movedTo = p.loadToken
}

// hold stops what plays while a move checks the entry it reached: the
// current entry shows loading, and nothing it waited for comes back.
func (p *Player) hold() {
	p.loadToken++
	p.retry = retryState{}
	p.endWait()
	p.atEnd = false
	p.s.Error = ""
	p.s.Source = nil
	p.s.Buffering = true
	p.s.Loading = true
	p.loaded = false
	p.stream.Unload()
	p.emit(StateChange)
}

// passOver notes that the move m passed over the entry it, for why, to
// tell of it once a track plays; the entry the move began from isn't.
func (p *Player) passOver(m *move, it Item, why string) {
	if it.UID == m.from {
		return
	}
	m.passed++
	if why == SkipMadeWithAI {
		m.ai++
	}
	if slices.ContainsFunc(p.passing.skipped, func(s skippedEntry) bool { return s.uid == it.UID }) {
		return
	}
	p.passing.skipped = append(p.passing.skipped, skippedEntry{uid: it.UID, title: it.Track.Title, why: why,
		spotify: it.Track.Source == media.SourceSpotify})
}

// what names what the move m passed over, for its notices: the songs
// YouTube labels as made with AI, with those the user dislikes when it
// passed over those too.
func (m *move) what() string {
	if m.passed > m.ai {
		return "songs you dislike or YouTube labels as made with AI"
	}
	return "songs YouTube labels as made with AI"
}

// stopMove ends the move m, which found nothing to play past the entries
// the queue passes over: the current track stops, when it ended, or the
// move stopped it. It tells why, when the move passed over a track YouTube
// labels as made with AI: the queue passes over the tracks the user
// disliked without a word, as they know them.
func (p *Player) stopMove(m *move, why string) {
	p.passing.skipped = nil
	if m.held {
		// A Spotify device left in the middle of its track pauses.
		p.connect.stop(true)
		p.s.Loading, p.s.Buffering = false, false
		p.stopAtEnd(true)
	} else {
		p.stopAtEnd(m.ended)
	}
	if why != "" && m.ai > 0 {
		p.notify(Notice{Message: why, Settings: true})
	}
}

// sounded takes in a track playing: what was passed over to reach it is
// told, in one notice, and the radios start counting again.
func (p *Player) sounded() {
	if cur, ok := p.s.Current(); ok {
		t := cur.Track
		p.passing.lastPlayed = &t
	}
	p.passing.radios = 0
	p.tellSkipped()
}

// fadingInto takes in next, after the current entry, becoming current by
// a crossfade, which the engine tells no start of: it sounds, past the
// entries the queue passed over to reach it.
func (p *Player) fadingInto(next Item) {
	q := p.s.Queue
	m := &move{dir: 1}
	for k := 1; k < len(q) && p.s.Index >= 0; k++ {
		it := q[(p.s.Index+k)%len(q)]
		if it.UID == next.UID {
			break
		}
		if it.Skip != "" {
			p.passOver(m, it, it.Skip)
		}
	}
	t := next.Track
	p.passing.lastPlayed = &t
	p.passing.radios = 0
	p.tellSkipped()
}

// tellSkipped tells, in one notice, of the entries passed over since a
// track last played, when YouTube labels one of them as made with AI.
func (p *Player) tellSkipped() {
	s := p.passing.skipped
	p.passing.skipped = nil
	ai := 0
	for _, e := range s {
		if e.why == SkipMadeWithAI {
			ai++
		}
	}
	if ai == 0 {
		return
	}
	var msg string
	switch {
	case len(s) == 1 && s[0].spotify:
		msg = "Skipped " + s[0].title + ", whose recording on YouTube Music YouTube labels as made with AI"
	case len(s) == 1:
		msg = "Skipped " + s[0].title + ", which YouTube labels as made with AI"
	case ai < len(s):
		msg = fmt.Sprintf("Skipped %d songs you dislike or YouTube labels as made with AI", len(s))
	default:
		msg = fmt.Sprintf("Skipped %d songs YouTube labels as made with AI", len(s))
	}
	p.notify(Notice{Message: msg, Settings: true})
}

// allPassedOver tells why the queue would pass over every entry of items,
// for a list played whose tracks it all passes over.
func allPassedOver(items []Item) string {
	ai := 0
	for _, it := range items {
		if it.Skip == SkipMadeWithAI {
			ai++
		}
	}
	switch {
	case ai == 0:
		return "You dislike every song here"
	case ai == len(items):
		return "YouTube labels every song here as made with AI"
	}
	return "Every song here is one you dislike or YouTube labels as made with AI"
}

// leave moves on from the current entry, which the queue now passes over,
// to the next it doesn't, playing or paused as the current one was. At the
// end of the queue the current track stops, and the radio autoplay adds,
// from the last entry the queue doesn't pass over, carries on.
func (p *Player) leave() {
	m, at := p.moveOn(false)
	m.paused = !(p.s.Playing || p.s.Loading || p.s.Waiting && p.s.WillPlay)
	m.leaving = true
	p.step(m, at+1)
}
