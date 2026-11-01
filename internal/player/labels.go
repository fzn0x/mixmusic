package player

// Skipping the tracks YouTube labels as made with AI, as the settings may
// ask: the player passes over them as it moves on by itself, at the end
// of a track, on Next and Previous, as the queue goes round on repeat, as
// autoplay adds a radio, and from the start of a list played. A track the
// user chose plays, labeled or not.
//
// A track whose label the app doesn't know is checked before it plays,
// while its stream is found: what played stops meanwhile, and nothing is
// told of the track, to the app or the subscribers, until its check lets
// it play. A check that takes long lets it play unchecked.

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// Label is what the app knows of the label YouTube gives the recording of
// a track: whether it says it was made with AI.
type Label int

const (
	// LabelNone: YouTube doesn't label it so.
	LabelNone Label = iota
	// LabelAI: YouTube labels it as made with AI.
	LabelAI
	// LabelUnknown: it couldn't be checked, as when YouTube didn't answer.
	LabelUnknown
	// LabelPaused: it isn't checked for now, as YouTube failed the checks
	// before it.
	LabelPaused
)

// Labels are what the player asks of the app of YouTube's labels, when its
// Services have them too: without, the player skips nothing.
type Labels interface {
	// Label tells whether YouTube labels the recording of track on YouTube
	// Music as made with AI, asking YouTube when it must, until ctx ends.
	// The checks of tracks about to play, soon, go before the others.
	Label(ctx context.Context, track media.Track, soon bool) Label
	// KnownLabel is what Label tells without asking YouTube: LabelUnknown
	// when it would have to ask.
	KnownLabel(track media.Track) Label
}

var (
	// labelWait is how long a track waits for its check before it plays
	// unchecked.
	labelWait = 10 * time.Second
)

const (
	// labelAhead is how many entries after the current one the player
	// checks, as it finds them labeled, before the next one plays.
	labelAhead = 5
	// maxEmptyRadios stops autoplay once that many radios in a row added
	// nothing that played.
	maxEmptyRadios = 3
)

// The notices of skipping.
const (
	restLabeled   = "YouTube labels the rest of the queue as made with AI"
	radiosLabeled = "Autoplay stopped: three radios in a row held only songs YouTube labels as made with AI"
	radioLabeled  = "Autoplay stopped: the radio held nothing new but songs YouTube labels as made with AI"
	pausedChecks  = "YouTube isn't answering MixMusic's checks for AI labels, so tracks play unchecked for now"
)

// errOnlyLabeled is why a radio added nothing: all it held that wasn't
// queued is labeled as made with AI.
var errOnlyLabeled = errors.New("the radio held only songs labeled as made with AI")

// skipState is what the player keeps of skipping, on its goroutine.
type skipState struct {
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
	// toldPaused is set once the user was told that the checks pause,
	// until a check answers again.
	toldPaused bool
	// movedTo is the load token of the entry the player last moved to by
	// itself, rather than the user chose.
	movedTo int
}

// labelGate is the check of an entry a move reached, while what played
// stops.
type labelGate struct {
	token int
	uid   string
	m     *move
}

// skippedEntry is an entry passed over.
type skippedEntry struct {
	uid, title string
	// spotify is set for a Spotify track, of which YouTube labels the
	// recording on YouTube Music.
	spotify bool
}

