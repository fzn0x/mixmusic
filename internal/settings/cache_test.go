package settings

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// TestCacheDefaults checks the default cache size, folder, and merge flag.
func TestCacheDefaults(t *testing.T) {
	d := Default()
	if d.CacheSizeMB != 1024 || d.CacheFolder != "" || d.CacheWithDownloads {
		t.Errorf("defaults: size=%d folder=%q merge=%v", d.CacheSizeMB, d.CacheFolder, d.CacheWithDownloads)
	}
}

// TestCacheClean verifies unlimited, custom, clamped sizes and absolute vs
// relative cache folders.
func TestCacheClean(t *testing.T) {
	dir := t.TempDir()
	for _, c := range []struct {
		file string
		mb   int
	}{
		{`{}`, 1024},
		{`{"cacheSizeMb":0}`, 0},
		{`{"cacheSizeMb":2500}`, 2500},
		{`{"cacheSizeMb":102400}`, 102400},
		{`{"cacheSizeMb":-1}`, UnlimitedCache},
		{`{"cacheSizeMb":-99}`, UnlimitedCache},
		{`{"cacheSizeMb":99999999}`, MaxCacheSizeMB},
	} {
		s := Default()
		if err := json.Unmarshal([]byte(c.file), &s); err != nil {
			t.Fatal(err)
		}
		if got := s.Clean().CacheSizeMB; got != c.mb {
			t.Errorf("%s: CacheSizeMB = %d, want %d", c.file, got, c.mb)
		}
	}

	s := Default()
	s.CacheFolder = "  " + filepath.Join(dir, "a", "..", "cache") + " "
	if got := s.Clean().CacheFolder; got != filepath.Join(dir, "cache") {
		t.Errorf("absolute CacheFolder came out as %q", got)
	}
	s.CacheFolder = filepath.Join("relative", "cache")
	if got := s.Clean().CacheFolder; got != "" {
		t.Errorf("relative CacheFolder came out as %q", got)
	}
}

// TestCacheFolderNotUTF8 verifies base64 path encoding round-trips CacheFolder.
func TestCacheFolderNotUTF8(t *testing.T) {
	s := Default()
	s.CacheFolder = filepath.Join(t.TempDir(), "caf\xe9")
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"base64:`) || strings.ContainsRune(string(data), '\ufffd') {
		t.Fatalf("unexpected marshaled JSON: %s", data)
	}
	got, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	if got.CacheFolder != s.Clean().CacheFolder {
		t.Errorf("decoded CacheFolder = %q, want %q", got.CacheFolder, s.Clean().CacheFolder)
	}
}
