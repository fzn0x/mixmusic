package stream

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCacheUnlimited(t *testing.T) {
	dir := t.TempDir()
	c := NewCache(dir, Unlimited)
	for _, id := range []string{"aaaaaaaaaaa", "bbbbbbbbbbb", "ccccccccccc"} {
		cp := c.Begin(id, "audio/webm", 1024)
		if cp == nil {
			t.Fatalf("Begin refused %s under Unlimited", id)
		}
		io.WriteString(cp, strings.Repeat("x", 1024))
		cp.Commit()
	}
	if bytes, tracks := c.Usage(); bytes != 3*1024 || tracks != 3 {
		t.Fatalf("usage = %d bytes, %d tracks", bytes, tracks)
	}
	c.SetLimit(2500)
	if bytes, tracks := c.Usage(); bytes != 2048 || tracks != 2 {
		t.Errorf("after lowering limit: %d bytes, %d tracks", bytes, tracks)
	}
}

func TestCacheIgnoresNonCacheAndProtectedFiles(t *testing.T) {
	dir := t.TempDir()
	// Unrelated and incomplete files must be ignored.
	for name, content := range map[string]string{
		"notes.txt":                 "keep me",
		"index.json":                "{}",
		".mixcache-aaaaaaaaaaa.tmp": "partial",
		".aaaaaaaaaaa.part":         "download part",
		"aaaaaaaaaaa.webm":          "downloaded",
		"bbbbbbbbbbb.webm":          "cached",
		"ccccccccccc.5.1.m4a":       "cached surround",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	c := NewCache(dir, 1<<20)
	c.SetProtected(func(d, name string) bool {
		return name == "aaaaaaaaaaa.webm"
	})
	bytes, tracks := c.Usage()
	wantBytes := int64(len("cached") + len("cached surround"))
	if tracks != 2 || bytes != wantBytes {
		t.Fatalf("Usage = %d bytes, %d tracks; want %d bytes, 2 tracks", bytes, tracks, wantBytes)
	}
	// Pruning to 0 must not touch protected or non-cache files.
	c.SetLimit(10)
	if _, err := os.Stat(filepath.Join(dir, "aaaaaaaaaaa.webm")); err != nil {
		t.Errorf("protected file deleted by prune: %v", err)
	}
	if err := c.Clear(); err != nil {
		t.Fatal(err)
	}
	for _, keep := range []string{"notes.txt", "index.json", ".aaaaaaaaaaa.part", "aaaaaaaaaaa.webm"} {
		if _, err := os.Stat(filepath.Join(dir, keep)); err != nil {
			t.Errorf("%s deleted by Clear: %v", keep, err)
		}
	}
}

func TestCacheTakeInPlace(t *testing.T) {
	dir := t.TempDir()
	c := NewCache(dir, 1<<20)
	cp := c.Begin(testID, "audio/webm", 5)
	io.WriteString(cp, "hello")
	cp.Commit()
	dst := filepath.Join(dir, testID+".webm")
	if ok, err := c.Take(testID, dst); !ok || err != nil {
		t.Fatalf("Take in place = %v, %v", ok, err)
	}
	b, err := os.ReadFile(dst)
	if err != nil || string(b) != "hello" {
		t.Errorf("file after in-place Take = %q, %v", b, err)
	}
}

func TestCacheMoveAndRollback(t *testing.T) {
	src := t.TempDir()
	c := NewCache(src, 1<<20)
	cp := c.Begin("aaaaaaaaaaa", "audio/webm", 5)
	io.WriteString(cp, "alpha")
	cp.Commit()
	cp = c.Begin("bbbbbbbbbbb", "audio/mp4", 4)
	io.WriteString(cp, "beta")
	cp.Commit()

	// Moving into a non-empty folder creates "MixMusic cache".
	parent := t.TempDir()
	if err := os.WriteFile(filepath.Join(parent, "readme.txt"), []byte("user file"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := c.Move(context.Background(), parent, MoveOptions{MustExist: true})
	if err != nil {
		t.Fatal(err)
	}
	wantDir := filepath.Join(parent, cacheMoveFolder)
	if got != wantDir || c.Folder() != wantDir {
		t.Fatalf("moved to %q (active %q), want %q", got, c.Folder(), wantDir)
	}
	if _, err := os.Stat(filepath.Join(wantDir, "aaaaaaaaaaa.webm")); err != nil {
		t.Errorf("moved file missing: %v", err)
	}

	// Cross-drive fallback via copy when rename fails.
	wasRename := cacheRename
	cacheRename = func(oldpath, newpath string) error { return errors.New("cross-device") }
	defer func() { cacheRename = wasRename }()

	next := t.TempDir()
	// Pre-existing file at target is preserved and source is removed.
	if err := os.WriteFile(filepath.Join(next, "bbbbbbbbbbb.m4a"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	got2, err := c.Move(context.Background(), next, MoveOptions{Exact: true})
	if err != nil || got2 != next {
		t.Fatalf("cross-drive move = %q, %v", got2, err)
	}
	if b, _ := os.ReadFile(filepath.Join(next, "bbbbbbbbbbb.m4a")); string(b) != "keep" {
		t.Errorf("existing file overwritten: %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(next, "aaaaaaaaaaa.webm")); string(b) != "alpha" {
		t.Errorf("copied file = %q", b)
	}

	// Validation errors.
	if _, err := c.Move(context.Background(), next, MoveOptions{Exact: true}); !errors.Is(err, ErrCacheSameFolder) {
		t.Errorf("same folder err = %v", err)
	}
	if _, err := c.Move(context.Background(), filepath.Join(next, "sub"), MoveOptions{Exact: true}); !errors.Is(err, ErrCacheIntoTheirs) {
		t.Errorf("inside folder err = %v", err)
	}
	avoid := t.TempDir()
	if _, err := c.Move(context.Background(), avoid, MoveOptions{Exact: true, Avoid: []string{avoid}}); !errors.Is(err, ErrCacheConflict) {
		t.Errorf("conflict err = %v", err)
	}

	// Cancelled move rolls back.
	cacheRename = wasRename
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	roll := t.TempDir()
	if _, err := c.Move(ctx, roll, MoveOptions{Exact: true}); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled move err = %v", err)
	}
	if c.Folder() != next {
		t.Errorf("active folder changed on failed move: %q", c.Folder())
	}

	// Unreachable mustExist root reports Problem and refuses Begin.
	missing := filepath.Join(t.TempDir(), "unplugged", "cache")
	c.SetDir(missing, missing, true)
	if c.Problem() == "" {
		t.Error("expected Problem for missing mustExist folder")
	}
	if cp := c.Begin("ccccccccccc", "audio/webm", 5); cp != nil {
		t.Error("Begin succeeded for missing mustExist folder")
	}
}
