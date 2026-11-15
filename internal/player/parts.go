package player

// Passing over the parts of videos SponsorBlock marks (#548), as sponsor
// reads, intros and the parts of music videos without music, while the
// settings ask: the app tells the parts of the recording a stream plays
// (Marked), the engine passes over those of the kinds the settings tick
// (audio.SetSkips), and the player marks them for the seek bars, tells of
// each one passed over, with undo, and counts plays and Previous without
// them.
//
// A stream's parts are asked for as it is found: the next track's as it's
// prepared, before it plays. Playback never waits for them: those that
// come late apply from then on.

import (
	"context"
	"slices"
	"time"

	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/sponsorblock"
)

// Marked is what the player asks of the app of the parts of the
// recordings it plays, when its Services have it too: without, it passes
// over no part.
type Marked interface {
	// Marks returns the parts of the recording src plays that SponsorBlock
	// marks, and its highlight, asking SponsorBlock when it must, until ctx
	// ends: none for audio SponsorBlock doesn't know, as a file on this
	// computer.
	Marks(ctx context.Context, src Source) ([]sponsorblock.Segment, error)
	// KnownMarks is Marks without asking: false when it would have to.
	KnownMarks(src Source) ([]sponsorblock.Segment, bool)
}

// Skipper is a Stream that passes over parts of tracks as it plays them,
// as the engine does (audio.Engine.SetSkips): without, as a cast device,
// the parts play.
type Skipper interface {
	SetSkips(url string, skips []audio.Skip)
}

// Part is a part of the current track the player passes over: a kind the
// settings skip, which fits its recording.
type Part struct {
	Kind     sponsorblock.Kind
	From, To time.Duration
}

// partState is what the player keeps of the parts of streams.
type partState struct {
	// known are the parts the app told of streams, by their URL, and
	// asking the streams asked of.
	known  map[string][]sponsorblock.Segment
	asking map[string]bool
	// url is the audio the engine has of the current entry, which it was
	// told the parts of, and src the stream it is of; length is the
	// length the current entry's parts were fitted to.
	url    string
	src    Source
	length time.Duration
	// settings are the settings' skipping of parts.
	settings settings.SkipParts
}

// keptStreams is how many streams' parts the player keeps: the current
// track's and the next ones'.
const keptStreams = 8

// marked returns the app's Marked, if its services have it.
func (p *Player) marked() (Marked, bool) {
	m, ok := p.svc.(Marked)
	return m, ok
}

// asksParts reports whether the player asks for the parts of src: the
// settings skip parts, the app knows them, and src is a stream the engine
// plays, which isn't live.
func (p *Player) asksParts(src Source) bool {
	_, ok := p.marked()
	return ok && p.parts.settings.On && src.Mode == ModeStream && !src.Live && src.URL != ""
}

// lookUp asks the app for the parts of src, unless the player knows them,
// or asks already: they apply as they come. A stream found without parts
// is asked of again, of the app, which knows it a while: as offline, the
// app may have known nothing of it.
func (p *Player) lookUp(src Source) {
	if !p.asksParts(src) {
		return
	}
	st := &p.parts
	if segs := st.known[src.URL]; len(segs) > 0 || st.asking[src.URL] {
		return
	}
	m, _ := p.marked()
	if segs, ok := m.KnownMarks(src); ok {
		p.keepParts(src, segs)
		return
	}
	if st.asking == nil {
		st.asking = map[string]bool{}
	}
	st.asking[src.URL] = true
	await(p, func(ctx context.Context) ([]sponsorblock.Segment, error) { return m.Marks(ctx, src) }, func(segs []sponsorblock.Segment, err error) {
		delete(p.parts.asking, src.URL)
		if err != nil {
			return // asked again as it plays next
		}
		p.keepParts(src, segs)
		// A late answer applies from now on.
		if src.URL == p.parts.src.URL && p.parts.url != "" {
			p.tellParts(p.parts.url, src)
			p.refit()
		}
		p.tellParts(src.URL, src)
		if src.SurroundURL != "" {
			p.tellParts(src.SurroundURL, src)
		}
	})
}

