package localfiles

import (
	"context"
	"errors"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/fzn0x/mixmusic/internal/settings"
)

// extensions are the kinds of files a scan reads, by their extensions:
// those of the formats the engine plays.
var extensions = map[string]bool{
	".mp3": true, ".flac": true, ".ogg": true, ".oga": true, ".opus": true,
	".m4a": true, ".mp4": true, ".wav": true, ".wave": true,
}

// Plays reports whether a file's name is of a kind the library takes.
func Plays(path string) bool { return extensions[strings.ToLower(filepath.Ext(path))] }

// workers is how many files a scan reads at once.
const workers = 4

// Scan reads the folders, and the files in them, into the index: those
// new to it, and those whose size or time of change changed, which their
// tags and their audio are read of again. It drops the files that are
// gone, and those of folders no longer named, but keeps the files of a
// folder it can't read, as on a drive unplugged, or one in a folder that
// MixMusic isn't allowed to read, until a scan reads it. A scan asked for
// while one runs runs after it, of the folders asked for last.
func (x *Index) Scan(ctx context.Context, folders []string) error {
	x.mu.Lock()
	if x.scanning {
		x.again, x.next = true, folders
		x.mu.Unlock()
		return nil
	}
	x.scanning, x.read, x.found = true, 0, 0
	x.mu.Unlock()
	x.changed()
	for {
		err := x.scan(ctx, folders)
		x.mu.Lock()
		again := x.again && ctx.Err() == nil
		folders, x.again, x.next = x.next, false, nil
		if !again {
			x.scanning = false
		}
		x.mu.Unlock()
		x.changed()
		if !again {
			return err
		}
	}
}

// found is a file a scan found, the music folder it's in, of the folders
// scanned, and what the folder it's in told of it.
type found struct {
	path string
	root int
	info fs.FileInfo
}

func (x *Index) scan(ctx context.Context, folders []string) error {
	var files []found
	status := make([]Folder, len(folders))
	why := make([]error, len(folders))
	for i, root := range folders {
		status[i].Path = root
		if _, err := os.Stat(root); err != nil {
			status[i].Problem = problemOf(err)
			continue
		}
		w, err := x.walkFolder(ctx, i, root)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			status[i].Problem = problemOf(err)
			x.mu.Lock()
			x.found -= len(w.files)
			x.mu.Unlock()
			continue
		}
		files = append(files, w.files...)
		status[i].Unread, why[i] = w.unread, w.why
	}
	x.changed()

	// The files read again: those new or changed, and those whose cover
	// the cache lost.
	x.mu.Lock()
	old := x.entries
	x.mu.Unlock()
	var todo []found
	for _, f := range files {
		e, ok := old[f.path]
		if !ok || e.Size != f.info.Size() || e.Modified != f.info.ModTime().UnixNano() || !x.hasArt(e.Art) {
			todo = append(todo, f)
		}
	}
	read := x.readAll(ctx, todo)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	// A drive unplugged while the scan read it isn't there at its end:
	// what the scan found of it doesn't count.
	for i := range status {
		if status[i].Problem != "" {
			continue
		}
		if _, err := os.Stat(status[i].Path); err != nil {
			status[i].Problem, status[i].Unread = problemOf(err), nil
		}
	}
	seen := map[string]bool{}
	next := map[string]Entry{}
	for _, f := range files {
		if status[f.root].Problem != "" {
			continue
		}
		seen[f.path] = true
		if e, ok := read[f.path]; ok {
			next[f.path] = e
		} else if e, ok := old[f.path]; ok {
			next[f.path] = e
		}
	}
	// The files of folders that can't be read stay, as they were: they
	// aren't gone until a scan reads their folder.
	for path, e := range old {
		if !seen[path] && kept(path, status) {
			next[path] = e
		}
	}
	for i, f := range status {
		if len(f.Unread) > 0 {
			log.Printf("mixmusic: scanning the music on this computer: %d of the folders in %s can't be read, as %v", len(f.Unread), f.Path, why[i])
		}
	}
	for i := range status {
		for path, e := range next {
			if within(path, status[i].Path) {
				if e.Skipped == "" {
					status[i].Tracks++
				} else {
					status[i].Skipped++
				}
			}
		}
	}
	x.mu.Lock()
	x.entries, x.folders = next, status
	x.gone, x.stats = map[string]bool{}, map[string]stat{}
	x.version++
	x.mu.Unlock()
	if err := x.save(); err != nil {
		log.Printf("mixmusic: keeping the index of the music on this computer: %v", err)
	}
	return nil
}

