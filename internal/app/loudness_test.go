package app

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/match"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/store"
	"github.com/fzn0x/mixmusic/internal/stream"
)

// openBook opens the loudness kept at path, asking with ask.
func openBook(t *testing.T, path string, ask func(context.Context, string) (float64, error)) *loudnessBook {
	t.Helper()
	known, err := store.Open(path, map[string]float64{})
	if err != nil {
		t.Fatal(err)
	}
	return newLoudnessBook(ask, known)
}

// waitClosed waits for ch to close.
func waitClosed(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("the ask didn't end")
	}
}

func TestLoudnessBookAsksOnce(t *testing.T) {
	var asks atomic.Int32
	answer := make(chan struct{})
	ask := func(ctx context.Context, videoID string) (float64, error) {
		asks.Add(1)
		<-answer
		return -7.93, nil
	}
	path := filepath.Join(t.TempDir(), "loudness.json")
	b := openBook(t, path, ask)
	first, second := b.learn("3IUqoyTxEU8"), b.learn("3IUqoyTxEU8")
	if _, ok := b.soon(context.Background(), "3IUqoyTxEU8", 10*time.Millisecond); ok {
		t.Error("known before YouTube answered")
	}
	close(answer)
	waitClosed(t, first)
	waitClosed(t, second)
	if v, ok := b.lkfs("3IUqoyTxEU8"); !ok || v != -7.93 || asks.Load() != 1 {
		t.Errorf("%v, %v after %d asks", v, ok, asks.Load())
	}
	waitClosed(t, b.learn("3IUqoyTxEU8"))
	if asks.Load() != 1 {
		t.Error("asked again about a video it knows")
	}
	// The next run knows it, from the cache folder.
	if v, ok := openBook(t, path, nil).lkfs("3IUqoyTxEU8"); !ok || v != -7.93 {
		t.Errorf("after a restart: %v, %v", v, ok)
	}
}

func TestLoudnessBookWaitsForAnAsk(t *testing.T) {
	b := openBook(t, filepath.Join(t.TempDir(), "loudness.json"), func(ctx context.Context, videoID string) (float64, error) {
		time.Sleep(20 * time.Millisecond)
		return -9, nil
	})
	b.learn("3IUqoyTxEU8")
	if v, ok := b.soon(context.Background(), "3IUqoyTxEU8", 5*time.Second); !ok || v != -9 {
		t.Errorf("soon = %v, %v", v, ok)
	}
}

func TestLoudnessBookAsksAgain(t *testing.T) {
	var asks atomic.Int32
	// Set before each ask starts, and read by it.
	var fail error
	b := openBook(t, filepath.Join(t.TempDir(), "loudness.json"), func(ctx context.Context, videoID string) (float64, error) {
		asks.Add(1)
		return 0, fail
	})
	// Offline, the next play asks again.
	fail = &net.OpError{Op: "dial", Err: errors.New("no network")}
	waitClosed(t, b.learn("3IUqoyTxEU8"))
	waitClosed(t, b.learn("3IUqoyTxEU8"))
	if asks.Load() != 2 {
		t.Errorf("%d asks offline", asks.Load())
	}
	// A video YouTube tells no loudness of isn't asked about again for a
	// while.
	fail = stream.ErrNoLoudness
	waitClosed(t, b.learn("XqZsoesa55w"))
	waitClosed(t, b.learn("XqZsoesa55w"))
	if asks.Load() != 3 {
		t.Errorf("%d asks of a video with no measure", asks.Load())
	}
	if _, ok := b.lkfs("XqZsoesa55w"); ok {
		t.Error("a failed ask kept a measure")
	}
	// A stream that tells its loudness does.
	b.keep("XqZsoesa55w", -6.96)
	if v, ok := b.lkfs("XqZsoesa55w"); !ok || v != -6.96 {
		t.Errorf("kept %v, %v", v, ok)
	}
}

func TestLoudnessBookKeepsOnlyMeasures(t *testing.T) {
	b := openBook(t, filepath.Join(t.TempDir(), "loudness.json"), func(ctx context.Context, videoID string) (float64, error) {
		return 3, nil
	})
	waitClosed(t, b.learn("3IUqoyTxEU8"))
	b.keep("CHIWNDAwTqQ", -90)
	for _, id := range []string{"3IUqoyTxEU8", "CHIWNDAwTqQ"} {
		if v, ok := b.lkfs(id); ok {
			t.Errorf("%s kept at %v LKFS", id, v)
		}
	}
}

