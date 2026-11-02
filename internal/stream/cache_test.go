package stream

import (
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCacheKeepsWholeStreams(t *testing.T) {
	c := NewCache(t.TempDir(), 1<<20)
	cp := c.Begin("w3XzgjlKFjc", `audio/webm; codecs="opus"`, 5)
	if cp == nil {
		t.Fatal("Begin refused a stream that fits")
	}
	io.WriteString(cp, "hello")
	cp.Commit()

	rec := httptest.NewRecorder()
	if !c.Has("w3XzgjlKFjc") || !c.Serve(rec, httptest.NewRequest("GET", "/yt/w3XzgjlKFjc", nil), "w3XzgjlKFjc") {
		t.Fatal("the committed stream is not served")
	}
	if rec.Body.String() != "hello" || rec.Header().Get("Content-Type") != "audio/webm" {
		t.Errorf("served %q as %q", rec.Body.String(), rec.Header().Get("Content-Type"))
	}

	rec = httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/yt/w3XzgjlKFjc", nil)
	req.Header.Set("Range", "bytes=1-2")
	c.Serve(rec, req, "w3XzgjlKFjc")
	if rec.Code != 206 || rec.Body.String() != "el" {
		t.Errorf("range: %d %q", rec.Code, rec.Body.String())
	}
}

func TestCacheDropsIncompleteStreams(t *testing.T) {
	dir := t.TempDir()
	c := NewCache(dir, 1<<20)
	cp := c.Begin("w3XzgjlKFjc", "audio/webm", 10)
	io.WriteString(cp, "short")
	cp.Commit()
	cp = c.Begin("aaaaaaaaaaa", "audio/webm", 10)
	io.WriteString(cp, "0123456789")
	cp.Abort()
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("left files behind: %v", entries)
	}
}

func TestCacheOffAndUnknownTypes(t *testing.T) {
	if NewCache(t.TempDir(), 0).Begin("w3XzgjlKFjc", "audio/webm", 5) != nil {
		t.Error("a cache of size 0 took a stream")
	}
	if NewCache(t.TempDir(), 100).Begin("w3XzgjlKFjc", "video/x-unknown", 5) != nil {
		t.Error("a stream of unknown type was taken")
	}
}

func TestCachePrunesLeastRecent(t *testing.T) {
	dir := t.TempDir()
	c := NewCache(dir, 25)
	old := time.Now().Add(-time.Hour)
	for i, id := range []string{"aaaaaaaaaaa", "bbbbbbbbbbb", "ccccccccccc"} {
		cp := c.Begin(id, "audio/webm", 10)
		io.WriteString(cp, strings.Repeat("x", 10))
		cp.Commit()
		at := old.Add(time.Duration(i) * time.Minute)
		os.Chtimes(filepath.Join(dir, id+".webm"), at, at)
	}
	c.SetLimit(25)
	if _, _, ok := c.find("aaaaaaaaaaa"); ok {
		t.Error("the least recently played track was kept")
	}
	if bytes, tracks := c.Usage(); bytes != 20 || tracks != 2 {
		t.Errorf("usage = %d bytes, %d tracks", bytes, tracks)
	}
	if err := c.Clear(); err != nil {
		t.Fatal(err)
	}
	if _, tracks := c.Usage(); tracks != 0 {
		t.Error("Clear left tracks")
	}
}

func TestCacheTake(t *testing.T) {
	c := NewCache(t.TempDir(), 1<<20)
	cp := c.Begin(testID, "audio/webm", 5)
	cp.Write([]byte("hello"))
	cp.Commit()
	dst := filepath.Join(t.TempDir(), "audio", testID+".webm")
	if ok, err := c.Take(testID, dst); !ok || err != nil {
		t.Fatalf("Take: %v, %v", ok, err)
	}
	if b, err := os.ReadFile(dst); err != nil || string(b) != "hello" {
		t.Errorf("took %q, %v", b, err)
	}
	if c.Has(testID) {
		t.Error("the cache still holds what it gave")
	}
	if ok, err := c.Take("aaaaaaaaaaa", dst); ok || err != nil {
		t.Errorf("Take of a track it doesn't hold: %v, %v", ok, err)
	}
}
