package app

// The parts of videos SponsorBlock marks, for the player to pass over as
// the settings ask (#548): asked once a video, by a hash prefix of its ID,
// and kept as the labels of AI are (labels.go). The videos found with
// parts stay in sponsorblock.json in the cache folder, the latest
// partsKept of them, so that downloads skip theirs offline too; those
// found without, in memory. A lookup that failed isn't kept.

import (
	"cmp"
	"context"
	"log"
	"maps"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/sponsorblock"
	"github.com/fzn0x/mixmusic/internal/store"
	"github.com/fzn0x/mixmusic/internal/stream"
)

// These are the parts cache's times; tests shorten some.
var (
	// partsFor is how long the parts found of a video are taken as they
	// are, and noPartsFor that it has none: then it's asked again, while
	// the network answers. Offline, the parts kept play however old.
	partsFor   = 7 * 24 * time.Hour
	noPartsFor = 24 * time.Hour
	// partsSaveAfter gathers the videos found with parts in a while into
	// one write of the file.
	partsSaveAfter = 5 * time.Second
)

// partsKept is how many videos found with parts the file keeps at most:
// those downloaded first, then the latest.
const partsKept = 5000

// keptParts is a video's parts as the file keeps them: when they were
// found, in Unix seconds, and the parts, in milliseconds.
type keptParts struct {
	At    int64      `json:"at"`
	Parts []keptPart `json:"parts"`
}

type keptPart struct {
	Kind sponsorblock.Kind `json:"kind"`
	From int64             `json:"from"`
	To   int64             `json:"to"`
	Of   int64             `json:"of,omitempty"`
}

// partCache keeps the parts SponsorBlock marks of videos. It asks once per
// video, however many ask at once. Its methods are safe for concurrent
// use, and a nil cache knows nothing and asks nothing.
type partCache struct {
	lookup func(ctx context.Context, server, videoID string) ([]sponsorblock.Segment, error)
	file   *store.File[map[string]keptParts]
	now    func() time.Time
	// keep reports whether a video is downloaded, whose parts the file
	// keeps before others.
	keep func(videoID string) bool

	mu     sync.Mutex
	found  map[string]keptParts
	none   map[string]time.Time
	calls  map[string]*partCall
	saving *time.Timer
	dirty  bool
	// writing orders the writes of the file.
	writing sync.Mutex
}

// partCall is a lookup under way, which the callers asking meanwhile
// share.
type partCall struct {
	done  chan struct{}
	parts []sponsorblock.Segment
	err   error
}

// newSponsorBlock returns the cache of the parts of videos, which asks
// SponsorBlock introducing itself as userAgent, kept in file, with the
// parts of the videos downloaded first.
func (a *App) newSponsorBlock(userAgent string, file *store.File[map[string]keptParts]) *partCache {
	c := newPartCache(sponsorblock.New(userAgent).Segments, file, time.Now)
	c.keep = func(videoID string) bool { return a.downloads.Snapshot().HasVideo(videoID) }
	return c
}

func newPartCache(lookup func(context.Context, string, string) ([]sponsorblock.Segment, error), file *store.File[map[string]keptParts], now func() time.Time) *partCache {
	found := maps.Clone(file.Get())
	if found == nil {
		found = map[string]keptParts{}
	}
	return &partCache{lookup: lookup, file: file, now: now, found: found, none: map[string]time.Time{}, calls: map[string]*partCall{}}
}

// Known returns the parts of a video the cache knows, and whether it does,
// without asking: those found a while ago, it doesn't, but stale tells
// them all the same, as offline, when none can be asked for.
func (c *partCache) Known(videoID string, stale bool) ([]sponsorblock.Segment, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.knownLocked(videoID, stale)
}

func (c *partCache) knownLocked(videoID string, stale bool) ([]sponsorblock.Segment, bool) {
	now := c.now()
	if k, ok := c.found[videoID]; ok && (stale || now.Sub(time.Unix(k.At, 0)) < partsFor) {
		return segmentsOf(k), true
	}
	if at, ok := c.none[videoID]; ok && now.Sub(at) < noPartsFor {
		return nil, true
	}
	return nil, false
}

