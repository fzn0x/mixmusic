package app

import (
	"path/filepath"
	"testing"

	"github.com/fzn0x/mixmusic/builtin"
)

// TestResetKeepsDownloadsFolder puts the settings back, but for the folder
// the downloads are in: they would be lost to MixMusic otherwise.
func TestResetKeepsDownloadsFolder(t *testing.T) {
	dir := t.TempDir()
	a, err := New(Options{DataDir: filepath.Join(dir, "data"), CacheDir: filepath.Join(dir, "cache"), Builtin: builtin.FS, Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	p := &Preferences{a}
	s := p.Get()
	s.DownloadsFolder = filepath.Join(dir, "music")
	s.DownloadOnMetered = true
	s.Crossfade = 6
	if _, err := p.Set(s); err != nil {
		t.Fatal(err)
	}
	got, err := p.Reset()
	if err != nil {
		t.Fatal(err)
	}
	if got.DownloadsFolder != s.DownloadsFolder || got.DownloadOnMetered || got.Crossfade != 0 {
		t.Errorf("reset: folder %q, metered %v, crossfade %d", got.DownloadsFolder, got.DownloadOnMetered, got.Crossfade)
	}
}
