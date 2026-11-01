package app

import (
	"context"
	"errors"
	"log"
	"net"
	"sync"
	"time"

	"github.com/fzn0x/mixmusic/internal/store"
	"github.com/fzn0x/mixmusic/internal/stream"
)

const (
	// loudnessAsk bounds asking YouTube how loud a video is.
	loudnessAsk = 15 * time.Second
	// loudnessGrace is how long the engine, opening a track, waits for
	// YouTube to tell how loud it is, while MixMusic asks.
	loudnessGrace = 500 * time.Millisecond
	// unknownFor is how long a video YouTube told no loudness of goes
	// without asking again.
	unknownFor = time.Hour
)

// loudnessBook keeps how loud YouTube measured the videos MixMusic played,
// in LKFS, in loudness.json in the cache folder, so that tracks play at an
// even loudness from the cache too, offline. Its methods are safe for
// concurrent use, and a nil book knows nothing.
type loudnessBook struct {
	// ask asks YouTube how loud a video is.
	ask   func(ctx context.Context, videoID string) (float64, error)
	known *store.File[map[string]float64]

	mu sync.Mutex
	// kept holds the measures that came this run, which known may have
	// failed to save.
	kept map[string]float64
	// unknown holds when to ask again about the videos YouTube told no
	// loudness of: never kept, so that each run asks once.
	unknown map[string]time.Time
	// asking holds the asks under way, closed as they end.
	asking map[string]chan struct{}
}

func newLoudnessBook(ask func(context.Context, string) (float64, error), known *store.File[map[string]float64]) *loudnessBook {
	return &loudnessBook{
		ask:     ask,
		known:   known,
		kept:    map[string]float64{},
		unknown: map[string]time.Time{},
		asking:  map[string]chan struct{}{},
	}
}

// measurable tells whether lkfs is a loudness a track can have.
func measurable(lkfs float64) bool { return lkfs >= -70 && lkfs <= 0 }

// lkfs returns the loudness of a video, when the book knows it.
func (b *loudnessBook) lkfs(videoID string) (float64, bool) {
	if b == nil {
		return 0, false
	}
	b.mu.Lock()
	v, ok := b.kept[videoID]
	b.mu.Unlock()
	if !ok {
		v, ok = b.known.Get()[videoID]
	}
	return v, ok
}

// soon returns the loudness of a video, when the book knows it, after
// waiting up to d for an ask under way.
func (b *loudnessBook) soon(ctx context.Context, videoID string, d time.Duration) (float64, bool) {
	if b == nil {
		return 0, false
	}
	b.mu.Lock()
	asking := b.asking[videoID]
	b.mu.Unlock()
	if asking != nil {
		t := time.NewTimer(d)
		defer t.Stop()
		select {
		case <-asking:
		case <-t.C:
		case <-ctx.Done():
		}
	}
	return b.lkfs(videoID)
}

// keep keeps the loudness of a video, when it is one a track can have, as
// a stream of YouTube's player API tells it.
func (b *loudnessBook) keep(videoID string, lkfs float64) {
	if b == nil || !measurable(lkfs) {
		return
	}
	b.mu.Lock()
	b.kept[videoID] = lkfs
	delete(b.unknown, videoID)
	b.mu.Unlock()
	if v, ok := b.known.Get()[videoID]; ok && v == lkfs {
		return
	}
	err := b.known.Update(func(m *map[string]float64) error {
		if *m == nil {
			*m = map[string]float64{}
		}
		(*m)[videoID] = lkfs
		return nil
	})
	if err != nil {
		log.Printf("mixmusic: keeping the loudness of %s: %v", videoID, err)
	}
}

// learn asks YouTube how loud a video is, unless the book knows, or asks
// already, or YouTube told no loudness of it a while ago. The channel it
// returns is closed once the book knows, or won't.
func (b *loudnessBook) learn(videoID string) <-chan struct{} {
	done := make(chan struct{})
	if b == nil {
		close(done)
		return done
	}
	if _, ok := b.lkfs(videoID); ok {
		close(done)
		return done
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if asking := b.asking[videoID]; asking != nil {
		return asking
	}
	if time.Now().Before(b.unknown[videoID]) {
		close(done)
		return done
	}
	b.asking[videoID] = done
	go func() {
		// Not the caller's context: the engine may wait for the answer.
		ctx, cancel := context.WithTimeout(context.Background(), loudnessAsk)
		defer cancel()
		lkfs, err := b.ask(ctx, videoID)
		var netErr net.Error
		switch {
		case err == nil && measurable(lkfs):
			b.keep(videoID, lkfs)
		case errors.As(err, &netErr), errors.Is(err, context.DeadlineExceeded):
			// Offline: the next play asks again.
		default:
			b.mu.Lock()
			b.unknown[videoID] = time.Now().Add(unknownFor)
			b.mu.Unlock()
			if err != nil && !errors.Is(err, stream.ErrNoLoudness) {
				log.Printf("mixmusic: asking the loudness of %s: %v", videoID, err)
			}
		}
		b.mu.Lock()
		delete(b.asking, videoID)
		b.mu.Unlock()
		close(done)
	}()
	return done
}

// measured is the audio of a video whose loudness MixMusic knows, which
// the engine plays at an even loudness.
type measured struct {
	*stream.Source
	lkfs float64
}

func (m measured) Loudness() (float64, bool) { return m.lkfs, true }
