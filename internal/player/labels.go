package player

// Passing over the tracks YouTube labels as made with AI, as the settings
// may ask, on the queue's own path of passing over entries (skip.go).
//
// A track whose label the app doesn't know is checked before it plays,
// while its stream is found: what played stops meanwhile, and nothing is
// told of the track, to the app or the subscribers, until its check lets
// it play. A check that takes long lets it play unchecked. The next track
// is checked before it's preloaded or crossfaded into.

import (
	"context"
	"errors"
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
// Services have them too: without, the player passes over no track for its
// label.
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

// labelAhead is how many entries after the current one the player checks,
// as it finds them labeled, before the next one plays.
const labelAhead = 5

// pausedChecks tells that the checks of labels pause.
const pausedChecks = "YouTube isn't answering MixMusic's checks for AI labels, so tracks play unchecked for now"

// errOnlyLabeled is why a radio added nothing: all it held that wasn't
// queued is labeled as made with AI.
var errOnlyLabeled = errors.New("the radio held only songs labeled as made with AI")

// labelGate is the check of an entry a move reached, while what played
// stops.
type labelGate struct {
	token int
	uid   string
	m     *move
}

// labels returns the app's Labels, if its services have them.
func (p *Player) labels() (Labels, bool) {
	l, ok := p.svc.(Labels)
	return l, ok
}

// skipsAI reports whether the player passes over what YouTube labels as
// made with AI: the settings ask it, and the app knows the labels.
func (p *Player) skipsAI() bool {
	_, ok := p.labels()
	return ok && p.skipAI
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
	return p.skipsAI() && streams(t, p.spotifyMode, p.here)
}

// knownLabel is the label the app knows of t, without asking YouTube.
func (p *Player) knownLabel(t media.Track) Label {
	l, ok := p.labels()
	if !ok {
		return LabelNone
	}
	return l.KnownLabel(t)
}

// labelSkip is SkipMadeWithAI for a track the app knows YouTube labels as
// made with AI, while the player passes over those, and "" otherwise.
func (p *Player) labelSkip(t media.Track) string {
	if p.checks(t) && p.knownLabel(t) == LabelAI {
		return SkipMadeWithAI
	}
	return ""
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
	p.passing.gate = &labelGate{token: token, uid: entry.UID, m: m}
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
		p.passing.gate = nil
		at := p.indexOf(entry.UID)
		switch {
		case at < 0:
			// Taken out of the queue meanwhile: the move goes on from its
			// place.
			if m.dir < 0 {
				i--
			}
			p.step(m, i)
		case p.s.Queue[at].Skip != "":
			// Labeled, or disliked meanwhile.
			p.passOver(m, entry, p.s.Queue[at].Skip)
			p.step(m, at+m.dir)
		default:
			p.playAt(at, 0, !m.paused)
			p.passing.movedTo = p.loadToken
		}
	})
}

// learned takes in the label a check told of t: the entries of a track
// labeled are marked, and a pause of the checks is told once.
func (p *Player) learned(t media.Track, l Label) {
	switch l {
	case LabelAI:
		p.mark(t.ID)
		p.passing.toldPaused = false
	case LabelNone:
		p.passing.toldPaused = false
	case LabelPaused:
		p.tellPaused()
	}
}

// tellPaused tells, once, that tracks play unchecked for now.
func (p *Player) tellPaused() {
	if !p.passing.toldPaused {
		p.passing.toldPaused = true
		p.notify(Notice{Message: pausedChecks, Settings: true})
	}
}

// mark has the queue pass over the entries of the track id, which YouTube
// labels as made with AI, while the settings skip those: but those the
// user chose, those it passes over already, and the current one, which
// plays on. It changes the queue and its own order, both cloned: what is
// published shares them.
func (p *Player) mark(id string) {
	if !p.skipAI {
		return
	}
	cur, _ := p.s.Current()
	p.setSkips(id, func(it Item) string {
		if it.Chosen || it.Skip != "" || it.UID == cur.UID || !p.checks(it.Track) {
			return it.Skip
		}
		return SkipMadeWithAI
	})
}

// ForgetLabel forgets that YouTube labels the track id as made with AI, as
// when it labels the wrong recording of a Spotify track: its entries play,
// until a check finds the label again.
func (p *Player) ForgetLabel(id string) {
	p.do(func() {
		p.setSkips(id, func(it Item) string {
			if it.Skip == SkipMadeWithAI {
				return ""
			}
			return it.Skip
		})
	})
}

// setSkips sets why the queue passes over the entries of the track id, as
// skip tells, in the queue and in its own order.
func (p *Player) setSkips(id string, skip func(Item) string) {
	changed := false
	set := func(q []Item) []Item {
		var out []Item
		for i, it := range q {
			if it.Track.ID != id {
				continue
			}
			if why := skip(it); why != it.Skip {
				if out == nil {
					out = slices.Clone(q)
				}
				out[i].Skip = why
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

// prefetch downloads the audio of next, the entry after cur, to play it at
// once. While the player skips what YouTube labels, next is checked first,
// unless its label is known: labeled, it's marked, and the entry after it
// is prefetched instead, up to labelAhead of them; one whose check failed
// isn't, and is checked again as its turn comes.
func (p *Player) prefetch(cur, next Item) {
	if next.Chosen || !p.checks(next.Track) {
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
	if p.passing.aheadOf != cur.UID {
		p.passing.aheadOf, p.passing.ahead = cur.UID, 0
	}
	if p.passing.ahead >= labelAhead {
		return
	}
	p.passing.ahead++
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
	if next.Chosen || !p.checks(next.Track) {
		return true
	}
	l := p.knownLabel(next.Track)
	return l == LabelNone || l == LabelPaused
}

// withoutLabeled leaves out of a radio's tracks those the app knows
// YouTube labels as made with AI, while the player skips them, and tells
// whether it left out any.
func (p *Player) withoutLabeled(tracks []media.Track) ([]media.Track, bool) {
	if !p.skipsAI() {
		return tracks, false
	}
	kept := slices.DeleteFunc(slices.Clone(tracks), func(t media.Track) bool { return p.labelSkip(t) != "" })
	return kept, len(kept) < len(tracks)
}

// checkRadio checks the labels of the entries a radio added, beside the
// player, after the checks of tracks about to play, and marks those YouTube
// labels as the answers come.
func (p *Player) checkRadio(items []Item) {
	if !p.skipsAI() {
		return
	}
	labels, _ := p.labels()
	for _, it := range items {
		t := it.Track
		if it.Skip != "" || !p.checks(t) || p.knownLabel(t) != LabelUnknown {
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
	if token != p.passing.movedTo || !p.skipsAI() || entry.Chosen || streams(entry.Track, p.spotifyMode, p.here) {
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
		p.passOver(m, entry, SkipMadeWithAI)
		p.step(m, p.indexOf(entry.UID)+1)
	})
}
