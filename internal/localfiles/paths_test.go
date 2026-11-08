package localfiles

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestPathsNotUTF8 keeps the path of a file whose name isn't UTF-8, as a
// name on a Linux drive may be, in the index file, so that the index
// read again, as after a restart, names the same file: in base64, while
// a path that is UTF-8 reads as text.
func TestPathsNotUTF8(t *testing.T) {
	ti := newIndex(t)
	text := filepath.Join(ti.dir, "Café", "Café Nights.mp3") // decomposed, as macOS's names are
	raw := filepath.Join(ti.dir, "caf\xe9", "\xff01.mp3")
	ti.mu.Lock()
	for _, path := range []string{text, raw} {
		ti.entries[path] = Entry{Path: path, Size: 12, Modified: 34}
	}
	ti.mu.Unlock()
	if err := ti.save(); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(filepath.Dir(ti.dir), "local-files.json")
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Café Nights.mp3") || strings.Contains(string(data), `�`) {
		t.Errorf("the index file holds %s", data)
	}
	x := ti.open(t, file, filepath.Join(filepath.Dir(ti.dir), "cache", "local-art"))
	for _, path := range []string{text, raw} {
		e, ok := x.Entry(path)
		if !ok || e.Path != path || e.Size != 12 {
			t.Errorf("%q read back as %+v, %v", path, e, ok)
		}
		if got, ok := PathOf(TrackOf(e).ID); !ok || got != path {
			t.Errorf("the track of %q names %q", path, got)
		}
	}
}

// TestIndexOfBefore reads the index file of the versions before, whose
// paths are all text, a path that wasn't UTF-8 with replacement
// characters in place of its bytes.
func TestIndexOfBefore(t *testing.T) {
	ti := newIndex(t)
	file := filepath.Join(filepath.Dir(ti.dir), "local-files.json")
	a := filepath.Join(ti.dir, "a.mp3")
	b := filepath.Join(ti.dir, "caf�.mp3")
	quoted := func(p string) string { return strings.ReplaceAll(p, `\`, `\\`) }
	old := `{"version": 1, "files": [{"path": "` + quoted(a) + `", "size": 3, "modified": 4, "title": "One"},
		{"path": "` + quoted(b) + `", "size": 5, "modified": 6}]}`
	if err := os.WriteFile(file, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	x := ti.open(t, file, filepath.Join(filepath.Dir(ti.dir), "cache", "local-art"))
	if e, ok := x.Entry(a); !ok || e.Title != "One" {
		t.Errorf("a: %+v, %v", e, ok)
	}
	if e, ok := x.Entry(b); !ok || e.Size != 5 {
		t.Errorf("b: %+v, %v", e, ok)
	}
}

// TestScanNotUTF8 scans files and a folder whose names aren't UTF-8, on
// Linux, whose names are bytes: the index read again, as after a
// restart, holds them, and its next scan reads none of them again.
// Windows' names are UTF-16, which always reads as UTF-8, and macOS's
// disks refuse such names.
func TestScanNotUTF8(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("only Linux names files with bytes that aren't UTF-8")
	}
	ti := newIndex(t)
	song := ti.write(t, "caf\xe9.mp3", "Night|Ada Field", time.Hour)
	folder := filepath.Join(filepath.Dir(ti.dir), "Mus\xefk")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(folder, "\xff01.mp3")
	if err := os.WriteFile(other, []byte("Day|Orin Vale"), 0o644); err != nil {
		t.Fatal(err)
	}
	if n := ti.scan(t, ti.dir, folder); n != 2 {
		t.Fatalf("read %d files, want 2", n)
	}
	x := ti.open(t, filepath.Join(filepath.Dir(ti.dir), "local-files.json"), filepath.Join(filepath.Dir(ti.dir), "cache", "local-art"))
	for _, path := range []string{song, other} {
		e, ok := x.Entry(path)
		if !ok || e.Skipped != "" {
			t.Fatalf("%q after a restart: %+v, %v", path, e, ok)
		}
		if x.Missing(TrackID(path)) {
			t.Errorf("%q is missing after a restart", path)
		}
	}
	before := ti.reads.Load()
	if err := x.Scan(context.Background(), []string{ti.dir, folder}); err != nil {
		t.Fatal(err)
	}
	if n := ti.reads.Load() - before; n != 0 || len(x.Tracks()) != 2 {
		t.Errorf("the scan after a restart read %d files again, and holds %d", n, len(x.Tracks()))
	}
	if st := x.Status(); st.Folders[1].Problem != "" || st.Folders[1].Tracks != 1 {
		t.Errorf("the folder whose name isn't UTF-8: %+v", st.Folders[1])
	}
}
