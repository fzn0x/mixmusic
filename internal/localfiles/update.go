package localfiles

import (
	"context"
	"errors"
	"io/fs"
	"log"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/fzn0x/mixmusic/internal/settings"
)

// The watch of the folders (watch.go) tells which paths changed, and
// update reads only those again, as a scan would read them: a file new or
// changed, the files of a folder made or renamed to, and none of those
// that went. A scan and an update don't change the entries at once: each
// holds work.

// scanAlone scans the folders while no update runs.
func (x *Index) scanAlone(ctx context.Context, folders []string) error {
	x.work.Lock()
	defer x.work.Unlock()
	return x.scan(ctx, folders)
}

// update reads again what changed at the paths of changed, each true when
// it was made, renamed or removed, so that a folder there is read whole,
// and false when it was only written: the file there, when it's new to the
// index or its size or time of change changed, and the music files of a
// folder made or renamed to, but those of hidden folders, as a scan reads
// them. It drops the files that went, and those of a folder that went,
// but keeps those of a folder it can't read. Paths beyond the folders the
// last scan read are passed over. It reports whether a folder of music
// itself changed, as when it was renamed or removed, its drive unplugged,
// or it came back, which only a scan tells of.
func (x *Index) update(ctx context.Context, changed map[string]bool) (rescan bool) {
	x.work.Lock()
	defer x.work.Unlock()
	if ctx.Err() != nil {
		return false
	}
	x.mu.Lock()
	old, folders := x.entries, slices.Clone(x.folders)
	x.mu.Unlock()
	if len(folders) == 0 {
		return false // no scan yet: the first reads them
	}
	var (
		files []found
		// gone are the paths whose files went, with what was below
		// them; walked the folders read whole; and unread the files and
		// folders that couldn't be read, whose entries stay as they were.
		gone, walked, unread = map[string]bool{}, map[string]bool{}, map[string]bool{}
	)
	for _, p := range slices.Sorted(maps.Keys(changed)) {
		if ctx.Err() != nil {
			return false
		}
		i := slices.IndexFunc(folders, func(f Folder) bool { return settings.SamePath(p, f.Path) || within(p, f.Path) })
		switch {
		case i < 0 || under(p, walked):
			continue // beyond the folders, or read with its folder
		case settings.SamePath(p, folders[i].Path):
			// The folder of music itself: renamed or removed, or only
			// written in, as its files tell.
			rescan = rescan || changed[p]
			continue
		case folders[i].Problem != "":
			rescan = true // back, as the last scan couldn't read it
			continue
		case hiddenIn(folders[i].Path, p):
			continue
		}
		info, err := os.Lstat(p)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			if _, err := os.Stat(folders[i].Path); err != nil {
				rescan = true // the folder of music went, or its drive
				continue
			}
			gone[p] = true
		case err != nil:
			unread[p] = true
		case info.Mode().IsRegular() && Plays(p):
			files = append(files, found{path: p, info: info})
		case info.IsDir() && !changed[p]:
			// Written in: its files tell what changed.
		case info.IsDir() && !strings.HasPrefix(info.Name(), "."):
			in, failed := walkDir(ctx, p)
			files = append(files, in...)
			for _, dir := range failed {
				unread[dir] = true
			}
			walked[p] = true
		default:
			// A hidden folder, a link, or a file of a kind the library
			// doesn't take: what the index held there goes.
			gone[p] = true
		}
	}

	seen := map[string]bool{}
	var todo []found
	for _, f := range files {
		seen[f.path] = true
		e, ok := old[f.path]
		if !ok || e.Size != f.info.Size() || e.Modified != f.info.ModTime().UnixNano() || !x.hasArt(e.Art) {
			todo = append(todo, f)
		}
	}
	next := maps.Clone(old)
	dirty := false
	for path := range old {
		if seen[path] || under(path, unread) {
			continue
		}
		if under(path, gone) || under(path, walked) {
			delete(next, path)
			dirty = true
		}
	}
	read := x.readAll(ctx, todo)
	if ctx.Err() != nil {
		return false
	}
	for path, e := range read {
		next[path] = e
		dirty = true
	}
	if !dirty {
		return rescan
	}
	for i := range folders {
		folders[i].Tracks, folders[i].Skipped = 0, 0
		for path, e := range next {
			if within(path, folders[i].Path) {
				if e.Skipped == "" {
					folders[i].Tracks++
				} else {
					folders[i].Skipped++
				}
			}
		}
	}
	x.mu.Lock()
	x.entries, x.folders = next, folders
	for path := range read {
		delete(x.gone, path)
		delete(x.stats, path)
	}
	x.version++
	x.mu.Unlock()
	if err := x.save(); err != nil {
		log.Printf("mixmusic: keeping the index of the music on this computer: %v", err)
	}
	x.changed()
	return rescan
}

// walkDir finds the music files of dir and of the folders in it, but
// hidden ones, as a scan does, and the folders in it and files it couldn't
// read, whose entries stay as they were: those gone as it came to them
// hold nothing.
func walkDir(ctx context.Context, dir string) (files []found, unread []string) {
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		switch {
		case err != nil && errors.Is(err, fs.ErrNotExist):
			return nil
		case err != nil:
			unread = append(unread, path)
			return nil // what else it holds
		case d.IsDir():
			if path != dir && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		case !d.Type().IsRegular() || !Plays(path):
			return nil
		}
		info, err := d.Info()
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				unread = append(unread, path)
			}
			return nil
		}
		files = append(files, found{path: path, info: info})
		return nil
	})
	return files, unread
}

// under reports whether path, or a folder it's in, is one of set.
func under(path string, set map[string]bool) bool {
	if len(set) == 0 {
		return false
	}
	for {
		if set[path] {
			return true
		}
		parent := filepath.Dir(path)
		if parent == path {
			return false
		}
		path = parent
	}
}

// hiddenIn reports whether path is in a hidden folder of root, one whose
// name starts with a dot, which a scan passes over.
func hiddenIn(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	parts := strings.Split(rel, string(filepath.Separator))
	return slices.ContainsFunc(parts[:len(parts)-1], func(p string) bool { return strings.HasPrefix(p, ".") })
}
