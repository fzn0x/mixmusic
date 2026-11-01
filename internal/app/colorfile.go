package app

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/egoist/mygo"

	"github.com/fzn0x/mixmusic/internal/colorfile"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// MatugenFile is the file in the data folder that MixMusic's matugen
// template writes, which the colours of settings.ColorsMatugen come from.
const MatugenFile = "matugen.json"

// ColorFileStatus is what MixMusic read of the colour file the settings
// name.
type ColorFileStatus struct {
	// Source is the setting followed.
	Source settings.ColorSource
	// Path is the file read: "" while the colours are the theme's, or no
	// file is chosen yet.
	Path string
	// Colors are the colours read last, which stay while the file can't be
	// read; none while it isn't there.
	Colors colorfile.Colors
	// ReadAt is when Colors were read.
	ReadAt time.Time
	// Err tells why the file couldn't be read the last time, "" if it
	// could.
	Err string
	// Missing is set while the file isn't there: the theme's colours show.
	Missing bool
}

// colorFile follows the colour file the settings name: it reads it again
// each time it changes.
type colorFile struct {
	mu sync.Mutex
	// source and file are the settings followed, and gen counts the times
	// they changed: a read for settings since changed is dropped.
	source settings.ColorSource
	file   string
	gen    int
	st     ColorFileStatus
	// raw is the content of the file as last read whole.
	raw []byte
	// cancel stops the watch, nil while none was started; running counts
	// the goroutines that read the file.
	cancel  context.CancelFunc
	running sync.WaitGroup
}

// fileWatch is a watch of a file, as mygo.WatchFiles starts.
type fileWatch interface {
	Next(ctx context.Context) ([]mygo.FileEvent, error)
	Close() error
}

var (
	// watchFile watches the file at path; tests replace it.
	watchFile = func(ctx context.Context, path string) (fileWatch, error) {
		return mygo.WatchFiles(ctx, path, mygo.FileWatchOptions{Debounce: 200 * time.Millisecond})
	}
	// watchReady tells whether watches may start: once the app runs, which
	// tests don't. Until then, the file is read where its settings are.
	watchReady = func() bool { return mygo.App.IsReady() }
	// colorFileLook is how long MixMusic waits to look again for a file
	// that isn't there, or a watch that ended; colorFileRetry how long it
	// waits to read again a file that didn't read whole, colorFileTries
	// times. pywal and matugen empty a file before they write it again.
	colorFileLook  = 5 * time.Second
	colorFileRetry = 150 * time.Millisecond
	colorFileTries = 3
	// readColors reads a colour file; tests replace it.
	readColors = colorfile.Read
)

// ColorFile returns what MixMusic read of the colour file the settings
// name.
func (p *Preferences) ColorFile() ColorFileStatus {
	f := &p.a.colorFile
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.st
}

// followColorFile follows the colour file of s. Settings that changed start
// over: the file is read again, in the watch once the app runs, and before
// it in place, as when the app opens, before its first frame. The watch
// starts once the app runs, from Start.
func (a *App) followColorFile(s settings.Settings) {
	f := &a.colorFile
	ready := watchReady()
	f.mu.Lock()
	same := s.ColorSource == f.source && s.ColorFile == f.file
	if same && (f.cancel != nil || !ready || f.source == settings.ColorsTheme) {
		f.mu.Unlock()
		return
	}
	if f.cancel != nil {
		f.cancel()
		f.cancel = nil
	}
	if !same {
		f.source, f.file = s.ColorSource, s.ColorFile
		f.gen++
		f.st, f.raw = ColorFileStatus{Source: s.ColorSource}, nil
	}
	gen := f.gen
	if s.ColorSource == settings.ColorsTheme || s.ColorSource == settings.ColorsFile && s.ColorFile == "" {
		f.mu.Unlock()
		return
	}
	var ctx context.Context
	if ready {
		ctx, f.cancel = context.WithCancel(context.Background())
	}
	f.mu.Unlock()
	if ready {
		f.running.Add(1)
		go func() {
			defer f.running.Done()
			a.watchColorFile(ctx, gen, s)
		}()
		return
	}
	path, err := a.colorFilePath(s)
	if err != nil {
		a.colorFileFailed(gen, s.ColorFile, err)
		return
	}
	a.readColorFile(context.Background(), gen, path, false)
}