// Get returns the parts SponsorBlock marks of a video, asking server when
// the cache doesn't know them, until ctx ends. A lookup that fails tells
// the parts found before, if any, else its error.
func (c *partCache) Get(ctx context.Context, server, videoID string) ([]sponsorblock.Segment, error) {
	if c == nil {
		return nil, nil
	}
	c.mu.Lock()
	if parts, ok := c.knownLocked(videoID, false); ok {
		c.mu.Unlock()
		return parts, nil
	}
	call := c.calls[videoID]
	if call == nil {
		call = &partCall{done: make(chan struct{})}
		c.calls[videoID] = call
		go c.run(server, videoID, call)
	}
	c.mu.Unlock()
	select {
	case <-call.done:
		return call.parts, call.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// run looks up the parts of a video on a context of its own: the callers
// may stop waiting, and the answer is kept for the next.
func (c *partCache) run(server, videoID string, call *partCall) {
	defer close(call.done)
	parts, err := c.lookup(context.Background(), server, videoID)
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.calls, videoID)
	now := c.now()
	switch {
	case err != nil:
		log.Printf("mixmusic: asking SponsorBlock for the parts of %s: %v", videoID, err)
		if k, ok := c.found[videoID]; ok {
			call.parts = segmentsOf(k)
			return
		}
		call.err = err
	case len(parts) == 0:
		delete(c.found, videoID)
		if len(c.none) >= partsKept {
			maps.DeleteFunc(c.none, func(_ string, at time.Time) bool { return now.Sub(at) >= noPartsFor })
		}
		c.none[videoID] = now
		c.later()
	default:
		call.parts = parts
		c.found[videoID] = keptOf(parts, now)
		c.later()
	}
}

// later saves the file a moment later, with what came meanwhile. Its caller
// holds mu.
func (c *partCache) later() {
	c.dirty = true
	if c.saving == nil {
		c.saving = time.AfterFunc(partsSaveAfter, c.save)
	}
}

// save writes the videos found with parts to the file: partsKept of them
// at most, those downloaded first, then the latest.
func (c *partCache) save() {
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
	ids := slices.Collect(maps.Keys(c.found))
	kept := func(id string) bool { return c.keep != nil && c.keep(id) }
	slices.SortFunc(ids, func(a, b string) int {
		if ka, kb := kept(a), kept(b); ka != kb {
			if ka {
				return -1
			}
			return 1
		}
		return cmp.Compare(c.found[b].At, c.found[a].At)
	})
	out := make(map[string]keptParts, min(len(ids), partsKept))
	for _, id := range ids[:min(len(ids), partsKept)] {
		out[id] = c.found[id]
	}
	c.mu.Unlock()
	if err := c.file.Set(out); err != nil {
		log.Printf("mixmusic: keeping the parts SponsorBlock marks: %v", err)
	}
}

func keptOf(parts []sponsorblock.Segment, at time.Time) keptParts {
	k := keptParts{At: at.Unix(), Parts: make([]keptPart, len(parts))}
	for i, p := range parts {
		k.Parts[i] = keptPart{Kind: p.Kind, From: p.From.Milliseconds(), To: p.To.Milliseconds(), Of: p.Of.Milliseconds()}
	}
	return k
}

func segmentsOf(k keptParts) []sponsorblock.Segment {
	out := make([]sponsorblock.Segment, len(k.Parts))
	for i, p := range k.Parts {
		out[i] = sponsorblock.Segment{Kind: p.Kind, From: time.Duration(p.From) * time.Millisecond,
			To: time.Duration(p.To) * time.Millisecond, Of: time.Duration(p.Of) * time.Millisecond}
	}
	return out
}

// streamVideo returns the video whose audio src plays, the address of a
// stream or a download the proxy serves: none for a file on this computer
// or a live stream.
func streamVideo(src player.Source) (string, bool) {
	if src.Mode != player.ModeStream || src.Live {
		return "", false
	}
	u, err := url.Parse(src.URL)
	if err != nil {
		return "", false
	}
	videoID, ok := strings.CutPrefix(u.Path, stream.Path(""))
	videoID = strings.TrimSuffix(videoID, stream.SurroundSuffix)
	return videoID, ok && stream.ValidVideoID(videoID)
}

var _ player.Marked = playerServices{}

// Marks returns the parts SponsorBlock marks of the recording src plays,
// as the player asks, while the settings skip them: a Spotify track's
// recording's too, never a file on this computer's. Offline, it doesn't
// ask: what is kept plays.
func (s playerServices) Marks(ctx context.Context, src player.Source) ([]sponsorblock.Segment, error) {
	set := s.a.settings.Get().Clean().SkipParts
	videoID, ok := streamVideo(src)
	if !set.On || !ok {
		return nil, nil
	}
	if s.a.online != nil && s.a.online.Offline() {
		parts, _ := s.a.parts.Known(videoID, true)
		return parts, nil
	}
	return s.a.parts.Get(ctx, set.Server, videoID)
}

// KnownMarks is Marks without asking SponsorBlock.
func (s playerServices) KnownMarks(src player.Source) ([]sponsorblock.Segment, bool) {
	videoID, ok := streamVideo(src)
	if !ok {
		return nil, true // there are none to ask for
	}
	offline := s.a.online != nil && s.a.online.Offline()
	parts, known := s.a.parts.Known(videoID, offline)
	return parts, known || offline
}

// SetSkips has the engine pass over the parts of url's track the player
// tells, as player.Skipper.
func (s engineStream) SetSkips(url string, skips []audio.Skip) { s.e.SetSkips(url, skips) }

var _ player.Skipper = engineStream{}

// Skipped tells the player of a part the engine passed over, as
// audio.SkipEvents.
func (e engineEvents) Skipped(url string, from, to time.Duration) {
	if s, ok := e.to().(audio.SkipEvents); ok {
		s.Skipped(url, from, to)
	}
}

var _ audio.SkipEvents = engineEvents{}
