// Package clips keeps the clips of moving covers (#569): an album's short
// video, which loops muted in place of its cover on the now playing page.
// It is a store of its own, beside the covers' (package artwork), so that
// clips never push covers out. Each clip is a file of up to MaxClip bytes,
// fetched whole, and an index keeps each album's match, a miss too, so
// that its source is asked once and a clip kept plays without the network.
// Past its limit, the clips played least recently go first, but for the
// one kept last, which plays.
package clips

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	// MaxClip bounds the bytes of a clip: one larger isn't kept.
	MaxClip = 24 << 20
	// maxMisses bounds the albums kept as having no clip, the oldest
	// going first.
	maxMisses = 5000
	// indexName is the file of the index, in the store's folder.
	indexName = "index.json"
)

// ErrTooLarge is a clip of more than MaxClip bytes.
var ErrTooLarge = fmt.Errorf("clips: a clip of over %d MB", MaxClip>>20)

// Match is what the store keeps of an album: the clip found for it, or a
// miss, with no File.
type Match struct {
	// File is the name of the clip's file in the store's folder.
	File string `json:"file,omitempty"`
	// Mime is the clip's type, as video/mp4, and Width and Height its
	// size in pixels.
	Mime   string `json:"mime,omitempty"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
	// Bytes is the size of the file.
	Bytes int64 `json:"bytes,omitempty"`
	// Source is where the clip came from: the address of the rendition
	// its source gave, which isn't tried again once it failed.
	Source string `json:"source,omitempty"`
	// Found is when the album was looked for, and Played when its clip
	// last played.
	Found  time.Time `json:"found"`
	Played time.Time `json:"played,omitzero"`
}

// Miss reports whether the album has no clip.
func (m Match) Miss() bool { return m.File == "" }

// Store keeps the clips of moving covers in a folder, and its index. Its
// methods are safe from any goroutine.
type Store struct {
	dir string
	// now is time.Now, which tests replace.
	now func() time.Time

	mu sync.Mutex
	// limit bounds the bytes of the clips: 0 keeps the one kept last
	// alone, and one below 0 bounds them not.
	limit int64
	// index is the albums' matches by their keys, read once; last is the
	// key of the clip kept last, which stays.
	index  map[string]Match
	loaded bool
	last   string
}

// NewStore returns a store of clips in dir, of limit bytes.
func NewStore(dir string, limit int64) *Store {
	return &Store{dir: dir, limit: limit, now: time.Now}
}

// load reads the index, once. A store whose index went is empty: its
// clips, unknown, are removed.
func (s *Store) load() {
	if s.loaded {
		return
	}
	s.loaded = true
	s.index = map[string]Match{}
	data, err := os.ReadFile(filepath.Join(s.dir, indexName))
	if err == nil {
		err = json.Unmarshal(data, &s.index)
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("mixmusic: the index of moving covers doesn't read: %v", err)
		s.index = map[string]Match{}
	}
	// What the index doesn't name, as a clip whose write a crash cut off,
	// goes; and what it names that went is forgotten.
	named := map[string]bool{indexName: true}
	for key, m := range s.index {
		if m.Miss() {
			continue
		}
		if _, err := os.Stat(filepath.Join(s.dir, m.File)); err != nil {
			delete(s.index, key)
			continue
		}
		named[m.File] = true
	}
	entries, _ := os.ReadDir(s.dir)
	for _, e := range entries {
		if !named[e.Name()] {
			os.Remove(filepath.Join(s.dir, e.Name()))
		}
	}
}

// save writes the index.
func (s *Store) save() {
	data, err := json.Marshal(s.index)
	if err == nil {
		err = os.MkdirAll(s.dir, 0o755)
	}
	if err == nil {
		tmp := filepath.Join(s.dir, indexName+".tmp")
		if err = os.WriteFile(tmp, data, 0o644); err == nil {
			err = os.Rename(tmp, filepath.Join(s.dir, indexName))
		}
	}
	if err != nil {
		log.Printf("mixmusic: the index of moving covers isn't saved: %v", err)
	}
}

// Match returns what the store keeps of the album of key: its clip, or a
// miss.
func (s *Store) Match(key string) (Match, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	m, ok := s.index[key]
	return m, ok
}

// File returns the path of the clip kept of the album of key.
func (s *Store) File(key string) (string, bool) {
	m, ok := s.Match(key)
	if !ok || m.Miss() {
		return "", false
	}
	return filepath.Join(s.dir, m.File), true
}

// Path returns the path of a clip's file by its name, as its match gives
// it, while the store keeps it.
func (s *Store) Path(name string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	for _, m := range s.index {
		if !m.Miss() && m.File == name {
			return filepath.Join(s.dir, name), true
		}
	}
	return "", false
}

// Keep keeps the clip r reads as the album of key's, of the type and the
// size m tells, in place of what the store kept of it, and returns its
// match. A clip of more than MaxClip bytes isn't kept.
func (s *Store) Keep(key string, m Match, r io.Reader) (Match, error) {
	// The index reads first: it removes the files it doesn't name.
	s.mu.Lock()
	s.load()
	s.mu.Unlock()
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return Match{}, err
	}
	tmp, err := os.CreateTemp(s.dir, "clip-*")
	if err != nil {
		return Match{}, err
	}
	n, err := io.Copy(tmp, io.LimitReader(r, MaxClip+1))
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil && n > MaxClip {
		err = ErrTooLarge
	}
	if err == nil && n == 0 {
		err = errors.New("clips: a clip of no bytes")
	}
	if err != nil {
		os.Remove(tmp.Name())
		return Match{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	// A name of the album and the rendition: another rendition of the
	// same album is another clip to the page that plays it.
	sum := sha256.Sum256([]byte(key + "|" + m.Source))
	m.File = hex.EncodeToString(sum[:12]) + ext(m.Mime)
	m.Bytes, m.Found, m.Played = n, s.now(), s.now()
	if err := os.Rename(tmp.Name(), filepath.Join(s.dir, m.File)); err != nil {
		os.Remove(tmp.Name())
		return Match{}, err
	}
	if old, ok := s.index[key]; ok && !old.Miss() && old.File != m.File {
		os.Remove(filepath.Join(s.dir, old.File))
	}
	s.index[key], s.last = m, key
	s.trim()
	s.save()
	return m, nil
}

// ext is the extension of a clip's file of type mime.
func ext(mime string) string {
	if strings.HasPrefix(mime, "video/webm") {
		return ".webm"
	}
	return ".mp4"
}

// Miss keeps that the album of key has no clip, so that its source isn't
// asked again.
func (s *Store) Miss(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	if old, ok := s.index[key]; ok && !old.Miss() {
		os.Remove(filepath.Join(s.dir, old.File))
	}
	s.index[key] = Match{Found: s.now()}
	s.trimMisses()
	s.save()
}

// Forget forgets what the store kept of the album of key, its clip too:
// its source is asked again, as Find the moving cover again does, and a
// kept clip that failed to play goes so.
func (s *Store) Forget(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	old, ok := s.index[key]
	if !ok {
		return
	}
	if !old.Miss() {
		os.Remove(filepath.Join(s.dir, old.File))
	}
	delete(s.index, key)
	s.save()
}

// Played tells the store the clip of the album of key played, which keeps
// it from the clips that go first.
func (s *Store) Played(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	if m, ok := s.index[key]; ok && !m.Miss() {
		m.Played = s.now()
		s.index[key] = m
		s.save()
	}
}

// SetLimit bounds the bytes of the clips kept: 0 keeps the one kept last
// alone, and a limit below 0 bounds them not. Those past it go.
func (s *Store) SetLimit(limit int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.limit == limit {
		return
	}
	s.limit = limit
	s.load()
	if s.trim() {
		s.save()
	}
}

// trim removes the clips played least recently past the limit, but for
// the one kept last, and reports whether it removed any.
func (s *Store) trim() bool {
	if s.limit < 0 {
		return false
	}
	var used int64
	var keys []string
	for key, m := range s.index {
		if !m.Miss() {
			used += m.Bytes
			keys = append(keys, key)
		}
	}
	slices.SortFunc(keys, func(a, b string) int { return s.index[a].Played.Compare(s.index[b].Played) })
	removed := false
	for _, key := range keys {
		if used <= s.limit {
			break
		}
		if key == s.last {
			continue
		}
		m := s.index[key]
		os.Remove(filepath.Join(s.dir, m.File))
		delete(s.index, key)
		used -= m.Bytes
		removed = true
	}
	return removed
}

// trimMisses forgets the oldest misses past maxMisses.
func (s *Store) trimMisses() {
	var misses []string
	for key, m := range s.index {
		if m.Miss() {
			misses = append(misses, key)
		}
	}
	if len(misses) <= maxMisses {
		return
	}
	slices.SortFunc(misses, func(a, b string) int {
		return cmp.Or(s.index[a].Found.Compare(s.index[b].Found), strings.Compare(a, b))
	})
	for _, key := range misses[:len(misses)-maxMisses] {
		delete(s.index, key)
	}
}

// Usage returns the bytes of the clips kept, and how many there are.
func (s *Store) Usage() (bytes int64, clips int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	for _, m := range s.index {
		if !m.Miss() {
			bytes += m.Bytes
			clips++
		}
	}
	return bytes, clips
}

// Clear removes the clips kept, and forgets the albums' matches, misses
// too: each is looked for again.
func (s *Store) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	s.index, s.last = map[string]Match{}, ""
	if err := os.RemoveAll(s.dir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
