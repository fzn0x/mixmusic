package store

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// replaceWith replaces the file at path with text.
func replaceWith(path, text string) error {
	return Replace(path, 0o644, func(f *os.File) error {
		_, err := io.WriteString(f, text)
		return err
	})
}

// onlyFile checks that dir holds the file name alone, of text.
func onlyFile(t *testing.T, dir, name, text string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != name {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("the folder holds %q", names)
	}
	if b, err := os.ReadFile(filepath.Join(dir, name)); err != nil || string(b) != text {
		t.Errorf("the file holds %q, %v; want %q", b, err, text)
	}
}

// TestReplace writes a file anew beside the old one, which it renames
// over it, leaving nothing else.
func TestReplace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "song.flac")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := replaceWith(path, "new"); err != nil {
		t.Fatal(err)
	}
	onlyFile(t, dir, "song.flac", "new")
}

// TestReplaceFails leaves the old file as it was, and nothing beside it,
// when the new one can't be written.
func TestReplaceFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "song.flac")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	broke := errors.New("the disk is full")
	err := Replace(path, 0o644, func(f *os.File) error {
		io.WriteString(f, "half of the new")
		return broke
	})
	if !errors.Is(err, broke) {
		t.Errorf("Replace: %v", err)
	}
	onlyFile(t, dir, "song.flac", "old")
}

// TestReplaceWhileRead replaces a file another reader has open through
// OpenShared, which goes on reading the old one.
func TestReplaceWhileRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "song.flac")
	if err := os.WriteFile(path, []byte("old audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := OpenShared(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	head := make([]byte, 4)
	if _, err := io.ReadFull(f, head); err != nil {
		t.Fatal(err)
	}
	if err := replaceWith(path, "new audio"); err != nil {
		t.Fatalf("replacing a file read: %v", err)
	}
	rest, err := io.ReadAll(f)
	if err != nil || string(head)+string(rest) != "old audio" {
		t.Errorf("the reader read %q, %v", string(head)+string(rest), err)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		t.Errorf("seeking the old file: %v", err)
	}
	f.Close()
	onlyFile(t, dir, "song.flac", "new audio")
}
