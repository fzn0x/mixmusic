package stream

import (
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

// Unlimited is the cache byte limit that keeps every cached track without
// eviction.
const Unlimited int64 = -1

// Cache keeps the audio of tracks played to their end on disk, up to a
// size, so that playing them again needs no network. The tracks played
// least recently go first.
type Cache struct {
	mu        sync.Mutex
	dir       string
	rootDir   string
	mustExist bool
	moving    bool
	limit     int64
	protected func(dir, name string) bool
}

// extensions maps content types to the extensions cached files get, which
// tell the content type back when they are served.
var extensions = map[string]string{
	"audio/webm": ".webm",
	"audio/mp4":  ".m4a",
	"audio/mpeg": ".mp3",
}

// NewCache returns a cache in dir of at most limit bytes; 0 caches nothing,
// and Unlimited keeps every track.
func NewCache(dir string, limit int64) *Cache {
	return &Cache{dir: dir, rootDir: dir, limit: limit}
}

// Folder returns the directory currently holding cached audio.
func (c *Cache) Folder() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.dir
}

// SetDir switches the directory where cached audio is kept. When mustExist
// is set, Begin refuses to create rootDir if it is missing.
func (c *Cache) SetDir(dir, rootDir string, mustExist bool) {
	if rootDir == "" {
		rootDir = dir
	}
	c.mu.Lock()
	c.dir = dir
	c.rootDir = rootDir
	c.mustExist = mustExist
	c.mu.Unlock()
}

// SetProtected sets a predicate that reports whether a file name inside dir
// is owned externally (such as by Downloads) and must not be counted,
// pruned, cleared, or moved out.
func (c *Cache) SetProtected(fn func(dir, name string) bool) {
	c.mu.Lock()
	c.protected = fn
	c.mu.Unlock()
}

func (c *Cache) isProtected(dir, name string) bool {
	c.mu.Lock()
	fn := c.protected
	c.mu.Unlock()
	return fn != nil && fn(dir, name)
}

// BeginExternalMove marks the cache as moving under an external coordinator
// (such as Downloads moving a shared directory). It returns false if a move
// is already in progress.
func (c *Cache) BeginExternalMove() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.moving {
		return false
	}
	c.moving = true
	return true
}

// EndExternalMove completes an external move. If dir is non-empty, the
// cache's location is updated to it and pruned to the current limit.
func (c *Cache) EndExternalMove(dir, rootDir string, mustExist bool) {
	c.mu.Lock()
	if dir != "" {
		if rootDir == "" {
			rootDir = dir
		}
		c.dir = dir
		c.rootDir = rootDir
		c.mustExist = mustExist
	}
	c.moving = false
	c.mu.Unlock()
	if dir != "" {
		c.prune()
	}
}

// Problem returns a plain description if the configured cache root folder
// must exist and cannot be reached, or "" otherwise.
func (c *Cache) Problem() string {
	if u := c.Unreachable(); u != "" {
		return "The folder of the offline cache can't be reached: " + u
	}
	return ""
}

// Unreachable returns the configured cache root folder when it must exist
// and cannot be reached, or "" otherwise.
func (c *Cache) Unreachable() string {
	c.mu.Lock()
	rootDir := c.rootDir
	mustExist := c.mustExist
	c.mu.Unlock()
	if !mustExist || rootDir == "" {
		return ""
	}
	if info, err := os.Stat(rootDir); err != nil || !info.IsDir() {
		return rootDir
	}
	return ""
}

// SetLimit changes the size of the cache, removing what no longer fits.
func (c *Cache) SetLimit(limit int64) {
	c.mu.Lock()
	c.limit = limit
	c.mu.Unlock()
	c.prune()
}

func (c *Cache) limitBytes() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.limit
}

// find returns the cached file of a video, if there is one.
func (c *Cache) find(videoID string) (string, string, bool) {
	dir := c.Folder()
	if dir == "" {
		return "", "", false
	}
	for mime, ext := range extensions {
		p := filepath.Join(dir, videoID+ext)
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p, mime, true
		}
	}
	return "", "", false
}

// Has reports whether the cache holds the audio of a video, which then
// plays without the network.
func (c *Cache) Has(videoID string) bool {
	_, _, ok := c.find(videoID)
	return ok
}

// Find returns the cached file of a video, and its content type, if the
// cache holds one.
func (c *Cache) Find(videoID string) (path, mime string, ok bool) { return c.find(videoID) }

// Take moves the cached audio of a video to dst, out of the cache, and
// reports whether the cache held it: a download of a track the cache holds
// costs no network. If the file is already at dst (as when sharing a folder
// with downloads), it stays in place. A file it can't move, as on another
// volume, or open on Windows, it copies, leaving the cached one to the
// cache.
func (c *Cache) Take(videoID, dst string) (bool, error) {
	path, _, ok := c.find(videoID)
	if !ok {
		return false, nil
	}
	if sameCachePath(path, dst) {
		return true, nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return true, err
	}
	if os.Rename(path, dst) == nil {
		return true, nil
	}
	return true, copyFile(path, dst)
}

