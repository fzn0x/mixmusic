package app

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/match"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/store"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

// fakeLabels is YouTube telling the labels of videos: those of ai are
// labeled, those of errs fail, and every lookup waits for release, when
// set. It counts the lookups, and those running at once.
type fakeLabels struct {
	mu      sync.Mutex
	ai      map[string]bool
	errs    map[string]error
	release chan struct{}
	asked   map[string]int
	running int
	most    int
}

func (f *fakeLabels) lookup(ctx context.Context, videoID string) (bool, error) {
	f.mu.Lock()
	f.asked[videoID]++
	f.running++
	f.most = max(f.most, f.running)
	release := f.release
	f.mu.Unlock()
	defer func() {
		f.mu.Lock()
		f.running--
		f.mu.Unlock()
	}()
	if release != nil {
		select {
		case <-release:
		case <-ctx.Done():
			return false, ctx.Err()
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.errs[videoID]; err != nil {
		return false, err
	}
	return f.ai[videoID], nil
}

func (f *fakeLabels) count(videoID string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.asked[videoID]
}

// clock is a time the test moves.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) move(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// eventually waits until ok, for 5 seconds at most.
func eventually(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("%s didn't happen", what)
		}
		time.Sleep(time.Millisecond)
	}
}

// openLabels opens the labels kept at path, which f tells.
func openLabels(t *testing.T, path string, f *fakeLabels, c *clock) *labelCache {
	t.Helper()
	file, err := store.Open(path, map[string]int64{})
	if err != nil {
		t.Fatal(err)
	}
	cache := newLabelCache(f.lookup, file, c.now)
	// Saved before the folder goes, rather than a moment after.
	t.Cleanup(cache.save)
	return cache
}

func newFakeLabels(ai ...string) *fakeLabels {
	f := &fakeLabels{ai: map[string]bool{}, errs: map[string]error{}, asked: map[string]int{}}
	for _, id := range ai {
		f.ai[id] = true
	}
	return f
}

func TestLabelCacheAsksOnce(t *testing.T) {
	f := newFakeLabels("labeledAI01")
	f.release = make(chan struct{})
	c := openLabels(t, filepath.Join(t.TempDir(), "ai-labels.json"), f, &clock{t: time.Unix(1_800_000_000, 0)})
	ctx := context.Background()
	var wg sync.WaitGroup
	got := make([]player.Label, 5)
	for i := range got {
		wg.Go(func() { got[i] = c.Get(ctx, "labeledAI01", i%2 == 0) })
	}
	eventually(t, "the lookup", func() bool { return f.count("labeledAI01") == 1 })
	close(f.release)
	wg.Wait()
	for i, l := range got {
		if l != player.LabelAI {
			t.Errorf("caller %d got %v", i, l)
		}
	}
	if c.Get(ctx, "labeledAI01", true) != player.LabelAI || c.Get(ctx, "notLabeled1", true) != player.LabelNone {
		t.Error("the labels weren't kept")
	}
	c.Get(ctx, "notLabeled1", false)
	if f.count("labeledAI01") != 1 || f.count("notLabeled1") != 1 {
		t.Errorf("asked %v", f.asked)
	}
}

// TestLabelCacheSlots runs labelSlots lookups at once at most, of which
// laterSlots of tracks not about to play: one about to play doesn't wait
// behind them.
func TestLabelCacheSlots(t *testing.T) {
	f := newFakeLabels()
	f.release = make(chan struct{})
	c := openLabels(t, filepath.Join(t.TempDir(), "ai-labels.json"), f, &clock{t: time.Unix(1_800_000_000, 0)})
	ctx := context.Background()
	var wg sync.WaitGroup
	for _, id := range []string{"later000001", "later000002", "later000003", "later000004", "later000005"} {
		wg.Go(func() { c.Get(ctx, id, false) })
	}
	running := func() int {
		f.mu.Lock()
		defer f.mu.Unlock()
		return f.running
	}
	t.Cleanup(func() {
		select {
		case <-f.release:
		default:
			close(f.release) // the lookups end, should the test fail
		}
	})
	eventually(t, "the radio's lookups", func() bool { return running() == laterSlots })
	time.Sleep(20 * time.Millisecond)
	if n := running(); n != laterSlots {
		t.Errorf("%d lookups of a radio at once", n)
	}
	// A track about to play goes ahead, and so does one waiting, asked
	// again for a track about to play.
	waiting := ""
	for _, id := range []string{"later000001", "later000002", "later000003"} {
		if f.count(id) == 0 {
			waiting = id
		}
	}
	wg.Go(func() { c.Get(ctx, "soon0000001", true) })
	wg.Go(func() { c.Get(ctx, waiting, true) })
	eventually(t, "the lookups of tracks about to play", func() bool { return running() == laterSlots+2 })
	for _, id := range []string{"soon0000002", "soon0000003"} {
		wg.Go(func() { c.Get(ctx, id, true) })
	}
	time.Sleep(20 * time.Millisecond)
	if n := running(); n != labelSlots {
		t.Errorf("%d lookups at once", n)
	}
	close(f.release)
	wg.Wait()
	if f.most > labelSlots {
		t.Errorf("%d lookups at once", f.most)
	}
}