// keepParts keeps the parts of src, forgetting those of other streams once
// many are kept, but the current entry's.
func (p *Player) keepParts(src Source, segs []sponsorblock.Segment) {
	st := &p.parts
	if st.known == nil || len(st.known) >= keptStreams {
		cur, ok := st.known[st.src.URL]
		st.known = map[string][]sponsorblock.Segment{}
		if ok {
			st.known[st.src.URL] = cur
		}
	}
	st.known[src.URL] = segs
}

// skipsOf returns what the engine passes over of src: the parts the
// player knows of it, of the kinds the settings skip.
func (p *Player) skipsOf(src Source) []audio.Skip {
	if !p.asksParts(src) {
		return nil
	}
	var out []audio.Skip
	for _, s := range p.parts.known[src.URL] {
		if s.Kind != sponsorblock.Highlight && p.parts.settings.Skips(s.Kind) {
			out = append(out, audio.Skip{From: s.From, To: s.To, Of: s.Of})
		}
	}
	return out
}

// tellParts tells the engine what it passes over of src as it plays url,
// while the player asks for src's parts.
func (p *Player) tellParts(url string, src Source) {
	if s, ok := p.stream.(Skipper); ok && p.asksParts(src) {
		s.SetSkips(url, p.skipsOf(src))
	}
}

// partsFor tells the engine the parts of src, a stream it is to play at
// url, that the player knows, and asks for them when it doesn't.
func (p *Player) partsFor(url string, src Source) {
	p.lookUp(src)
	p.tellParts(url, src)
}

// currentParts is partsFor of the current entry's stream, whose parts the
// seek bars show.
func (p *Player) currentParts(url string, src Source) {
	p.parts.url, p.parts.src = url, src
	p.partsFor(url, src)
	p.refit()
}

// clearParts forgets the parts of the entry that was current, as another
// becomes current: its own show once its stream is found.
func (p *Player) clearParts() {
	p.parts.url, p.parts.src = "", Source{}
	p.s.Parts, p.s.Highlight = nil, 0
}

// refit finds again the current entry's parts and highlight that the seek
// bars show, of its length.
func (p *Player) refit() {
	parts, highlight := p.fitted()
	if !slices.Equal(parts, p.s.Parts) || highlight != p.s.Highlight {
		p.s.Parts, p.s.Highlight = parts, highlight
		p.emit(StateChange)
	}
}

// refitLength refits the current entry's parts once its length changed,
// as its stream told it.
func (p *Player) refitLength() {
	if p.parts.length != p.s.Duration {
		p.parts.length = p.s.Duration
		p.refit()
	}
}

// fitted returns the current entry's parts that the player passes over,
// of the kinds the settings skip, and its highlight, while they show it:
// those that fit its length, as the engine has them.
func (p *Player) fitted() ([]Part, time.Duration) {
	src, cur := p.parts.src, p.s.Source
	if _, skips := p.stream.(Skipper); !skips || cur == nil || cur.URL != src.URL || !p.asksParts(src) {
		return nil, 0 // a cast device plays them
	}
	length := p.s.Duration
	var parts []Part
	var highlight time.Duration
	for _, s := range p.parts.known[src.URL] {
		sk := audio.Skip{From: s.From, To: s.To, Of: s.Of}
		switch {
		case s.Kind == sponsorblock.Highlight:
			if p.parts.settings.Highlight && highlight == 0 && sk.Fits(length) && (length == 0 || s.From < length) {
				highlight = s.From
			}
		case p.parts.settings.Skips(s.Kind):
			// What the engine leaves out, it plays.
			if fit := audio.Spans([]audio.Skip{sk}, length); len(fit) == 1 {
				parts = append(parts, Part{Kind: s.Kind, From: fit[0].From, To: fit[0].To})
			}
		}
	}
	return parts, highlight
}

// spans are the stretches of the current entry the engine passes over.
func (p *Player) spans() []audio.Skip {
	skips := make([]audio.Skip, len(p.s.Parts))
	for i, s := range p.s.Parts {
		skips[i] = audio.Skip{From: s.From, To: s.To}
	}
	return audio.Spans(skips, p.s.Duration)
}

