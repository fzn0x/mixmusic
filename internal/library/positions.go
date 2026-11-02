package library

import (
	"log"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/store"
)

// The rules of where an episode is left.
const (
	// maxPositions is how many episodes' places are kept, the latest.
	maxPositions = 500
	// keepFrom is how far into an episode its place is kept: before, it
	// starts from its start again.
	keepFrom = 15 * time.Second
	// playedWithin is how near its end an episode counts as played, at
	// most, and playedShare the share of a short one.
	playedWithin = 2 * time.Minute
	playedShare  = 0.03
	// rewind is how far before where it was left an episode resumes.
	rewind = 5 * time.Second
)

// writeAfter is how long the places wait to be written after a change,
// so that an episode that plays writes them every few seconds, not each:
// tests shorten it.
var writeAfter = 10 * time.Second

// Position is where the user left a podcast's episode.
type Position struct {
	// Track is the episode, as it last played.
	Track      media.Track `json:"track"`
	PositionMs int         `json:"positionMs"`
	DurationMs int         `json:"durationMs,omitempty"`
	// Played is set once the episode played to its end, or near it.
	Played bool `json:"played,omitempty"`
	// Updated is when the place last changed.
	Updated time.Time `json:"updated"`
}

// Started reports whether the episode was left partway through.
func (p Position) Started() bool { return !p.Played && p.PositionMs > 0 }

// Left is how much of the episode is left to hear, or 0 when its length
// isn't known.
func (p Position) Left() time.Duration {
	if p.DurationMs <= p.PositionMs {
		return 0
	}
	return time.Duration(p.DurationMs-p.PositionMs) * time.Millisecond
}

// Positions keeps where the user left podcasts' episodes, in a file of
// their own: while an episode plays its place changes each second, and the
// file is written once it rested writeAfter, and as MixMusic quits. Its
// methods are safe for concurrent use.
type Positions struct {
	file *store.File[map[string]Position]
	now  func() time.Time
	// writing orders the writes: one that took the places earlier never
	// lands over a later one.
	writing sync.Mutex

	mu      sync.Mutex
	held    map[string]Position
	dirty   bool
	timer   *time.Timer
	version uint64
	// recent is held, the latest first, as of sortedAt's version.
	recent   []Position
	sortedAt uint64
}

// OpenPositions opens the places kept at path.
func OpenPositions(path string) (*Positions, error) {
	f, err := store.Open(path, map[string]Position{})
	if err != nil {
		return nil, err
	}
	held := maps.Clone(f.Get())
	if held == nil {
		held = map[string]Position{}
	}
	return &Positions{file: f, now: time.Now, held: held, sortedAt: ^uint64(0)}, nil
}

// Keep keeps where an episode is: at, of its length dur, or of the
// track's own when dur is 0. Under keepFrom, it forgets the place, but
// that the episode was played; near its end, it marks it played. It keeps
// nothing of a track that is no episode.
func (p *Positions) Keep(t media.Track, at, dur time.Duration) {
	if !t.IsEpisode() {
		return
	}
	if dur <= 0 {
		dur = time.Duration(t.DurationMs) * time.Millisecond
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	was, had := p.held[t.ID]
	next := Position{Track: t, PositionMs: int(at / time.Millisecond), DurationMs: int(dur / time.Millisecond), Updated: p.now()}
	switch {
	case dur > 0 && at >= dur-min(playedWithin, time.Duration(playedShare*float64(dur))):
		next.Played, next.PositionMs = true, 0
		if had && was.Played {
			return
		}
	case at < keepFrom:
		if !had || was.Played {
			return
		}
		delete(p.held, t.ID)
		p.changed()
		return
	case had && !was.Played && abs(was.PositionMs-next.PositionMs) < 1000:
		return
	}
	p.held[t.ID] = next
	if !had {
		p.trim()
	}
	p.changed()
}

// SetPlayed marks an episode played, which plays from its start again, or
// forgets that it was, and where it was left.
func (p *Positions) SetPlayed(t media.Track, played bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !played {
		if _, ok := p.held[t.ID]; ok {
			delete(p.held, t.ID)
			p.changed()
		}
		return
	}
	p.held[t.ID] = Position{Track: t, DurationMs: t.DurationMs, Played: true, Updated: p.now()}
	p.trim()
	p.changed()
}

// Get returns where the user left the episode id.
func (p *Positions) Get(id string) (Position, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	pos, ok := p.held[id]
	return pos, ok
}

// ResumeAt returns where an episode plays from as it starts: a little
// before where the user left it, or where its service says they left it
// when MixMusic has no place of its own; 0 for one played, and for a
// track that is no episode.
func (p *Positions) ResumeAt(t media.Track) time.Duration {
	if !t.IsEpisode() {
		return 0
	}
	at := time.Duration(0)
	if pos, ok := p.Get(t.ID); ok {
		if pos.Started() {
			at = time.Duration(pos.PositionMs) * time.Millisecond
		}
	} else if e := t.Episode; !e.Played && time.Duration(e.ResumeMs)*time.Millisecond >= keepFrom {
		at = time.Duration(e.ResumeMs) * time.Millisecond
	}
	return max(at-rewind, 0)
}

// InProgress returns the episodes left partway, the latest first, at most
// limit of them.
func (p *Positions) InProgress(limit int) []Position {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.sortedAt != p.version {
		p.recent = p.recent[:0]
		for _, pos := range p.held {
			if pos.Started() {
				p.recent = append(p.recent, pos)
			}
		}
		slices.SortFunc(p.recent, func(a, b Position) int { return b.Updated.Compare(a.Updated) })
		p.sortedAt = p.version
	}
	n := len(p.recent)
	if limit > 0 {
		n = min(n, limit)
	}
	return slices.Clone(p.recent[:n])
}

// Version counts the changes of the places: what was made of them holds
// until it changes.
func (p *Positions) Version() uint64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.version
}

// Flush writes the places at once, if they changed since they were last
// written.
func (p *Positions) Flush() error {
	p.writing.Lock()
	defer p.writing.Unlock()
	p.mu.Lock()
	if p.timer != nil {
		p.timer.Stop()
		p.timer = nil
	}
	if !p.dirty {
		p.mu.Unlock()
		return nil
	}
	p.dirty = false
	held := maps.Clone(p.held)
	p.mu.Unlock()
	return p.file.Update(func(v *map[string]Position) error {
		*v = held
		return nil
	})
}

// changed notes a change, and has the places written in writeAfter, if no
// write waits yet. p.mu is held.
func (p *Positions) changed() {
	p.version++
	p.dirty = true
	if p.timer == nil {
		p.timer = time.AfterFunc(writeAfter, func() {
			if err := p.Flush(); err != nil {
				log.Printf("mixmusic: keeping where episodes were left: %v", err)
			}
		})
	}
}

// trim drops the places of the oldest episodes past maxPositions. p.mu is
// held.
func (p *Positions) trim() {
	for len(p.held) > maxPositions {
		oldest := ""
		var at time.Time
		for id, pos := range p.held {
			if oldest == "" || pos.Updated.Before(at) {
				oldest, at = id, pos.Updated
			}
		}
		delete(p.held, oldest)
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