// TestLabelCacheKeeps keeps the videos found labeled across runs, for
// labeledFor, and those not labeled in memory alone, for cleanFor.
func TestLabelCacheKeeps(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ai-labels.json")
	f := newFakeLabels("labeledAI01")
	now := &clock{t: time.Unix(1_800_000_000, 0)}
	c := openLabels(t, path, f, now)
	ctx := context.Background()
	c.Get(ctx, "labeledAI01", true)
	c.Get(ctx, "notLabeled1", true)
	c.save()

	// The next run knows the label, without asking.
	f2 := newFakeLabels()
	f2.errs["labeledAI01"] = errors.New("offline")
	c2 := openLabels(t, path, f2, now)
	if l := c2.Known("labeledAI01"); l != player.LabelAI {
		t.Errorf("the next run knows %v", l)
	}
	if l := c2.Known("notLabeled1"); l != player.LabelUnknown {
		t.Errorf("a video not labeled was kept: %v", l)
	}
	// Until it expires.
	now.move(labeledFor + time.Second)
	if l := openLabels(t, path, f2, now).Known("labeledAI01"); l != player.LabelUnknown {
		t.Errorf("after %s, known %v", labeledFor, l)
	}
	// In memory, a video not labeled is asked again after cleanFor.
	if l := c.Known("notLabeled1"); l != player.LabelUnknown {
		t.Errorf("after %s, known %v", labeledFor, l)
	}
	c.Get(ctx, "notLabeled1", true)
	now.move(cleanFor - time.Second)
	c.Get(ctx, "notLabeled1", true)
	now.move(2 * time.Second)
	c.Get(ctx, "notLabeled1", true)
	if n := f.count("notLabeled1"); n != 3 {
		t.Errorf("asked %d times", n)
	}
}

// TestLabelCacheSavesLater gathers the labels found into one write, a
// moment after.
func TestLabelCacheSavesLater(t *testing.T) {
	saved := labelSaveAfter
	labelSaveAfter = 20 * time.Millisecond
	t.Cleanup(func() { labelSaveAfter = saved })
	path := filepath.Join(t.TempDir(), "ai-labels.json")
	c := openLabels(t, path, newFakeLabels("labeledAI01", "labeledAI02"), &clock{t: time.Unix(1_800_000_000, 0)})
	c.Get(context.Background(), "labeledAI01", true)
	c.Get(context.Background(), "labeledAI02", true)
	// Once the write is due, and done.
	eventually(t, "the write", func() bool {
		c.mu.Lock()
		due := c.saving == nil && !c.dirty
		c.mu.Unlock()
		return due
	})
	c.writing.Lock()
	c.writing.Unlock()
	file, err := store.Open(path, map[string]int64{})
	if err != nil {
		t.Fatal(err)
	}
	if got := file.Get(); len(got) != 2 || got["labeledAI01"] != 1_800_000_000 {
		t.Errorf("kept %v", got)
	}
}

