package native

import (
	"context"
	"errors"
	"image"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/artwork"
)

// scriptedArt is a source of images whose addresses fail as scripted, in
// order, then load.
type scriptedArt struct {
	mu    sync.Mutex
	fails map[string][]error
	asked map[string]int
}

func (s *scriptedArt) Image(_ context.Context, rawURL string) (image.Image, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.asked == nil {
		s.asked = map[string]int{}
	}
	s.asked[rawURL]++
	if errs := s.fails[rawURL]; len(errs) > 0 {
		s.fails[rawURL] = errs[1:]
		return nil, errs[0]
	}
	return image.NewRGBA(image.Rect(0, 0, 4, 4)), nil
}

func (s *scriptedArt) count(rawURL string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.asked[rawURL]
}

// testArt is artworks of src on a clock the test moves, without jitter.
// Its updates wait in the channel for the test to run them.
type testArt struct {
	*artworks
	updates chan func()
	at      time.Time
}

func newTestArt(src artSource) *testArt {
	t := &testArt{updates: make(chan func(), 16), at: time.Unix(1_800_000_000, 0)}
	t.artworks = newArtworks(func(fn func()) { t.updates <- fn }, src)
	t.now = func() time.Time { return t.at }
	t.jitter = func(d time.Duration) time.Duration { return d }
	return t
}

// settle runs the update of a fetch.
func (t *testArt) settle(tb testing.TB) {
	tb.Helper()
	select {
	case fn := <-t.updates:
		fn()
	case <-time.After(5 * time.Second):
		tb.Fatal("no fetch ended")
	}
}

func (t *testArt) entry(rawURL string) *artEntry { return t.m[artKey{rawURL, bucket(64), false}] }

var errOffline = &artwork.Error{Host: "i.ytimg.com", Temporary: true, Err: errors.New("dial tcp: i/o timeout")}

// TestArtworkTriesAgainSooner tries an image that couldn't load again
// after 2 seconds, then doubling up to 2 minutes: not before, and once.
func TestArtworkTriesAgainSooner(t *testing.T) {
	const u = "https://i.ytimg.com/vi/a/hq.jpg"
	var errs []error
	for range 8 {
		errs = append(errs, errOffline)
	}
	src := &scriptedArt{fails: map[string][]error{u: errs}}
	a := newTestArt(src)
	for _, want := range []time.Duration{2, 4, 8, 16, 32, 64, 120, 120} {
		a.get(nil, u, 64)
		a.get(nil, u, 64) // loading: no second fetch
		a.settle(t)
		e := a.entry(u)
		if got := e.retryAt.Sub(a.at); got != want*time.Second {
			t.Fatalf("after %d failures, it waits %v, not %v", e.fails, got, want*time.Second)
		}
		asked := src.count(u)
		a.at = e.retryAt.Add(-time.Millisecond)
		if a.get(nil, u, 64); src.count(u) != asked || e.loading {
			t.Fatalf("it tried again %v early", time.Millisecond)
		}
		a.at = e.retryAt
	}
	a.get(nil, u, 64)
	a.settle(t)
	if a.get(nil, u, 64) == nil {
		t.Error("the image that loaded at last doesn't show")
	}
}

// TestArtworkWaitsOnMissingImages waits half an hour on an address that
// gives no image.
func TestArtworkWaitsOnMissingImages(t *testing.T) {
	const u = "https://i.ytimg.com/vi/gone/hq.jpg"
	a := newTestArt(&scriptedArt{fails: map[string][]error{u: {&artwork.Error{Host: "i.ytimg.com", Status: 404, Err: errors.New("HTTP 404")}}}})
	a.get(nil, u, 64)
	a.settle(t)
	if got := a.entry(u).retryAt.Sub(a.at); got != artMissingRetry {
		t.Errorf("a missing image waits %v", got)
	}
}

