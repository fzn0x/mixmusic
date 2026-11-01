package app

// Whether YouTube labels the videos the player plays as made with AI, for
// the player to skip them as the settings ask. YouTube's website tells it
// of each video, which is asked once, signed out.

import (
	"context"
	"errors"
	"log"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/store"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

// These are the label cache's times and bounds; tests shorten some.
var (
	// labeledFor is how long a video found labeled stays known, and
	// cleanFor one found not labeled, in memory alone.
	labeledFor = 30 * 24 * time.Hour
	cleanFor   = 24 * time.Hour
	// labelAsk bounds a lookup, which the callers waiting for it share.
	labelAsk = 15 * time.Second
	// labelPause is how long lookups stop after labelFails of them failed
	// in a row, so that YouTube not answering doesn't hold every track.
	labelPause = 10 * time.Minute
	// labelSaveAfter gathers the videos found labeled in a while into one
	// write of the file.
	labelSaveAfter = 5 * time.Second
)

const (
	labelFails = 3
	// labelSlots is how many lookups run at once, of which laterSlots at
	// most of tracks not about to play, as those of a radio: the track
	// about to play never waits behind them.
	labelSlots = 4
	laterSlots = 2
	// labelsKept is how many videos found labeled the file keeps at most,
	// the latest.
	labelsKept = 5000
)

// labelCache keeps whether YouTube labels videos as made with AI: those
// labeled in ai-labels.json in the cache folder, by when they were found
// so, the others in memory. It asks once per video, however many ask at
// once. Its methods are safe for concurrent use, and a nil cache knows
// nothing and asks nothing.
type labelCache struct {
	lookup func(ctx context.Context, videoID string) (bool, error)
	file   *store.File[map[string]int64]
	now    func() time.Time
	slots  chan struct{}
	later  chan struct{}

	mu      sync.Mutex
	labeled map[string]time.Time
	clean   map[string]time.Time
	calls   map[string]*labelCall
	fails   int
	paused  time.Time // lookups stop until then
	saving  *time.Timer
	dirty   bool
	// writing orders the writes of the file.
	writing sync.Mutex
}

// labelCall is a lookup under way, which the callers asking meanwhile
// share. soon is closed once one of them waits for a track about to play.
type labelCall struct {
	done  chan struct{}
	soon  chan struct{}
	hurry sync.Once
	label player.Label
}

func newLabelCache(lookup func(context.Context, string) (bool, error), file *store.File[map[string]int64], now func() time.Time) *labelCache {
	c := &labelCache{
		lookup:  lookup,
		file:    file,
		now:     now,
		slots:   make(chan struct{}, labelSlots),
		later:   make(chan struct{}, laterSlots),
		labeled: map[string]time.Time{},
		clean:   map[string]time.Time{},
		calls:   map[string]*labelCall{},
	}
	for id, at := range file.Get() {
		if found := time.Unix(at, 0); now().Sub(found) < labeledFor {
			c.labeled[id] = found
		}
	}
	return c
}

// Known tells what the cache knows of a video's label, without asking:
// LabelUnknown when it would have to ask, and LabelPaused while lookups
// stop.
func (c *labelCache) Known(videoID string) player.Label {
	if c == nil {
		return player.LabelUnknown
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.knownLocked(videoID)
}

func (c *labelCache) knownLocked(videoID string) player.Label {
	now := c.now()
	if at, ok := c.labeled[videoID]; ok && now.Sub(at) < labeledFor {
		return player.LabelAI
	}
	if at, ok := c.clean[videoID]; ok && now.Sub(at) < cleanFor {
		return player.LabelNone
	}
	if now.Before(c.paused) {
		return player.LabelPaused
	}
	return player.LabelUnknown
}

// Get tells whether YouTube labels a video as made with AI, asking it when
// the cache doesn't know, until ctx ends: LabelUnknown then, or when
// YouTube couldn't tell. A lookup of a track about to play, soon, goes
// before the others.
func (c *labelCache) Get(ctx context.Context, videoID string, soon bool) player.Label {
	if c == nil {
		return player.LabelUnknown
	}
	c.mu.Lock()
	if l := c.knownLocked(videoID); l != player.LabelUnknown {
		c.mu.Unlock()
		return l
	}
	call := c.calls[videoID]
	if call == nil {
		call = &labelCall{done: make(chan struct{}), soon: make(chan struct{})}
		if soon {
			call.hurry.Do(func() { close(call.soon) })
		}
		c.calls[videoID] = call
		go c.run(videoID, call)
	}
	c.mu.Unlock()
	if soon {
		call.hurry.Do(func() { close(call.soon) })
	}
	select {
	case <-call.done:
		return call.label
	case <-ctx.Done():
		return player.LabelUnknown
	}
}

// run looks up the label of a video, once there is a slot for it, on a
// context of its own: the callers may stop waiting, and the answer is kept
// for the next.
func (c *labelCache) run(videoID string, call *labelCall) {
	defer close(call.done)
	select {
	case c.later <- struct{}{}:
		defer func() { <-c.later }()
	case <-call.soon:
	}
	c.slots <- struct{}{}
	defer func() { <-c.slots }()
	ctx, cancel := context.WithTimeout(context.Background(), labelAsk)
	defer cancel()
	ai, err := c.lookup(ctx, videoID)

	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.calls, videoID)
	now := c.now()
	switch {
	case err == nil && ai:
		c.fails = 0
		c.labeled[videoID] = now
		c.dirty = true
		if c.saving == nil {
			c.saving = time.AfterFunc(labelSaveAfter, c.save)
		}
		call.label = player.LabelAI
	case err == nil:
		c.fails = 0
		c.keepClean(videoID, now)
		call.label = player.LabelNone
	case errors.Is(err, youtube.ErrNotFound):
		// Removed or private: it plays, unchecked.
		c.fails = 0
		call.label = player.LabelUnknown
	default:
		call.label = player.LabelUnknown
		if c.fails++; c.fails >= labelFails && !now.Before(c.paused) {
			c.fails = 0
			c.paused = now.Add(labelPause)
			log.Printf("mixmusic: %d checks of AI labels in a row failed, the last with %v: tracks play unchecked for %s", labelFails, err, labelPause)
		}
	}
}

// keepClean keeps that a video isn't labeled, dropping what expired once
// many are kept.
func (c *labelCache) keepClean(videoID string, now time.Time) {
	if len(c.clean) >= labelsKept {
		maps.DeleteFunc(c.clean, func(_ string, at time.Time) bool { return now.Sub(at) >= cleanFor })
	}
	c.clean[videoID] = now
}

// save writes the videos found labeled to the file, the labelsKept latest
// of them.
func (c *labelCache) save() {
	if c == nil {
		return
	}
	c.writing.Lock()
	defer c.writing.Unlock()
	c.mu.Lock()
	if c.saving != nil {
		c.saving.Stop()
		c.saving = nil
	}
	if !c.dirty {
		c.mu.Unlock()
		return
	}
	c.dirty = false
	now := c.now()
	ids := make([]string, 0, len(c.labeled))
	for id, at := range c.labeled {
		if now.Sub(at) < labeledFor {
			ids = append(ids, id)
		}
	}
	slices.SortFunc(ids, func(a, b string) int { return c.labeled[b].Compare(c.labeled[a]) })
	kept := make(map[string]int64, min(len(ids), labelsKept))
	for _, id := range ids[:min(len(ids), labelsKept)] {
		kept[id] = c.labeled[id].Unix()
	}
	c.mu.Unlock()
	if err := c.file.Set(kept); err != nil {
		log.Printf("mixmusic: keeping the AI labels: %v", err)
	}
}

// recording returns the video of the recording t plays from on YouTube
// Music, finding it when ask: a Spotify track's match.
func (a *App) recording(ctx context.Context, t media.Track, ask bool) (string, bool) {
	yt, ok := a.matcher.Matched(t)
	if !ok && ask {
		var err error
		if yt, err = a.matcher.Match(ctx, t); err != nil {
			return "", false
		}
		ok = true
	}
	if !ok {
		return "", false
	}
	_, videoID, err := media.ParseID(yt.ID)
	return videoID, err == nil && yt.Source == media.SourceYouTube
}

// Label tells whether YouTube labels the recording of t on YouTube Music
// as made with AI, as the player asks, finding the recording of a Spotify
// track first. While the network is gone it doesn't ask: the track can't
// play anyway, and the lookups don't count as failed.
func (s playerServices) Label(ctx context.Context, t media.Track, soon bool) player.Label {
	if s.a.online != nil && s.a.online.Offline() {
		return s.a.labels.Known(s.knownVideo(t))
	}
	videoID, ok := s.a.recording(ctx, t, true)
	if !ok {
		return player.LabelUnknown
	}
	return s.a.labels.Get(ctx, videoID, soon)
}

// KnownLabel is Label without asking YouTube, nor searching for a match.
func (s playerServices) KnownLabel(t media.Track) player.Label {
	return s.a.labels.Known(s.knownVideo(t))
}

func (s playerServices) knownVideo(t media.Track) string {
	videoID, _ := s.a.recording(context.Background(), t, false)
	return videoID
}

var _ player.Labels = playerServices{}
