package app

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/builtin"
	"github.com/fzn0x/mixmusic/internal/backup"
	"github.com/fzn0x/mixmusic/internal/downloads"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
)

func TestAppCacheLocationLimitAndMerge(t *testing.T) {
	dir := t.TempDir()
	a, err := New(Options{
		DataDir:      filepath.Join(dir, "data"),
		CacheDir:     filepath.Join(dir, "cache"),
		DownloadsDir: filepath.Join(dir, "downloads"),
		Builtin:      builtin.FS,
		Version:      "test",
		ProblemWait:  -1,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.downloads.Close)
	t.Cleanup(a.artStore.Close)

	sys := &System{a}
	prefs := &Preferences{a}
	dl := &Downloads{a}

	// Custom and unlimited size via settings.
	s := prefs.Get()
	s.CacheSizeMB = settings.UnlimitedCache
	if _, err := prefs.Set(s); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"aaaaaaaaaaa", "bbbbbbbbbbb"} {
		cp := a.cache.Begin(id, "audio/webm", 5)
		if cp == nil {
			t.Fatalf("Begin returned nil for %s", id)
		}
		io.WriteString(cp, "audio")
		cp.Commit()
	}
	if _, tracks := a.cache.Usage(); tracks != 2 {
		t.Fatalf("tracks = %d, want 2", tracks)
	}

	// Move cache to a custom folder and back to default.
	custom := filepath.Join(dir, "custom-cache")
	res, err := sys.MoveCache(context.Background(), custom, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Folder != custom || res.Merged || !sameFolder(sys.CacheFolder(), custom) {
		t.Fatalf("MoveCache to custom = %+v, active=%q", res, sys.CacheFolder())
	}
	s = prefs.Get()
	s.CacheFolder = res.Folder
	if _, err := prefs.Set(s); err != nil {
		t.Fatal(err)
	}
	if _, tracks := a.cache.Usage(); tracks != 2 {
		t.Fatalf("after MoveCache tracks = %d, want 2", tracks)
	}

	// Merge with downloads via SetCacheMerged.
	res, err = sys.SetCacheMerged(context.Background(), true, nil)
	if err != nil {
		t.Fatal(err)
	}
	wantMerged := filepath.Join(dl.Folder(), "audio")
	if !res.Merged || res.Folder != custom || !sameFolder(sys.CacheFolder(), wantMerged) {
		t.Fatalf("SetCacheMerged(true) = %+v, active=%q", res, sys.CacheFolder())
	}
	s = prefs.Get()
	s.CacheWithDownloads = true
	if _, err := prefs.Set(s); err != nil {
		t.Fatal(err)
	}

	// Add an actual download in the shared folder and verify cache.Clear() leaves it alone.
	audio, err := os.ReadFile(filepath.Join("..", "..", "webm", "testdata", "stereo-cues-front.webm"))
	if err != nil {
		t.Fatal(err)
	}
	a.downloads.Close()
	a.downloads = downloads.Open(dl.Folder(), downloads.Options{
		Fetcher:   webmFetcher{audio},
		Cache:     a.cache,
		AvoidFunc: a.downloadsAvoid,
	})
	t.Cleanup(a.downloads.Close)
	a.downloads.Add([]media.Track{ytTrack("ccccccccccc")}, "best")
	a.downloads.Start()
	for deadline := time.Now().Add(10 * time.Second); a.downloads.Status().Tracks < 1; time.Sleep(5 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("download didn't finish: %+v", a.downloads.Status())
		}
	}

	// Moving Downloads while merged moves both downloaded and cached tracks.
	newDL := filepath.Join(dir, "new-downloads")
	gotDL, err := dl.Move(context.Background(), newDL, nil)
	if err != nil || !sameFolder(gotDL, newDL) {
		t.Fatalf("dl.Move = %q, %v", gotDL, err)
	}
	if !sameFolder(sys.CacheFolder(), filepath.Join(newDL, "audio")) {
		t.Errorf("cache folder after dl.Move = %q", sys.CacheFolder())
	}
	for _, name := range []string{"aaaaaaaaaaa.webm", "bbbbbbbbbbb.webm", "ccccccccccc.webm"} {
		if _, err := os.Stat(filepath.Join(newDL, "audio", name)); err != nil {
			t.Errorf("missing %s after dl.Move: %v", name, err)
		}
	}

	// Clearing cache leaves downloaded ccccccccccc.webm intact.
	if err := sys.ClearCache(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(newDL, "audio", "ccccccccccc.webm")); err != nil {
		t.Errorf("downloaded file deleted by ClearCache: %v", err)
	}

	// Re-add a cached track, then un-merge: only cached track moves out, download stays.
	cp := a.cache.Begin("ddddddddddd", "audio/webm", 5)
	io.WriteString(cp, "hello")
	cp.Commit()

	res, err = sys.SetCacheMerged(context.Background(), false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Merged || res.Folder != custom || !sameFolder(sys.CacheFolder(), custom) {
		t.Fatalf("SetCacheMerged(false) = %+v, active=%q", res, sys.CacheFolder())
	}
	if _, err := os.Stat(filepath.Join(custom, "ddddddddddd.webm")); err != nil {
		t.Errorf("un-merged cached track missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(newDL, "audio", "ddddddddddd.webm")); err == nil {
		t.Error("un-merged cached track left in downloads")
	}
	if _, err := os.Stat(filepath.Join(newDL, "audio", "ccccccccccc.webm")); err != nil {
		t.Errorf("downloaded track missing after un-merge: %v", err)
	}

	// Reset keeps CacheFolder and CacheWithDownloads, resets CacheSizeMB.
	s = prefs.Get()
	s.CacheFolder = custom
	s.CacheWithDownloads = true
	s.CacheSizeMB = 2500
	if _, err := prefs.Set(s); err != nil {
		t.Fatal(err)
	}
	reset, err := prefs.Reset()
	if err != nil {
		t.Fatal(err)
	}
	if reset.CacheFolder != custom || !reset.CacheWithDownloads || reset.CacheSizeMB != 1024 {
		t.Errorf("Reset = folder:%q merged:%v size:%d", reset.CacheFolder, reset.CacheWithDownloads, reset.CacheSizeMB)
	}

	// ReloadSettings applies updated CacheSizeMB and CacheFolder.
	writeSettings(t, a.settings.Path(), strings.ReplaceAll(`{"cacheSizeMb":3000,"cacheFolder":"`+filepath.Join(dir, "reloaded")+`"}`, `\`, `\\`))
	a.ReloadSettings()
	if prefs.Get().CacheSizeMB != 3000 || !sameFolder(sys.CacheFolder(), filepath.Join(dir, "reloaded")) {
		t.Errorf("ReloadSettings: size=%d active=%q", prefs.Get().CacheSizeMB, sys.CacheFolder())
	}

	// backup.KeepLocal preserves local CacheFolder and CacheWithDownloads.
	incoming := settings.Default()
	incoming.CacheFolder = "/other/cache"
	incoming.CacheWithDownloads = false
	here := settings.Default()
	here.CacheFolder = custom
	here.CacheWithDownloads = true
	kept := backup.KeepLocal(incoming, here)
	if kept.CacheFolder != custom || !kept.CacheWithDownloads {
		t.Errorf("KeepLocal = %q, %v", kept.CacheFolder, kept.CacheWithDownloads)
	}
}

func TestCacheMergedFreshDefaultDownloads(t *testing.T) {
	dir := t.TempDir()
	a, err := New(Options{
		DataDir:      filepath.Join(dir, "data"),
		CacheDir:     filepath.Join(dir, "cache"),
		DownloadsDir: filepath.Join(dir, "downloads"),
		Builtin:      builtin.FS,
		Version:      "test",
		ProblemWait:  -1,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.downloads.Close)
	t.Cleanup(a.artStore.Close)

	sys := &System{a}
	prefs := &Preferences{a}
	s := prefs.Get()
	s.CacheWithDownloads = true
	if _, err := prefs.Set(s); err != nil {
		t.Fatal(err)
	}
	if prob := sys.CacheProblem(); prob != "" {
		t.Fatalf("fresh default downloads folder reported problem: %q", prob)
	}
	cp := a.cache.Begin("fffffffffff", "audio/webm", 4)
	if cp == nil {
		t.Fatal("Begin returned nil on fresh default downloads folder")
	}
	io.WriteString(cp, "data")
	cp.Commit()
	if use := sys.CacheUsage(); use.Tracks != 1 {
		t.Fatalf("CacheUsage = %+v, want 1 track", use)
	}
}