// copyFile copies src to dst, through a file beside dst that takes its
// name once whole.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.CreateTemp(filepath.Dir(dst), "."+filepath.Base(dst)+".*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(out.Name()) // a no-op once renamed
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
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

// Serve answers r with the cached audio of a video, and reports whether it
// had it.
func (c *Cache) Serve(w http.ResponseWriter, r *http.Request, videoID string) bool {
	path, mime, ok := c.find(videoID)
	if !ok {
		return false
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	now := time.Now()
	os.Chtimes(path, now, now) // played again: the last to go
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, "", time.Time{}, f)
	return true
}

// Begin starts keeping the audio of a video as it streams, or returns nil
// when the cache is off, moving, unreachable, or cannot take it.
func (c *Cache) Begin(videoID, mime string, size int64) *capture {
	c.mu.Lock()
	dir, rootDir, mustExist, limit, moving := c.dir, c.rootDir, c.mustExist, c.limit, c.moving
	c.mu.Unlock()

	base, _, _ := strings.Cut(mime, ";")
	ext, known := extensions[strings.TrimSpace(base)]
	if moving || dir == "" || limit == 0 || !known || size <= 0 || (limit > 0 && size > limit) {
		return nil
	}
	if mustExist {
		if info, err := os.Stat(rootDir); err != nil || !info.IsDir() {
			return nil
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil
	}
	f, err := os.CreateTemp(dir, ".mixcache-"+videoID+"-*.tmp")
	if err != nil {
		return nil
	}
	return &capture{cache: c, f: f, final: filepath.Join(dir, videoID+ext), size: size}
}

// capture writes a stream into the cache as it is served. Its Write never
// fails, so that a full disk never interrupts playback.
type capture struct {
	cache   *Cache
	f       *os.File
	final   string
	size    int64
	written int64
	failed  bool
	done    bool
}

func (c *capture) Write(p []byte) (int, error) {
	if !c.failed {
		n, err := c.f.Write(p)
		c.written += int64(n)
		c.failed = err != nil
	}
	return len(p), nil
}

// Commit keeps the stream if it arrived whole and the cache still owns its
// destination.
func (c *capture) Commit() {
	if c.done {
		return
	}
	c.done = true
	err := c.f.Close()
	dir := filepath.Dir(c.final)
	if err != nil || c.failed || c.written != c.size ||
		!sameCachePath(c.cache.Folder(), dir) ||
		c.cache.isProtected(dir, filepath.Base(c.final)) ||
		os.Rename(c.f.Name(), c.final) != nil {
		os.Remove(c.f.Name())
		return
	}
	c.cache.prune()
}

// Abort drops a stream that did not arrive whole.
func (c *capture) Abort() {
	if c.done {
		return
	}
	c.done = true
	c.f.Close()
	os.Remove(c.f.Name())
}

// Usage returns the bytes and the number of tracks the cache holds.
func (c *Cache) Usage() (bytes int64, tracks int) {
	for _, f := range c.files() {
		bytes += f.size
		tracks++
	}
	return bytes, tracks
}

// Clear removes every cached track.
func (c *Cache) Clear() error {
	var errs []error
	for _, f := range c.files() {
		errs = append(errs, os.Remove(f.path))
	}
	return errors.Join(errs...)
}

type cachedFile struct {
	path    string
	size    int64
	modTime time.Time
}

// isCacheFile reports whether name has a cached audio file name: a valid
// video ID (optionally followed by .5.1 or .aac) and a known audio extension.
func isCacheFile(name string) bool {
	stem := ""
	for _, ext := range extensions {
		if s, ok := strings.CutSuffix(name, ext); ok {
			stem = s
			break
		}
	}
	if stem == "" {
		return false
	}
	if s, ok := strings.CutSuffix(stem, ".5.1"); ok {
		stem = s
	} else if s, ok := strings.CutSuffix(stem, ".aac"); ok {
		stem = s
	}
	return ValidVideoID(stem)
}

// files lists the cached tracks, the least recently played first.
func (c *Cache) files() []cachedFile {
	c.mu.Lock()
	dir, protected := c.dir, c.protected
	c.mu.Unlock()
	if dir == "" {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var files []cachedFile
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || strings.HasPrefix(name, ".") || !isCacheFile(name) {
			continue
		}
		info, err := e.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		if protected != nil && protected(dir, name) {
			continue
		}
		files = append(files, cachedFile{filepath.Join(dir, name), info.Size(), info.ModTime()})
	}
	slices.SortFunc(files, func(a, b cachedFile) int { return a.modTime.Compare(b.modTime) })
	return files
}

// prune removes the least recently played tracks until the cache fits its
// limit.
func (c *Cache) prune() {
	limit := c.limitBytes()
	if limit < 0 {
		return
	}
	files := c.files()
	var total int64
	for _, f := range files {
		total += f.size
	}
	for _, f := range files {
		if total <= limit {
			return
		}
		if err := os.Remove(f.path); err == nil || errors.Is(err, fs.ErrNotExist) {
			total -= f.size
		}
	}
}
