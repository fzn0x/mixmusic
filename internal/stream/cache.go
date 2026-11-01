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

// Cache keeps the audio of tracks played to their end on disk, up to a
// size, so that playing them again needs no network. The tracks played
// least recently go first.
type Cache struct {
	Dir string

	mu    sync.Mutex
	limit int64
}

// extensions maps content types to the extensions cached files get, which
// tell the content type back when they are served.
var extensions = map[string]string{
	"audio/webm": ".webm",
	"audio/mp4":  ".m4a",
	"audio/mpeg": ".mp3",
}

// NewCache returns a cache in dir of at most limit bytes; 0 caches nothing.
func NewCache(dir string, limit int64) *Cache {
	return &Cache{Dir: dir, limit: limit}
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
	for mime, ext := range extensions {
		p := filepath.Join(c.Dir, videoID+ext)
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
// costs no network. A file it can't move, as on another volume, or open on
// Windows, it copies, leaving the cached one to the cache.
func (c *Cache) Take(videoID, dst string) (bool, error) {
	path, _, ok := c.find(videoID)
	if !ok {
		return false, nil
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
// when the cache is off or cannot take it.
func (c *Cache) Begin(videoID, mime string, size int64) *capture {
	limit := c.limitBytes()
	base, _, _ := strings.Cut(mime, ";")
	ext, known := extensions[strings.TrimSpace(base)]
	if limit <= 0 || !known || size <= 0 || size > limit {
		return nil
	}
	if err := os.MkdirAll(c.Dir, 0o755); err != nil {
		return nil
	}
	f, err := os.CreateTemp(c.Dir, "."+videoID+"-*.part")
	if err != nil {
		return nil
	}
	return &capture{cache: c, f: f, final: filepath.Join(c.Dir, videoID+ext), size: size}
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

// Commit keeps the stream if it arrived whole.
func (c *capture) Commit() {
	if c.done {
		return
	}
	c.done = true
	err := c.f.Close()
	if err != nil || c.failed || c.written != c.size || os.Rename(c.f.Name(), c.final) != nil {
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

// files lists the cached tracks, the least recently played first.
func (c *Cache) files() []cachedFile {
	entries, err := os.ReadDir(c.Dir)
	if err != nil {
		return nil
	}
	var files []cachedFile
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, cachedFile{filepath.Join(c.Dir, e.Name()), info.Size(), info.ModTime()})
	}
	slices.SortFunc(files, func(a, b cachedFile) int { return a.modTime.Compare(b.modTime) })
	return files
}

// prune removes the least recently played tracks until the cache fits its
// limit.
func (c *Cache) prune() {
	limit := c.limitBytes()
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
