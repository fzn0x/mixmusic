package downloads

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/stream"
)

func TestOwnsAudioFile(t *testing.T) {
	dir := t.TempDir()
	f := &fetcher{}
	m := open(t, dir, Options{Fetcher: f})
	m.Add([]media.Track{yt("aaaaaaaaaaa", "Owned")}, "best")
	m.Start()
	until(t, "download finished", func() bool { return kind(m, "yt:aaaaaaaaaaa") == Done })

	if !m.OwnsAudioFile("aaaaaaaaaaa.webm") {
		t.Error("expected OwnsAudioFile true for downloaded track")
	}
	if m.OwnsAudioFile("aaaaaaaaaaa.5.1.m4a") {
		t.Error("surround variant should not be owned by stereo download")
	}
	if m.OwnsAudioFile("bbbbbbbbbbb.webm") {
		t.Error("unowned cached file reported as owned")
	}
	m.mu.Lock()
	m.busy["ccccccccccc"] = true
	m.mu.Unlock()
	if !m.OwnsAudioFile("ccccccccccc.webm") {
		t.Error("active busy video should be owned")
	}
}

func TestCacheMergePromotionAndMove(t *testing.T) {
	dir := t.TempDir()
	audioDir := filepath.Join(dir, "audio")
	cache := stream.NewCache(audioDir, 10<<20)

	f := &fetcher{}
	m := open(t, dir, Options{Fetcher: f, Cache: cache})
	cache.SetProtected(func(d, name string) bool {
		return filepath.Clean(d) == filepath.Clean(filepath.Join(m.Folder(), "audio")) && m.OwnsAudioFile(name)
	})

	// Seed a track in the shared audio folder as a cached track.
	cp := cache.Begin("ddddddddddd", "audio/webm", int64(len(webmAudio)))
	if cp == nil {
		t.Fatal("cache.Begin returned nil")
	}
	cp.Write(webmAudio)
	cp.Commit()

	// Also seed a second track that stays cache-only.
	cp2 := cache.Begin("eeeeeeeeeee", "audio/webm", int64(len(webmAudio)))
	cp2.Write(webmAudio)
	cp2.Commit()

	if _, tracks := cache.Usage(); tracks != 2 {
		t.Fatalf("cache usage before promotion = %d, want 2", tracks)
	}

	// Downloading ddddddddddd promotes it in-place from cache with zero network calls.
	m.Add([]media.Track{yt("ddddddddddd", "Promoted")}, "best")
	m.Start()
	until(t, "promoted from cache", func() bool { return kind(m, "yt:ddddddddddd") == Done })
	if len(f.asked()) != 0 {
		t.Errorf("fetcher called %d times, want 0", len(f.asked()))
	}

	// After promotion, cache Usage counts only eeeeeeeeeee, and Clear leaves ddddddddddd intact.
	if _, tracks := cache.Usage(); tracks != 1 {
		t.Errorf("cache usage after promotion = %d, want 1", tracks)
	}
	if err := cache.Clear(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(audioDir, "ddddddddddd.webm")); err != nil {
		t.Errorf("downloaded file deleted by cache.Clear: %v", err)
	}
	if _, err := os.Stat(filepath.Join(audioDir, "eeeeeeeeeee.webm")); err == nil {
		t.Error("cache-only file still exists after cache.Clear")
	}

	// Re-add a cache-only track and move Downloads to a new folder; both move together.
	cp3 := cache.Begin("fffffffffff", "audio/webm", int64(len(webmAudio)))
	cp3.Write(webmAudio)
	cp3.Commit()

	dst := t.TempDir()
	got, err := m.Move(context.Background(), dst, true, nil)
	if err != nil || got != dst {
		t.Fatalf("Move = %q, %v", got, err)
	}
	cache.SetDir(filepath.Join(got, "audio"), got, true)
	for _, id := range []string{"ddddddddddd.webm", "fffffffffff.webm"} {
		if _, err := os.Stat(filepath.Join(got, "audio", id)); err != nil {
			t.Errorf("missing %s after Move: %v", id, err)
		}
	}
}