// move is the player moving on by itself, forward or back, past the
// entries it skips.
type move struct {
	dir int
	// ended is set when the current track ended, rather than the user
	// skipping it.
	ended bool
	// from is the UID of the entry current as the move began, or "".
	from string
	// wrapped is set once the move went round the queue, on repeat all.
	wrapped bool
	// held is set once the move stopped what played, to check an entry.
	held bool
	// passed counts the entries the move passed over.
	passed int
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
	if g := p.ai.gate; g != nil && g.token == p.loadToken {
		if i := p.indexOf(g.uid); i >= 0 {
			at = i
		}
		m.from, m.held, m.wrapped = g.m.from, true, g.m.wrapped
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

// labels returns the app's Labels, if its services have them.
func (p *Player) labels() (Labels, bool) {
	l, ok := p.svc.(Labels)
	return l, ok
}

// skipping reports whether the player skips what YouTube labels as made
// with AI: the settings ask it, and the app knows the labels.
func (p *Player) skipping() bool {
	_, ok := p.labels()
	return ok && p.s.SkipAI
}

// streams reports whether t plays from YouTube Music, rather than on a
// Spotify device, as the settings play Spotify tracks.
func streams(t media.Track, spotify settings.SpotifyPlayback, here Device) bool {
	switch t.Source {
	case media.SourceYouTube:
		return true
	case media.SourceSpotify:
		return spotify == settings.SpotifyPlaybackMatch || spotify == settings.SpotifyPlaybackHere && here == nil
	}
	return false
}

// checks reports whether the player checks the label of t before it
// plays: it skips what YouTube labels, and t plays from YouTube Music.
func (p *Player) checks(t media.Track) bool {
	return p.skipping() && streams(t, p.spotifyMode, p.here)
}

// knownLabel is the label the app knows of t, without asking YouTube.
func (p *Player) knownLabel(t media.Track) Label {
	l, ok := p.labels()
	if !ok {
		return LabelNone
	}
	return l.KnownLabel(t)
}

// step goes on with the move m from the entry at i: it plays the first
// entry from there, in m's direction, that the player doesn't pass over,
// and once there is none, goes round, adds a radio or stops.
func (p *Player) step(m *move, i int) {
	q := p.s.Queue
	for ; i >= 0 && i < len(q); i += m.dir {
		if !p.s.Skips(q[i]) {
			p.goTo(m, i)
			return
		}
		p.passOver(m, q[i])
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
// start on repeat all, to a radio with autoplay, or it stops there.
func (p *Player) pastEnd(m *move) {
	switch {
	case p.s.Repeat == RepeatAll && !m.wrapped:
		m.wrapped = true
		p.step(m, 0)
	case p.s.Repeat == RepeatAll:
		// Round the whole queue, nothing played.
		p.stopMove(m, restLabeled)
	case p.svc.Settings().Autoplay:
		if p.skipping() && p.ai.radios >= maxEmptyRadios {
			p.stopMove(m, radiosLabeled)
			return
		}
		token := p.loadToken
		p.extendWithRadio(func(added bool, err error) {
			switch {
			case added && token == p.loadToken:
				p.step(m, p.s.Index+1)
			case !added && m.ended && token == p.loadToken && p.radioOffline(err):
				p.waitRadio()
			case !added && token == p.loadToken && (m.passed > 0 || errors.Is(err, errOnlyLabeled)):
				p.stopMove(m, radioLabeled)
			case !added && token == p.loadToken && m.held:
				p.stopMove(m, "")
			case !added:
				p.stopAtEnd(m.ended)
			}
		})
	case m.passed > 0:
		p.stopMove(m, restLabeled)
	case m.held:
		p.stopMove(m, "")
	default:
		p.stopAtEnd(m.ended)
	}
}

// goTo plays the entry at i, which the move m reached, unless its label,
// known or checked first, has it passed over.
func (p *Player) goTo(m *move, i int) {
	entry := p.s.Queue[i]
	label := LabelNone
	if p.checks(entry.Track) {
		label = p.knownLabel(entry.Track)
	}
	switch label {
	case LabelAI:
		p.mark(entry.Track.ID)
		p.passOver(m, entry)
		p.step(m, i+m.dir)
		return
	case LabelUnknown:
		p.check(m, i)
		return
	case LabelPaused:
		p.tellPaused()
	}
	p.playAt(i, 0, true)
	p.ai.movedTo = p.loadToken
}

// check checks the label of the entry at i, which the move m reached,
// before it plays. What played stops meanwhile, and shows loading, while
// nothing is told of the entry, until it plays, or is passed over. Its
// stream is found meanwhile, to play at once once checked.
func (p *Player) check(m *move, i int) {
	entry := p.s.Queue[i]
	labels, _ := p.labels()
	p.hold()
	m.held = true
	token := p.loadToken
	p.ai.gate = &labelGate{token: token, uid: entry.UID, m: m}
	await(p, func(ctx context.Context) (Source, error) { return p.svc.Prepare(ctx, entry.Track) }, func(Source, error) {})
	await(p, func(ctx context.Context) (Label, error) {
		ctx, cancel := context.WithTimeout(ctx, labelWait)
		defer cancel()
		return labels.Label(ctx, entry.Track, true), nil
	}, func(label Label, _ error) {
		p.learned(entry.Track, label)
		if token != p.loadToken {
			return // the user chose another track meanwhile
		}
		p.ai.gate = nil
		at := p.indexOf(entry.UID)
		switch {
		case at < 0:
			// Taken out of the queue meanwhile: the move goes on from its
			// place.
			if m.dir < 0 {
				i--
			}
			p.step(m, i)
		case label == LabelAI:
			p.passOver(m, entry)
			p.step(m, at+m.dir)
		default:
			p.playAt(at, 0, true)
			p.ai.movedTo = p.loadToken
		}
	})
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

// learned takes in the label a check told of t: the entries of a track
// labeled are marked, and a pause of the checks is told once.
func (p *Player) learned(t media.Track, l Label) {
	switch l {
	case LabelAI:
		p.mark(t.ID)
		p.ai.toldPaused = false
	case LabelNone:
		p.ai.toldPaused = false
	case LabelPaused:
		p.tellPaused()
	}
}

// tellPaused tells, once, that tracks play unchecked for now.
func (p *Player) tellPaused() {
	if !p.ai.toldPaused {
		p.ai.toldPaused = true
		p.notify(Notice{Message: pausedChecks, Settings: true})
	}
}

// passOver notes that the move m passed over the entry it, to tell of it
// once a track plays; the entry the move began from isn't.
func (p *Player) passOver(m *move, it Item) {
	if it.UID == m.from {
		return
	}
	m.passed++
	if slices.ContainsFunc(p.ai.skipped, func(s skippedEntry) bool { return s.uid == it.UID }) {
		return
	}
	p.ai.skipped = append(p.ai.skipped, skippedEntry{uid: it.UID, title: it.Track.Title, spotify: it.Track.Source == media.SourceSpotify})
}

// stopMove ends the move m, which found nothing to play past the entries
// it skips, telling why, unless why is "": the current track stops, when
// it ended, or the move stopped it.
func (p *Player) stopMove(m *move, why string) {
	p.ai.skipped = nil
	if m.held {
		// A Spotify device left in the middle of its track pauses.
		p.connect.stop(true)
		p.s.Loading, p.s.Buffering = false, false
		p.stopAtEnd(true)
	} else {
		p.stopAtEnd(m.ended)
	}
	if why != "" {
		p.notify(Notice{Message: why, Settings: true})
	}
}

// sounded takes in a track playing: what was passed over to reach it is
// told, in one notice, and the radios start counting again.
func (p *Player) sounded() {
	if cur, ok := p.s.Current(); ok {
		t := cur.Track
		p.ai.lastPlayed = &t
	}
	p.ai.radios = 0
	p.tellSkipped()
}

// fadingInto takes in next, after the current entry, becoming current by
// a crossfade, which the engine tells no start of: it sounds, past the
// entries the player passed over to reach it.
func (p *Player) fadingInto(next Item) {
	q := p.s.Queue
	m := &move{dir: 1}
	for k := 1; k < len(q) && p.s.Index >= 0; k++ {
		it := q[(p.s.Index+k)%len(q)]
		if it.UID == next.UID {
			break
		}
		if p.s.Skips(it) {
			p.passOver(m, it)
		}
	}
	t := next.Track
	p.ai.lastPlayed = &t
	p.ai.radios = 0
	p.tellSkipped()
}

// tellSkipped tells of the entries passed over since a track last played,
// in one notice.
func (p *Player) tellSkipped() {
	s := p.ai.skipped
	if len(s) == 0 {
		return
	}
	p.ai.skipped = nil
	msg := fmt.Sprintf("Skipped %d songs YouTube labels as made with AI", len(s))
	switch {
	case len(s) == 1 && s[0].spotify:
		msg = "Skipped " + s[0].title + ", whose recording on YouTube Music YouTube labels as made with AI"
	case len(s) == 1:
		msg = "Skipped " + s[0].title + ", which YouTube labels as made with AI"
	}
	p.notify(Notice{Message: msg, Settings: true})
}

// mark marks the entries of the track id as made with AI.
func (p *Player) mark(id string) { p.setMark(id, true) }

// setMark sets or clears the mark of the entries of the track id, in the
// queue and in its own order, both cloned: what is published shares them.
func (p *Player) setMark(id string, on bool) {
	changed := false
	set := func(q []Item) []Item {
		var out []Item
		for i, it := range q {
			if it.Track.ID == id && it.MadeWithAI != on {
				if out == nil {
					out = slices.Clone(q)
				}
				out[i].MadeWithAI = on
			}
		}
		if out == nil {
			return q
		}
		changed = true
		return out
	}
	p.s.Queue = set(p.s.Queue)
	if p.unshuffled != nil {
		p.unshuffled = set(p.unshuffled)
	}
	if changed {
		p.emit(QueueChange)
	}
}

// markKnown marks the entries whose label the app knows without asking
// YouTube, as those of a queue kept from the last run.
func (p *Player) markKnown() {
	seen := map[string]bool{}
	for _, it := range p.s.Queue {
		if it.MadeWithAI || seen[it.Track.ID] || !p.checks(it.Track) {
			continue
		}
		seen[it.Track.ID] = true
		if p.knownLabel(it.Track) == LabelAI {
			p.mark(it.Track.ID)
		}
	}
}

// remark marks the entries again, as the way Spotify tracks play changed:
// one that plays on a Spotify device isn't passed over for the label of
// its recording on YouTube Music.
func (p *Player) remark() {
	for _, it := range p.s.Queue {
		if it.MadeWithAI && it.Track.Source == media.SourceSpotify && !streams(it.Track, p.spotifyMode, p.here) {
			p.setMark(it.Track.ID, false)
		}
	}
	p.markKnown()
}

// setSkipAI has the player skip what YouTube labels as made with AI, or no
// longer. The current track plays on either way.
func (p *Player) setSkipAI(on bool) {
	if on == p.s.SkipAI {
		return
	}
	p.s.SkipAI = on
	if on {
		p.markKnown()
	}
	p.emit(QueueChange)
}

// ForgetLabel forgets that YouTube labels the track id as made with AI, as
// when it labels the wrong recording of a Spotify track: its entries play,
// until a check finds the label again.
func (p *Player) ForgetLabel(id string) { p.do(func() { p.setMark(id, false) }) }

// prefetch downloads the audio of next, the entry after cur, to play it at
// once. While the player skips what YouTube labels, next is checked first,
// unless its label is known: labeled, it's marked, and the entry after it
// is prefetched instead, up to labelAhead of them; one whose check failed
// isn't, and is checked again as its turn comes.
func (p *Player) prefetch(cur, next Item) {
	if !p.checks(next.Track) {
		p.preload(next)
		return
	}
	switch p.knownLabel(next.Track) {
	case LabelAI:
		p.mark(next.Track.ID)
		p.prefetched = "" // the entry after it, as the track plays on
		return
	case LabelNone, LabelPaused:
		p.preload(next)
		return
	}
	if p.ai.aheadOf != cur.UID {
		p.ai.aheadOf, p.ai.ahead = cur.UID, 0
	}
	if p.ai.ahead >= labelAhead {
		return
	}
	p.ai.ahead++
	labels, _ := p.labels()
	token := p.loadToken
	await(p, func(ctx context.Context) (Label, error) { return labels.Label(ctx, next.Track, true), nil }, func(l Label, _ error) {
		p.learned(next.Track, l)
		if token != p.loadToken || p.prefetched != next.UID {
			return
		}
		switch l {
		case LabelAI:
			p.prefetched = ""
		case LabelNone, LabelPaused:
			p.preload(next)
		}
	})
}

// mayStart reports whether next may start before its turn, crossfaded
// into: the player doesn't check it, or its check let it.
func (p *Player) mayStart(next Item) bool {
	if !p.checks(next.Track) {
		return true
	}
	l := p.knownLabel(next.Track)
	return l == LabelNone || l == LabelPaused
}

// withoutLabeled leaves out of a radio's tracks those the app knows
// YouTube labels as made with AI, while the player skips them, and tells
// whether it left out any.
func (p *Player) withoutLabeled(tracks []media.Track) ([]media.Track, bool) {
	if !p.skipping() {
		return tracks, false
	}
	kept := slices.DeleteFunc(slices.Clone(tracks), func(t media.Track) bool {
		return p.checks(t) && p.knownLabel(t) == LabelAI
	})
	return kept, len(kept) < len(tracks)
}

// checkRadio checks the labels of the entries a radio added, beside the
// player, after the checks of tracks about to play, and marks those YouTube
// labels as the answers come. It counts the radio among those added since
// a track last played.
func (p *Player) checkRadio(items []Item) {
	if !p.skipping() {
		return
	}
	labels, _ := p.labels()
	p.ai.radios++
	for _, it := range items {
		t := it.Track
		if !p.checks(t) || p.knownLabel(t) != LabelUnknown {
			continue
		}
		await(p, func(ctx context.Context) (Label, error) { return labels.Label(ctx, t, false), nil }, func(l Label, _ error) {
			if l == LabelAI {
				p.mark(t.ID)
			}
		})
	}
}

// checkFallback checks the label of a Spotify track the player moved to,
// which plays from its recording on YouTube Music as no Spotify device
// could, and then plays it, or passes over it, labeled.
func (p *Player) checkFallback(token int, entry Item, play func()) {
	if token != p.ai.movedTo || !p.skipping() || streams(entry.Track, p.spotifyMode, p.here) {
		play() // chosen, or checked before it played
		return
	}
	labels, _ := p.labels()
	await(p, func(ctx context.Context) (Label, error) {
		ctx, cancel := context.WithTimeout(ctx, labelWait)
		defer cancel()
		return labels.Label(ctx, entry.Track, true), nil
	}, func(l Label, _ error) {
		p.learned(entry.Track, l)
		if token != p.loadToken {
			return
		}
		if l != LabelAI {
			play()
			return
		}
		m := &move{dir: 1, ended: true, held: true}
		p.passOver(m, entry)
		p.step(m, p.indexOf(entry.UID)+1)
	})
}
