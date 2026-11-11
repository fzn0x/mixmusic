package app

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/store"
)

// The engine asks a file on this computer whether it was cut short.
var _ audio.Shortened = fileSource{}

// TestFileSourceShortened tells a file cut short after it was opened, and
// not one whose tags were written meanwhile, which is a new file put in its
// place (#456).
func TestFileSourceShortened(t *testing.T) {
	dir := t.TempDir()
	open := func(name string) (string, fileSource) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, bytes.Repeat([]byte("x"), 4000), 0o644); err != nil {
			t.Fatal(err)
		}
		f, err := store.OpenShared(path)
		if err != nil {
			t.Fatal(err)
		}
		s := newFileSource(f)
		t.Cleanup(func() { s.Close() })
		if s.Shortened() {
			t.Fatalf("%s: a whole file is shortened", name)
		}
		return path, s
	}

	path, s := open("cut.flac")
	if err := os.Truncate(path, 1000); err != nil {
		t.Fatal(err)
	}
	if !s.Shortened() {
		t.Error("a file cut short as it played isn't shortened")
	}

	// Tags are written as tagwrite does, through store.Replace.
	path, s = open("tagged.flac")
	if err := store.Replace(path, 0o644, func(f *os.File) error {
		_, err := f.Write(bytes.Repeat([]byte("y"), 100))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if s.Shortened() {
		t.Error("a new file put in place of the one open, as a tag edit does, shortens it")
	}

	path, s = open("longer.flac")
	if err := os.WriteFile(path, bytes.Repeat([]byte("z"), 8000), 0o644); err != nil {
		t.Fatal(err)
	}
	if s.Shortened() {
		t.Error("a file that grew is shortened")
	}
}
