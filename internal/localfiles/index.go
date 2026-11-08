// Package localfiles keeps the index of the music on the user's computer:
// the files of the folders the settings name, with what their tags tell,
// which MixMusic plays from the files themselves. The index is kept in
// the data folder (local-files.json), and each scan reads only the files
// whose size or time of change changed since the scan before; the covers
// the files hold are kept in a folder of the cache, never beside the
// music. Nothing of it leaves the computer.
package localfiles

import (
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/fzn0x/mixmusic/internal/store"
	"github.com/fzn0x/mixmusic/internal/tags"
)

// version is the version of the index file: one of another is read again
// from the files. 2 reads the lengths of VBR MP3s from their frame counts
// (#370), which the files indexed before held wrong.
const version = 2

// Entry is a file of the index: where it is, its size and time of change,
// which tell a scan whether it changed, and what its tags tell.
type Entry struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
	// Modified is the file's time of change, in nanoseconds since 1970.
	Modified     int64    `json:"modified"`
	Title        string   `json:"title,omitempty"`
	Artists      []string `json:"artists,omitempty"`
	Album        string   `json:"album,omitempty"`
	AlbumArtists []string `json:"albumArtists,omitempty"`
	Track        int      `json:"track,omitempty"`
	Disc         int      `json:"disc,omitempty"`
	Year         int      `json:"year,omitempty"`
	DurationMs   int      `json:"durationMs,omitempty"`
	// Art is the file of the cover in the folder of covers, "" without
	// one.
	Art string `json:"art,omitempty"`
	// Skipped is why MixMusic can't play the file, "" when it can.
	Skipped string `json:"skipped,omitempty"`
}

// saved is what the index file holds.
type saved struct {
	Version int     `json:"version"`
	Files   []Entry `json:"files"`
}

// Folder is a folder of the index, as the last scan found it.
type Folder struct {
	Path string
	// Tracks counts the files MixMusic plays, and Skipped those it can't.
	Tracks, Skipped int
	// Problem is why the folder can't be read, as when it is on a drive
	// that isn't there: "" when it can. Its tracks stay meanwhile.
	Problem string
}

// Status is what the index holds and does.
type Status struct {
	Folders []Folder
	// Scanning is set while a scan runs, which has read Read files of
	// Found so far.
	Scanning    bool
	Read, Found int
	// Tracks counts the files MixMusic plays, of every folder.
	Tracks int
}

// Index is the index of the music on the computer. It is safe for
// concurrent use.
type Index struct {
	file *store.File[saved]
	art  string
	// probe and readTags read a file, as audio.Probe and tags.Read do;
	// tests give their own.
	probe    func(io.ReadSeeker) (time.Duration, error)
	readTags func(io.ReadSeeker) (tags.Tags, error)
	now      func() time.Time

	mu      sync.Mutex
	entries map[string]Entry // by path
	folders []Folder
	// scanning is set while a scan runs; again when one more was asked for
	// meanwhile, to the folders of next.
	scanning    bool
	again       bool
	next        []string
	read, found int
	// work is held by a scan, and by an update of what the watch of the
	// folders told changed, so that one changes the entries at a time.
	work sync.Mutex
	// gone are the files found missing since the last scan, as playing them
	// did, and stats what the files beyond the index looked like lately.
	gone  map[string]bool
	stats map[string]stat
	// version counts the changes to the entries, which views is of.
	version uint64
	views   *views
	watch   []*func()
}

// stat is whether a file was there, and when it was looked at.
type stat struct {
	there bool
	at    time.Time
}

// Open opens the index kept at path, whose covers are kept in the folder
// art. probe tells how long a file plays, or why it can't, as audio.Probe
// does.
func Open(path, art string, probe func(io.ReadSeeker) (time.Duration, error)) (*Index, error) {
	f, err := store.Open(path, saved{Version: version})
	if err != nil {
		return nil, err
	}
	x := &Index{file: f, art: art, probe: probe, readTags: tags.Read, now: time.Now,
		entries: map[string]Entry{}, gone: map[string]bool{}, stats: map[string]stat{}}
	if s := f.Get(); s.Version == version {
		for _, e := range s.Files {
			if filepath.IsAbs(e.Path) {
				x.entries[e.Path] = e
			}
		}
	}
	return x, nil
}

