package localfiles

import (
	"context"
	"errors"
	"io/fs"
	"log"
	"os"
	"path/filepath"
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
// folder it can't read, as on a drive unplugged. A scan asked for while
// one runs runs after it, of the folders asked for last.
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
		err := x.scanAlone(ctx, folders)
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

// found is a file a scan found, and what the folder it's in told of it.
type found struct {
	path string
	info fs.FileInfo
}

func (x *Index) scan(ctx context.Context, folders []string) error {
	var files []found
	status := make([]Folder, len(folders))
	reachable := map[string]bool{}
	for i, root := range folders {
		status[i].Path = root
		if _, err := os.Stat(root); err != nil {
			status[i].Problem = problemOf(err)
			continue
		}
		reachable[root] = true
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err != nil {
				return nil // a folder it can't read: what else it holds
			}
			if d.IsDir() {
				if path != root && strings.HasPrefix(d.Name(), ".") {
					return filepath.SkipDir
				}
				return nil
			}
			if !d.Type().IsRegular() || !Plays(path) {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			files = append(files, found{path, info})
			x.mu.Lock()
			x.found++
			x.mu.Unlock()
			return nil
		})
		if err != nil && ctx.Err() == nil {
			status[i].Problem = problemOf(err)
		}
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	x.changed()

	// The files read again: those new or changed, and those whose cover
	// the cache lost.
	x.mu.Lock()
	old := x.entries
	x.mu.Unlock()
	seen := map[string]bool{}
	var todo []found
	next := map[string]Entry{}
	for _, f := range files {
		seen[f.path] = true
		e, ok := old[f.path]
		if ok && e.Size == f.info.Size() && e.Modified == f.info.ModTime().UnixNano() && x.hasArt(e.Art) {
			next[f.path] = e
			continue
		}
		todo = append(todo, f)
	}
	read := x.readAll(ctx, todo)
	for path, e := range read {
		next[path] = e
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	// The files of folders that can't be read stay, as they were.
	for path, e := range old {
		if seen[path] {
			continue
		}
		for _, root := range folders {
			if !reachable[root] && within(path, root) {
				next[path] = e
			}
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