// colorFilePath is the file of s: pywal's, where pywal writes it now;
// matugen's, in the data folder; or the user's own.
func (a *App) colorFilePath(s settings.Settings) (string, error) {
	switch s.ColorSource {
	case settings.ColorsPywal:
		return colorfile.PywalPath(os.Getenv, func(p string) bool {
			_, err := os.Stat(p)
			return err == nil
		}), nil
	case settings.ColorsMatugen:
		return filepath.Join(a.opts.DataDir, MatugenFile), nil
	case settings.ColorsFile:
		return colorfile.Expand(s.ColorFile)
	}
	return "", nil
}

// watchColorFile reads the colour file of s, and again each time it
// changes, until ctx ends. A file that isn't there, or whose folder goes,
// is looked for again every colorFileLook, where pywal writes it then.
func (a *App) watchColorFile(ctx context.Context, gen int, s settings.Settings) {
	for ctx.Err() == nil {
		path, err := a.colorFilePath(s)
		if err != nil {
			if a.colorFileFailed(gen, s.ColorFile, err) {
				notify()
			}
			return
		}
		w, werr := watchFile(ctx, path)
		// Read once the watch started, which reports no change made before
		// it: a write meanwhile isn't missed.
		if a.readColorFile(ctx, gen, path, true) {
			notify()
		}
		if werr == nil {
			for {
				if _, err := w.Next(ctx); err != nil {
					break // the folder went, or ctx ended
				}
				if a.readColorFile(ctx, gen, path, true) {
					notify()
				}
			}
			w.Close()
		}
		if !sleep(ctx, colorFileLook) {
			return
		}
	}
}

// readColorFile reads the colour file at path for the settings of gen, and
// tells whether what MixMusic shows of it changed. With retry, a file that
// doesn't read whole is read again a moment later: it may be being
// written. A file that still doesn't keeps the colours read last.
func (a *App) readColorFile(ctx context.Context, gen int, path string, retry bool) bool {
	colors, raw, err := readColors(path)
	for i := 0; retry && i < colorFileTries && err != nil && !errors.Is(err, fs.ErrNotExist); i++ {
		if !sleep(ctx, colorFileRetry) {
			return false
		}
		colors, raw, err = readColors(path)
	}
	f := &a.colorFile
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.gen != gen {
		return false
	}
	prev := f.st
	st := prev
	st.Path = path
	switch {
	case errors.Is(err, fs.ErrNotExist):
		st.Colors, st.ReadAt, st.Err, st.Missing = colorfile.Colors{}, time.Time{}, "", true
		f.raw = nil
	case err != nil:
		st.Err, st.Missing = err.Error(), false
	case bytes.Equal(raw, f.raw) && prev.Err == "" && !prev.Missing:
		return false
	default:
		// New colours, or the same after an error: a change, though
		// the clock, which on Windows moves in steps of up to 16 ms,
		// may tell the same time as the read before.
		st.Colors, st.ReadAt, st.Err, st.Missing = colors, time.Now(), "", false
		f.raw = raw
		f.st = st
		return true
	}
	f.st = st
	return st.Path != prev.Path || st.Err != prev.Err || st.Missing != prev.Missing
}

// colorFileFailed tells why the file of the settings of gen, the path as
// the user gave it, can't be read, and whether that's news.
func (a *App) colorFileFailed(gen int, path string, err error) bool {
	f := &a.colorFile
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.gen != gen || f.st.Err == err.Error() && f.st.Path == path {
		return false
	}
	f.st.Path, f.st.Err = path, err.Error()
	return true
}

// rereadColorFile reads the colour file again, as Reload add-ons asks:
// beside the main thread once the app runs, and tells the windows if what
// they show of it changed.
func (a *App) rereadColorFile() {
	f := &a.colorFile
	f.mu.Lock()
	gen, s := f.gen, settings.Settings{ColorSource: f.source, ColorFile: f.file}
	f.mu.Unlock()
	if s.ColorSource == settings.ColorsTheme || s.ColorSource == settings.ColorsFile && s.ColorFile == "" {
		return
	}
	read := func() bool {
		path, err := a.colorFilePath(s)
		return err == nil && a.readColorFile(context.Background(), gen, path, false)
	}
	if !watchReady() {
		read()
		return
	}
	f.running.Add(1)
	go func() {
		defer f.running.Done()
		if read() {
			notify()
		}
	}()
}

// sleep waits for d, and tells whether ctx is still going after it.
func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