// Watch calls fn after each change to what the index holds or does, until
// the returned function is called.
func (x *Index) Watch(fn func()) (stop func()) {
	x.mu.Lock()
	defer x.mu.Unlock()
	p := &fn
	x.watch = append(x.watch, p)
	return func() {
		x.mu.Lock()
		defer x.mu.Unlock()
		x.watch = slices.DeleteFunc(x.watch, func(q *func()) bool { return q == p })
	}
}

// changed calls the watchers, outside the lock.
func (x *Index) changed() {
	x.mu.Lock()
	fns := slices.Clone(x.watch)
	x.mu.Unlock()
	for _, fn := range fns {
		(*fn)()
	}
}

// Status returns what the index holds and does.
func (x *Index) Status() Status {
	x.mu.Lock()
	defer x.mu.Unlock()
	st := Status{Folders: slices.Clone(x.folders), Scanning: x.scanning, Read: x.read, Found: x.found}
	for _, e := range x.entries {
		if e.Skipped == "" {
			st.Tracks++
		}
	}
	return st
}

// Version counts the changes to what the index holds, which a window's
// views are of.
func (x *Index) Version() uint64 {
	x.mu.Lock()
	defer x.mu.Unlock()
	return x.version
}

// Entry returns the entry of the file at path.
func (x *Index) Entry(path string) (Entry, bool) {
	x.mu.Lock()
	defer x.mu.Unlock()
	e, ok := x.entries[filepath.Clean(path)]
	return e, ok
}

// Missing reports whether the file of a local track isn't there: one the
// last scan found that playing found gone since, or, beyond the index, as
// of a mix, one that isn't there when looked at, at most every 30 seconds.
func (x *Index) Missing(id string) bool {
	path, ok := PathOf(id)
	if !ok {
		return false
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	if x.gone[path] {
		return true
	}
	if e, ok := x.entries[path]; ok && e.Skipped == "" {
		// The files of a folder that can't be read, as on a drive
		// unplugged, stay in the index, and are missing meanwhile.
		for _, f := range x.folders {
			if f.Problem != "" && within(path, f.Path) {
				return true
			}
		}
		return false
	}
	now := x.now()
	if s, ok := x.stats[path]; ok && now.Sub(s.at) < 30*time.Second {
		return !s.there
	}
	_, err := os.Stat(path)
	x.stats[path] = stat{there: err == nil, at: now}
	return err != nil
}

// Gone takes in that the file of a local track was found missing, as it
// was to play: it shows so until a scan, or Found, finds it.
func (x *Index) Gone(id string) {
	path, ok := PathOf(id)
	if !ok {
		return
	}
	x.mu.Lock()
	changed := !x.gone[path]
	x.gone[path] = true
	if changed {
		x.version++
	}
	x.mu.Unlock()
	if changed {
		x.changed()
	}
}

// Found takes in that the file of a local track is there, as it played.
func (x *Index) Found(id string) {
	path, ok := PathOf(id)
	if !ok {
		return
	}
	x.mu.Lock()
	changed := x.gone[path]
	delete(x.gone, path)
	delete(x.stats, path)
	if changed {
		x.version++
	}
	x.mu.Unlock()
	if changed {
		x.changed()
	}
}

// save keeps the entries in the index file.
func (x *Index) save() error {
	x.mu.Lock()
	s := saved{Version: version, Files: make([]Entry, 0, len(x.entries))}
	for _, e := range x.entries {
		s.Files = append(s.Files, e)
	}
	x.mu.Unlock()
	slices.SortFunc(s.Files, func(a, b Entry) int { return strings.Compare(a.Path, b.Path) })
	return x.file.Set(s)
}