// withoutParts returns what the current entry played at pos, of a length
// of dur, and its length, leaving out the parts the player passes over:
// they count for nothing in plays, nor in Previous.
func (p *Player) withoutParts(pos, dur time.Duration) (time.Duration, time.Duration) {
	for _, s := range p.spans() {
		if pos > s.From {
			pos -= min(pos, s.To) - s.From
		}
		if dur > 0 {
			dur -= s.To - s.From
		}
	}
	return pos, max(dur, 0)
}

// skippedLength is how long the parts of the current entry the player
// passes over last.
func (p *Player) skippedLength() time.Duration {
	var d time.Duration
	for _, s := range p.spans() {
		d += s.To - s.From
	}
	return d
}

// endOf is where the current entry, of a length of dur, ends: where the
// part that reaches its end starts, if any, where the crossfade takes it
// to end.
func (p *Player) endOf(dur time.Duration) time.Duration {
	if s := p.spans(); len(s) > 0 && s[len(s)-1].Ends(dur) {
		return s[len(s)-1].From
	}
	return dur
}

// partSettings takes in the settings' skipping of parts: the engine hears
// again what it passes over of the current track's stream, and the seek
// bars show what it does. Turned on, the current track's parts are asked
// for.
func (p *Player) partSettings(s settings.SkipParts) {
	st := &p.parts
	if s.On == st.settings.On && s.Highlight == st.settings.Highlight && s.Server == st.settings.Server && slices.Equal(s.Kinds, st.settings.Kinds) {
		return
	}
	if s.Server != st.settings.Server {
		st.known = nil // another server's
	}
	s.Kinds = slices.Clone(s.Kinds)
	st.settings = s
	if sk, ok := p.stream.(Skipper); ok && !s.On && st.url != "" {
		// Off, the engine passes over nothing more.
		sk.SetSkips(st.url, nil)
	}
	if st.url != "" {
		p.partsFor(st.url, st.src)
	}
	p.refit()
}

// Skipped takes in the engine passing over a part of the track of url, as
// audio.SkipEvents.
func (p *Player) Skipped(url string, from, to time.Duration) {
	p.do(func() { p.onSkipped(url, from, to) })
}

// onSkipped tells the user of a part the engine passed over of the current
// entry, with undo, which goes back to it: it plays then, as each part is
// passed over once a play. A part that ended the track has no undo, and
// one a track loaded paused starts past, as at start, goes untold.
func (p *Player) onSkipped(url string, from, to time.Duration) {
	cur, ok := p.s.Current()
	if !ok || url != p.parts.url || p.parts.url == "" || p.paused {
		return
	}
	n := Notice{Message: "Skipped " + p.partWords(from, to)}
	if !(audio.Skip{From: from, To: to}).Ends(p.s.Duration) {
		uid := cur.UID
		n.Undo = func() { p.UndoSkip(uid, from) }
	}
	p.notify(n)
}

// partWords names what the parts of the current entry from from to to
// are: "the intro", "the intro and a sponsor read", or how many.
func (p *Player) partWords(from, to time.Duration) string {
	var kinds []sponsorblock.Kind
	for _, s := range p.s.Parts {
		if s.From < to && s.To > from && !slices.Contains(kinds, s.Kind) {
			kinds = append(kinds, s.Kind)
		}
	}
	switch len(kinds) {
	case 0:
		return "a part"
	case 1:
		return kinds[0].Part()
	case 2:
		return kinds[0].Part() + " and " + kinds[1].Part()
	}
	return "parts of " + inWords(len(kinds)) + " kinds"
}

// inWords is n in words, for a notice.
func inWords(n int) string {
	if n < 10 {
		return [...]string{"no", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine"}[n]
	}
	return "many"
}

// UndoSkip goes back to at of the entry uid, where a part it passed over
// starts, while it is current: the part plays.
func (p *Player) UndoSkip(uid string, at time.Duration) {
	p.do(func() {
		if cur, ok := p.s.Current(); ok && cur.UID == uid && !p.s.Loading {
			p.seek(at)
		}
	})
}

// JumpToHighlight seeks to the current track's highlight, if it has one.
func (p *Player) JumpToHighlight() {
	p.do(func() {
		if p.s.Highlight > 0 && !p.s.Loading {
			p.seek(p.s.Highlight)
		}
	})
}

var _ audio.SkipEvents = (*Player)(nil)