func TestLoudnessBookKeepsWhatItCantSave(t *testing.T) {
	// The cache folder is a file once the book is open: nothing saves
	// there. Opening a file in it fails on Linux and macOS, as not a
	// folder, while Windows finds no file there.
	dir := filepath.Join(t.TempDir(), "cache")
	b := openBook(t, filepath.Join(dir, "loudness.json"), nil)
	if err := os.WriteFile(dir, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	b.keep("3IUqoyTxEU8", -7.93)
	if v, ok := b.lkfs("3IUqoyTxEU8"); !ok || v != -7.93 {
		t.Errorf("%v, %v", v, ok)
	}
	if _, ok := b.known.Get()["3IUqoyTxEU8"]; ok {
		t.Error("the loudness was saved")
	}
}

func TestNoLoudnessBook(t *testing.T) {
	var b *loudnessBook
	b.keep("3IUqoyTxEU8", -7.93)
	waitClosed(t, b.learn("3IUqoyTxEU8"))
	if _, ok := b.soon(context.Background(), "3IUqoyTxEU8", time.Second); ok {
		t.Error("a nil book knows a loudness")
	}
}

// TestPrepareStreamLearnsTheLoudness asks YouTube how loud a track is when
// its stream doesn't tell, as one yt-dlp found, or one in the cache.
func TestPrepareStreamLearnsTheLoudness(t *testing.T) {
	var asked atomic.Int32
	r := stream.NewResolver(nil, "")
	r.Run = func(context.Context, ...string) ([]byte, error) {
		return []byte(`{"url": "https://example.com/a", "ext": "webm", "acodec": "opus", "filesize": 1000}`), nil
	}
	cache := stream.NewCache(t.TempDir(), 1<<20)
	if err := os.WriteFile(filepath.Join(cache.Dir, "CHIWNDAwTqQ.webm"), []byte("audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := &App{resolver: r, cache: cache, matcher: match.New(nil, nil), settings: testSettings(t)}
	a.loudness = openBook(t, filepath.Join(t.TempDir(), "loudness.json"), func(ctx context.Context, videoID string) (float64, error) {
		asked.Add(1)
		return map[string]float64{"3IUqoyTxEU8": -7.93, "CHIWNDAwTqQ": -11.93}[videoID], nil
	})
	p := &Player{a: a}
	for id, want := range map[string]float64{"3IUqoyTxEU8": -7.93, "CHIWNDAwTqQ": -11.93} {
		track := media.Track{ID: "yt:" + id, Source: media.SourceYouTube}
		if _, err := p.PrepareStream(context.Background(), track); err != nil {
			t.Fatal(err)
		}
		if v, ok := a.loudness.soon(context.Background(), id, 5*time.Second); !ok || v != want {
			t.Errorf("%s: %v, %v", id, v, ok)
		}
	}
	if asked.Load() != 2 {
		t.Errorf("%d asks", asked.Load())
	}
}

// TestPrepareStreamAsksOnline asks how loud a track MixMusic holds is, from
// the cache or found before, only while the network answers: offline, the
// engine would wait for the ask in vain.
func TestPrepareStreamAsksOnline(t *testing.T) {
	n := newNetTest(t)
	ctx := context.Background()
	var asks atomic.Int32
	n.p.a.loudness = openBook(t, filepath.Join(n.dir, "loudness.json"), func(context.Context, string) (float64, error) {
		asks.Add(1)
		return -9, nil
	})
	// A stream found without PrepareStream, as a run of yt-dlp that went on
	// once the network had gone, and a track in the cache.
	n.ytdlp.Store(true)
	n.net.Store(true)
	if _, err := n.p.a.resolver.Resolve(ctx, "bbbbbbbbbbb"); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(n.dir, "audio"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(n.dir, "audio", "ddddddddddd.webm"), []byte("audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	held := []string{"bbbbbbbbbbb", "ddddddddddd"}

	n.ytdlp.Store(false)
	n.net.Store(false)
	if _, err := n.p.PrepareStream(ctx, ytTrack("w3XzgjlKFjc")); !errors.Is(err, player.ErrOffline) {
		t.Fatalf("without the network: %v", err)
	}
	for _, id := range held {
		if _, err := n.p.PrepareStream(ctx, ytTrack(id)); err != nil {
			t.Fatalf("%s offline: %v", id, err)
		}
		// soon waits for an ask under way, which ends with the measure.
		if _, ok := n.p.a.loudness.soon(ctx, id, 5*time.Second); ok || asks.Load() != 0 {
			t.Errorf("%s asked about offline", id)
		}
	}

	n.net.Store(true)
	n.p.a.online.Seen()
	for _, id := range held {
		if _, err := n.p.PrepareStream(ctx, ytTrack(id)); err != nil {
			t.Fatal(err)
		}
		if v, ok := n.p.a.loudness.soon(ctx, id, 5*time.Second); !ok || v != -9 {
			t.Errorf("%s back online: %v, %v", id, v, ok)
		}
	}
	if asks.Load() != 2 {
		t.Errorf("%d asks back online", asks.Load())
	}
}
