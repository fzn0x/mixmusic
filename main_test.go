package main

import (
	"path/filepath"
	"testing"
)

// TestDefaultDownloadsDir keeps the downloads where each system keeps an
// app's data, and with a portable copy in its own folder.
func TestDefaultDownloadsDir(t *testing.T) {
	root := t.TempDir()
	at := func(parts ...string) string { return filepath.Join(append([]string{root}, parts...)...) }
	for _, c := range []struct {
		name, goos, portable, data, cache, xdg, want string
	}{
		{"windows", "windows", "", at("Roaming", "MixMusic"), at("Local", "MixMusic"), "", at("Local", "MixMusic", "downloads")},
		{"macOS", "darwin", "", at("Library", "Application Support", "MixMusic"), at("Library", "Caches", "MixMusic"), "",
			at("Library", "Application Support", "MixMusic", "downloads")},
		{"linux", "linux", "", at(".config", "MixMusic"), at(".cache", "MixMusic"), "", at(".local", "share", "MixMusic", "downloads")},
		{"XDG_DATA_HOME", "linux", "", at(".config", "MixMusic"), at(".cache", "MixMusic"), at("data"), at("data", "MixMusic", "downloads")},
		{"a relative XDG_DATA_HOME", "linux", "", at(".config", "MixMusic"), at(".cache", "MixMusic"), "data",
			at(".local", "share", "MixMusic", "downloads")},
		{"portable", "windows", at("portable"), at("portable", "data"), at("portable", "cache"), "", at("portable", "downloads")},
	} {
		if got := defaultDownloadsDir(c.goos, c.portable, c.data, c.cache, root, c.xdg); got != c.want {
			t.Errorf("%s: %s, want %s", c.name, got, c.want)
		}
	}
}