// TestArtworkHonoursRetryAfter waits as long as the host asks.
func TestArtworkHonoursRetryAfter(t *testing.T) {
	const u = "https://i.ytimg.com/vi/busy/hq.jpg"
	busy := &artwork.Error{Host: "i.ytimg.com", Status: 503, Temporary: true, RetryAfter: 7 * time.Second, Err: errors.New("HTTP 503")}
	a := newTestArt(&scriptedArt{fails: map[string][]error{u: {busy}}})
	a.get(nil, u, 64)
	a.settle(t)
	if got := a.entry(u).retryAt.Sub(a.at); got != 7*time.Second {
		t.Errorf("it waits %v", got)
	}
}

// TestArtworkHostLoadsAgain has the images of a host that failed as the
// network did try again at once, when another of the host's loads; not
// another host's, nor a missing one.
func TestArtworkHostLoadsAgain(t *testing.T) {
	const (
		one     = "https://i.ytimg.com/vi/1/hq.jpg"
		two     = "https://i.ytimg.com/vi/2/hq.jpg"
		missing = "https://i.ytimg.com/vi/gone/hq.jpg"
		loads   = "https://i.ytimg.com/vi/3/hq.jpg"
		other   = "https://i.scdn.co/image/ab67"
	)
	src := &scriptedArt{fails: map[string][]error{
		one: {errOffline}, two: {errOffline}, other: {errOffline},
		missing: {&artwork.Error{Host: "i.ytimg.com", Status: 404, Err: errors.New("HTTP 404")}},
	}}
	a := newTestArt(src)
	for _, u := range []string{one, two, other, missing} {
		a.get(nil, u, 64)
		a.settle(t)
	}
	a.get(nil, loads, 64)
	a.settle(t)
	for u, want := range map[string]bool{one: true, two: true, other: false, missing: false} {
		a.get(nil, u, 64)
		if got := a.entry(u).loading; got != want {
			t.Errorf("%s tries again %v", u, got)
		}
	}
	a.settle(t)
	a.settle(t)
	if src.count(one) != 2 || src.count(two) != 2 {
		t.Errorf("asked for %d and %d times", src.count(one), src.count(two))
	}
}

// TestArtworkJitter waits between half of a wait and all of it.
func TestArtworkJitter(t *testing.T) {
	a := newArtworks(func(func()) {}, &scriptedArt{})
	for _, d := range []time.Duration{2 * time.Second, 16 * time.Second, artMaxRetry} {
		for range 1000 {
			if got := a.jitter(d); got < d/2 || got > d {
				t.Fatalf("jitter(%v) = %v", d, got)
			}
		}
	}
}

// TestArtColorWait has the accent ask again for the colour of artwork that
// couldn't load as soon as a cover would: a while after, as long as the
// host asks, and half an hour after for an address that gives no image.
func TestArtColorWait(t *testing.T) {
	for _, c := range []struct {
		err  error
		want time.Duration
	}{
		{context.DeadlineExceeded, artColorRetry},
		{errOffline, artColorRetry},
		{&artwork.Error{Host: "i.ytimg.com", Status: 503, Temporary: true, RetryAfter: time.Minute, Err: errors.New("HTTP 503")}, time.Minute},
		{&artwork.Error{Host: "i.ytimg.com", Status: 404, Err: errors.New("HTTP 404")}, artMissingRetry},
	} {
		if got := artColorWait(c.err); got != c.want {
			t.Errorf("after %v, the accent waits %v, not %v", c.err, got, c.want)
		}
	}
}

func TestBucket(t *testing.T) {
	for px, want := range map[int]int{1: 64, 64: 64, 65: 96, 120: 128, 600: 768, 2000: 1024} {
		if got := bucket(px); got != want {
			t.Errorf("bucket(%d) = %d, not %d", px, got, want)
		}
	}
}

func TestScaleDown(t *testing.T) {
	for _, c := range []struct{ w, h, px, ww, wh int }{
		{100, 50, 64, 64, 32},
		{50, 100, 64, 32, 64},
		{40, 30, 64, 40, 30}, // small enough
		{1000, 1, 64, 64, 1},
	} {
		got := scaleDown(image.NewRGBA(image.Rect(0, 0, c.w, c.h)), c.px).Bounds()
		if got.Dx() != c.ww || got.Dy() != c.wh {
			t.Errorf("%dx%d at %d: %dx%d", c.w, c.h, c.px, got.Dx(), got.Dy())
		}
	}
}