// walked is what a scan found in a music folder: its music files, and the
// folders in it that it couldn't read, of the first of which why tells
// why.
type walked struct {
	files  []found
	unread []string
	why    error
}

// walkFolder finds the music files of root, the folder scanned i, and of
// the folders in it, but hidden ones. A folder in it that can't be read,
// as one MixMusic isn't allowed to, or one that holds a file that can't
// be looked at, is unread; one that's gone holds nothing. err is why root
// itself can't be read.
func (x *Index) walkFolder(ctx context.Context, i int, root string) (w walked, err error) {
	walk := x.walk
	if walk == nil {
		walk = filepath.WalkDir
	}
	unread := map[string]bool{}
	failed := func(dir string, err error) {
		if !unread[dir] {
			unread[dir] = true
			w.unread = append(w.unread, dir)
		}
		if w.why == nil {
			w.why = err
		}
	}
	var rootErr error
	err = walk(root, func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		switch {
		case err != nil && path == root:
			rootErr = err
			return nil
		case err != nil && errors.Is(err, fs.ErrNotExist):
			return nil // gone as the scan came to it
		case err != nil:
			failed(path, err)
			return nil // what else it holds
		case d.IsDir():
			if path != root && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		case !d.Type().IsRegular() || !Plays(path):
			return nil
		}
		info, err := d.Info()
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				failed(filepath.Dir(path), err)
			}
			return nil
		}
		w.files = append(w.files, found{path, i, info})
		x.mu.Lock()
		x.found++
		x.mu.Unlock()
		return nil
	})
	if err == nil {
		err = rootErr
	}
	slices.Sort(w.unread)
	return w, err
}

// kept reports whether the entry of a file a scan didn't find stays, as it
// was: one in a music folder that can't be read, or in a folder in one
// that the scan couldn't read.
func kept(path string, status []Folder) bool {
	for _, f := range status {
		if !within(path, f.Path) {
			continue
		}
		if f.Problem != "" || slices.ContainsFunc(f.Unread, func(dir string) bool { return within(path, dir) }) {
			return true
		}
	}
	return false
}

// readAll reads the files, a few at a time.
func (x *Index) readAll(ctx context.Context, files []found) map[string]Entry {
	out := make(map[string]Entry, len(files))
	var mu sync.Mutex
	work := make(chan found)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for f := range work {
				e := x.readFile(f.path, f.info)
				mu.Lock()
				out[f.path] = e
				mu.Unlock()
				x.mu.Lock()
				x.read++
				n := x.read
				x.mu.Unlock()
				if n%50 == 0 {
					x.changed()
				}
			}
		}()
	}
	for _, f := range files {
		select {
		case work <- f:
		case <-ctx.Done():
		}
		if ctx.Err() != nil {
			break
		}
	}
	close(work)
	wg.Wait()
	return out
}

// within reports whether path is in the folder root, or below it.
func within(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return false
	}
	return !settings.SamePath(filepath.Join(root, rel), root)
}

// problemOf words why a folder can't be read.
func problemOf(err error) string {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "It isn't there: is its drive plugged in?"
	case errors.Is(err, fs.ErrPermission):
		return "MixMusic isn't allowed to read it"
	}
	return "It can't be read"
}
