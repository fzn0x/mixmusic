package clips

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testStore is a store in a folder of the test's, whose clock moves a
// second each time it's read.
func testStore(t *testing.T, limit int64) *Store {
	t.Helper()
	s := NewStore(filepath.Join(t.TempDir(), "moving-covers"), limit)
	at := time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC)
	s.now = func() time.Time {
		at = at.Add(time.Second)
		return at
	}
	return s
}

func keep(t *testing.T, s *Store, key string, n int) Match {
	t.Helper()
	m, err := s.Keep(key, Match{Mime: "video/mp4", Width: 600, Height: 600, Source: "https://clips.example/" + key + ".mp4"}, bytes.NewReader(make([]byte, n)))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// TestKeep keeps an album's clip, which plays from its file, and a miss,
// which keeps no file; both are read again by a store of the same
// folder, as MixMusic starting again.
func TestKeep(t *testing.T) {
	s := testStore(t, -1)
	m := keep(t, s, "ada field|kites", 1000)
	if m.Bytes != 1000 || m.Miss() || filepath.Ext(m.File) != ".mp4" || m.Width != 600 {
		t.Errorf("kept %+v", m)
	}
	path, ok := s.File("ada field|kites")
	if info, err := os.Stat(path); !ok || err != nil || info.Size() != 1000 {
		t.Errorf("the clip's file is %s: %v", path, err)
	}
	s.Miss("orin vale|paper")
	if m, ok := s.Match("orin vale|paper"); !ok || !m.Miss() {
		t.Errorf("the miss is %+v, %v", m, ok)
	}
	if _, ok := s.File("orin vale|paper"); ok {
		t.Error("a miss has a file")
	}
	if b, n := s.Usage(); b != 1000 || n != 1 {
		t.Errorf("the store holds %d clips of %d bytes", n, b)
	}

	again := NewStore(s.dir, -1)
	if m, ok := again.Match("ada field|kites"); !ok || m.Bytes != 1000 || m.Source != "https://clips.example/ada field|kites.mp4" {
		t.Errorf("started again, the clip is %+v", m)
	}
	if m, ok := again.Match("orin vale|paper"); !ok || !m.Miss() {
		t.Errorf("started again, the miss is %+v", m)
	}
	if _, ok := again.Match("mira holt|glass"); ok {
		t.Error("an album never looked for has a match")
	}
}

// TestKeepTooLarge keeps no clip of more than MaxClip bytes, nor leaves a
// file of it.
func TestKeepTooLarge(t *testing.T) {
	s := testStore(t, -1)
	if _, err := s.Keep("a", Match{Mime: "video/mp4"}, bytes.NewReader(make([]byte, MaxClip+1))); !errors.Is(err, ErrTooLarge) {
		t.Errorf("a clip too large: %v", err)
	}
	if _, err := s.Keep("a", Match{Mime: "video/mp4"}, strings.NewReader("")); err == nil {
		t.Error("a clip of no bytes is kept")
	}
	if entries, _ := os.ReadDir(s.dir); len(entries) != 0 {
		t.Errorf("the folder holds %d files", len(entries))
	}
}

// TestLimit lets the clips played least recently go past the limit, but
// for the one kept last; Off keeps that one alone, and Unlimited all.
func TestLimit(t *testing.T) {
	s := testStore(t, 2500)
	keep(t, s, "a", 1000)
	keep(t, s, "b", 1000)
	s.Played("a")
	keep(t, s, "c", 1000)
	if _, ok := s.File("b"); ok {
		t.Error("the clip played least recently stays")
	}
	for _, key := range []string{"a", "c"} {
		if _, ok := s.File(key); !ok {
			t.Errorf("%s went", key)
		}
	}
	keep(t, s, "big", 3000)
	if b, n := s.Usage(); n != 1 || b != 3000 {
		t.Errorf("past the limit, the clip kept last stays alone: %d clips of %d bytes", n, b)
	}
	s.SetLimit(-1)
	keep(t, s, "d", 1000)
	keep(t, s, "e", 1000)
	if _, n := s.Usage(); n != 3 {
		t.Errorf("unlimited, %d clips stay", n)
	}
	s.SetLimit(0)
	if _, n := s.Usage(); n != 1 {
		t.Errorf("off, %d clips stay", n)
	}
	if _, ok := s.File("e"); !ok {
		t.Error("off, the clip kept last went")
	}
	if entries, _ := os.ReadDir(s.dir); len(entries) != 2 {
		t.Errorf("off, the folder holds %d files, not the clip and the index", len(entries))
	}
}

// TestForget forgets an album's clip, or its miss, for its source to be
// asked again, and Clear forgets them all.
func TestForget(t *testing.T) {
	s := testStore(t, -1)
	m := keep(t, s, "a", 10)
	s.Forget("a")
	if _, ok := s.Match("a"); ok {
		t.Error("the clip is still kept")
	}
	if _, err := os.Stat(filepath.Join(s.dir, m.File)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("its file stays: %v", err)
	}
	s.Miss("b")
	s.Forget("b")
	if _, ok := s.Match("b"); ok {
		t.Error("the miss is still kept")
	}
	keep(t, s, "c", 10)
	s.Miss("d")
	if err := s.Clear(); err != nil {
		t.Fatal(err)
	}
	if b, n := s.Usage(); b != 0 || n != 0 {
		t.Errorf("cleared, the store holds %d clips", n)
	}
	if _, ok := s.Match("d"); ok {
		t.Error("cleared, a miss stays")
	}
	if _, err := os.Stat(s.dir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("cleared, the folder stays: %v", err)
	}
	// A clip that a miss replaces goes too.
	keep(t, s, "e", 10)
	path, _ := s.File("e")
	s.Miss("e")
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a clip a miss replaced stays: %v", err)
	}
}

// TestStrayFiles removes what the index doesn't name, as a clip whose
// write a crash cut off, and forgets a clip whose file went.
func TestStrayFiles(t *testing.T) {
	s := testStore(t, -1)
	keep(t, s, "a", 10)
	kept, _ := s.File("a")
	keep(t, s, "b", 10)
	gone, _ := s.File("b")
	os.Remove(gone)
	stray := filepath.Join(s.dir, "clip-123")
	if err := os.WriteFile(stray, []byte("cut off"), 0o644); err != nil {
		t.Fatal(err)
	}
	again := NewStore(s.dir, -1)
	if _, ok := again.Match("b"); ok {
		t.Error("a clip whose file went is still kept")
	}
	if path, ok := again.File("a"); !ok || path != kept {
		t.Errorf("the clip kept is %s", path)
	}
	if _, err := os.Stat(stray); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a stray file stays: %v", err)
	}
}

// TestMisses keeps maxMisses misses at most, the oldest going first.
func TestMisses(t *testing.T) {
	s := testStore(t, -1)
	s.load()
	for i := range maxMisses {
		s.index["old"+string(rune('a'+i%26))+string(rune('a'+i/26%26))+string(rune('a'+i/676))] = Match{Found: time.Unix(int64(i), 0)}
	}
	s.Miss("new")
	if _, ok := s.Match("new"); !ok {
		t.Error("the newest miss went")
	}
	if _, ok := s.Match("oldaaa"); ok {
		t.Error("the oldest miss stays")
	}
	if n := len(s.index); n != maxMisses {
		t.Errorf("%d misses kept", n)
	}
}
