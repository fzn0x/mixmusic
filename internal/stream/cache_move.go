package stream

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// cacheRename renames a cached file; tests can replace it to exercise
// cross-volume copy fallback.
var cacheRename = os.Rename

// cacheMoveFolder is the subfolder Move makes when moving the cache into a
// directory that already holds other files.
const cacheMoveFolder = "MixMusic cache"

// Why a move of the offline cache cannot start.
var (
	// ErrCacheMoving: the cache is moving already.
	ErrCacheMoving = errors.New("the offline cache is moving already")
	// ErrCacheSameFolder: the cache is in that folder already.
	ErrCacheSameFolder = errors.New("the offline cache is in that folder already")
	// ErrCacheIntoTheirs: the target folder is inside the current cache folder.
	ErrCacheIntoTheirs = errors.New("the offline cache can't move into a folder of its own")
	// ErrCacheConflict: the target folder conflicts with another MixMusic folder.
	ErrCacheConflict = errors.New("the offline cache can't share that folder")
)

// CacheWriteError reports why the cache cannot move to Folder.
type CacheWriteError struct {
	Folder string
	Err    error
}

func (e *CacheWriteError) Error() string {
	return "MixMusic can't write to " + e.Folder + ": " + e.Err.Error()
}

func (e *CacheWriteError) Unwrap() error { return e.Err }

// MoveOptions configures (*Cache).Move.
type MoveOptions struct {
	// Exact uses dir as the destination directly instead of creating a
	// MixMusic cache subfolder when dir holds other files.
	Exact bool
	// RootDir is the directory checked by Problem() and Begin() when
	// MustExist is true. Defaults to the final target directory.
	RootDir string
	// MustExist requires RootDir to exist when caching new tracks.
	MustExist bool
	// Avoid lists directories the cache must not overlap with.
	Avoid []string
	// Progress receives moved byte progress updates.
	Progress func(done, total int64)
}

// Move moves the cache's owned audio files to dir (or a MixMusic cache
// subfolder when dir holds unrelated files and !opts.Exact), updates the
// cache's active directory, and returns the final directory path.
func (c *Cache) Move(ctx context.Context, dir string, opts MoveOptions) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	target := abs
	if !opts.Exact && !emptyOrCacheDir(abs) {
		target = filepath.Join(abs, cacheMoveFolder)
	}

	c.mu.Lock()
	cur := c.dir
	switch {
	case c.moving:
		c.mu.Unlock()
		return "", ErrCacheMoving
	case cur != "" && sameCachePath(target, cur):
		c.mu.Unlock()
		return "", ErrCacheSameFolder
	case cur != "" && insideDir(target, cur):
		c.mu.Unlock()
		return "", ErrCacheIntoTheirs
	}
	for _, avoid := range opts.Avoid {
		if avoid == "" {
			continue
		}
		if sameCachePath(target, avoid) || insideDir(target, avoid) || insideDir(avoid, target) {
			c.mu.Unlock()
			return "", ErrCacheConflict
		}
	}
	c.moving = true
	c.mu.Unlock()

	release := func() {
		c.mu.Lock()
		c.moving = false
		c.mu.Unlock()
	}

	if err := cacheWritable(target); err != nil {
		release()
		return "", &CacheWriteError{Folder: target, Err: err}
	}

	files := c.files()
	mv, err := moveCacheFiles(ctx, files, target, opts.Progress)
	if err != nil {
		mv.undo()
		release()
		return "", err
	}

	rootDir := opts.RootDir
	if rootDir == "" {
		rootDir = target
	}
	c.mu.Lock()
	c.dir = target
	c.rootDir = rootDir
	c.mustExist = opts.MustExist
	c.moving = false
	c.mu.Unlock()

	for _, p := range mv.leave {
		if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
			log.Printf("mixmusic: removing moved cache file: %v", err)
		}
	}
	if cur != "" {
		os.Remove(cur) // succeeds only when empty
	}
	c.prune()
	return target, nil
}

// emptyOrCacheDir reports whether dir does not exist yet, is empty, or
// contains only valid cached audio files.
func emptyOrCacheDir(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return errors.Is(err, fs.ErrNotExist)
	}
	for _, e := range entries {
		if e.IsDir() || !isCacheFile(e.Name()) {
			return false
		}
	}
	return true
}

// cacheWritable ensures dir exists and can be written to.
func cacheWritable(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".mixcache-*.tmp")
	if err != nil {
		return err
	}
	f.Close()
	return os.Remove(f.Name())
}

// sameCachePath reports whether two paths refer to the same location.
func sameCachePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	a, b = filepath.Clean(a), filepath.Clean(b)
	if strings.EqualFold(a, b) && (os.PathSeparator == '\\' || a == b) {
		return true
	}
	ia, err1 := os.Stat(a)
	ib, err2 := os.Stat(b)
	return err1 == nil && err2 == nil && os.SameFile(ia, ib)
}

// insideDir reports whether a is a path inside b.
func insideDir(a, b string) bool {
	rel, err := filepath.Rel(b, a)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

type cacheMoved struct {
	renamed [][2]string
	copied  []string
	leave   []string
}

func (mv cacheMoved) undo() {
	for _, r := range slices.Backward(mv.renamed) {
		if err := os.Rename(r[1], r[0]); err != nil {
			log.Printf("mixmusic: putting cache file back: %v", err)
		}
	}
	for _, p := range mv.copied {
		os.Remove(p)
	}
}

func moveCacheFiles(ctx context.Context, files []cachedFile, target string, progress func(done, total int64)) (cacheMoved, error) {
	var mv cacheMoved
	var total int64
	for _, f := range files {
		total += f.size
	}
	var done int64
	tell := func() {
		if progress != nil {
			progress(done, total)
		}
	}
	tell()
	for _, f := range files {
		if err := ctx.Err(); err != nil {
			return mv, err
		}
		dst := filepath.Join(target, filepath.Base(f.path))
		if _, err := os.Stat(dst); err == nil {
			mv.leave = append(mv.leave, f.path)
		} else if err := cacheRename(f.path, dst); err == nil {
			mv.renamed = append(mv.renamed, [2]string{f.path, dst})
		} else if err := copyCacheFile(ctx, f.path, dst, func(n int64) { done += n; tell() }); err != nil {
			return mv, err
		} else {
			mv.copied = append(mv.copied, dst)
			mv.leave = append(mv.leave, f.path)
			continue
		}
		done += f.size
		tell()
	}
	return mv, nil
}

func copyCacheFile(ctx context.Context, src, dst string, wrote func(n int64)) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.CreateTemp(filepath.Dir(dst), "."+filepath.Base(dst)+".*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(out.Name())
	buf := make([]byte, 1<<20)
	for {
		if err := ctx.Err(); err != nil {
			out.Close()
			return err
		}
		n, rerr := in.Read(buf)
		if n > 0 {
			if _, err := out.Write(buf[:n]); err != nil {
				out.Close()
				return err
			}
			wrote(int64(n))
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			out.Close()
			return rerr
		}
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(out.Name(), dst)
}