// TestLabelCacheFails plays a video YouTube has none of, unchecked, and
// stops asking for labelPause once labelFails lookups failed in a row.
func TestLabelCacheFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ai-labels.json")
	f := newFakeLabels()
	f.errs["removed0001"] = youtube.ErrNotFound
	now := &clock{t: time.Unix(1_800_000_000, 0)}
	c := openLabels(t, path, f, now)
	ctx := context.Background()
	for range labelFails + 1 {
		if l := c.Get(ctx, "removed0001", true); l != player.LabelUnknown {
			t.Errorf("a removed video gave %v", l)
		}
	}
	if n := f.count("removed0001"); n != labelFails+1 {
		t.Errorf("a removed video was asked %d times", n)
	}
	for i := range labelFails {
		id := "failing000" + string(rune('1'+i))
		f.errs[id] = errors.New("HTTP 503")
		if l := c.Get(ctx, id, true); l != player.LabelUnknown {
			t.Errorf("a failed lookup gave %v", l)
		}
	}
	if l := c.Get(ctx, "another0001", true); l != player.LabelPaused || f.count("another0001") != 0 {
		t.Errorf("after %d failures: %v, asked %d times", labelFails, l, f.count("another0001"))
	}
	if l := c.Known("another0001"); l != player.LabelPaused {
		t.Errorf("known %v", l)
	}
	now.move(labelPause)
	if l := c.Get(ctx, "another0001", true); l != player.LabelNone || f.count("another0001") != 1 {
		t.Errorf("after the pause: %v, asked %d times", l, f.count("another0001"))
	}
}

// TestLabelCacheGivesUp gives a caller that stops waiting no label, while
// the lookup goes on, and is kept.
func TestLabelCacheGivesUp(t *testing.T) {
	f := newFakeLabels("labeledAI01")
	f.release = make(chan struct{})
	c := openLabels(t, filepath.Join(t.TempDir(), "ai-labels.json"), f, &clock{t: time.Unix(1_800_000_000, 0)})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if l := c.Get(ctx, "labeledAI01", true); l != player.LabelUnknown {
		t.Errorf("gave %v", l)
	}
	close(f.release)
	if l := c.Get(context.Background(), "labeledAI01", true); l != player.LabelAI || f.count("labeledAI01") != 1 {
		t.Errorf("then %v, asked %d times", l, f.count("labeledAI01"))
	}
}

// TestLabelOfRecording checks a Spotify track by its recording on YouTube
// Music, and knows nothing of one whose recording wasn't found yet.
func TestLabelOfRecording(t *testing.T) {
	dir := t.TempDir()
	matches, err := store.Open(filepath.Join(dir, "matches.json"), map[string]media.Track{})
	if err != nil {
		t.Fatal(err)
	}
	matches.Set(map[string]media.Track{"spotify:1": {ID: "yt:labeledAI01", Source: media.SourceYouTube}})
	f := newFakeLabels("labeledAI01")
	var searched atomic.Int32
	a := &App{matcher: match.New(searcher(func() { searched.Add(1) }), matches)}
	a.labels = openLabels(t, filepath.Join(dir, "ai-labels.json"), f, &clock{t: time.Unix(1_800_000_000, 0)})
	s := playerServices{a}
	ctx := context.Background()
	if l := s.KnownLabel(media.Track{ID: "spotify:1", Source: media.SourceSpotify}); l != player.LabelUnknown {
		t.Errorf("known before it was asked: %v", l)
	}
	if l := s.Label(ctx, media.Track{ID: "spotify:1", Source: media.SourceSpotify}, true); l != player.LabelAI {
		t.Errorf("Label = %v", l)
	}
	if l := s.KnownLabel(media.Track{ID: "yt:labeledAI01", Source: media.SourceYouTube}); l != player.LabelAI {
		t.Errorf("its recording: %v", l)
	}
	if l := s.KnownLabel(media.Track{ID: "spotify:2", Source: media.SourceSpotify}); l != player.LabelUnknown || searched.Load() != 0 {
		t.Errorf("a track not matched: %v, %d searches", l, searched.Load())
	}
}

// searcher is a YouTube Music that finds nothing, calling fn on each
// search.
type searcher func()

func (s searcher) Search(context.Context, string, youtube.Filter) ([]media.Item, error) {
	s()
	return nil, nil
}

func TestLabelCacheNil(t *testing.T) {
	var c *labelCache
	if c.Known("x") != player.LabelUnknown || c.Get(context.Background(), "x", true) != player.LabelUnknown {
		t.Error("a nil cache knows labels")
	}
	c.save()
}
