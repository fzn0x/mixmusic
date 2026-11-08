package localfiles

// The watch of the folders of music: while MixMusic runs, the system tells
// of what changes in them, and the index reads only that again (update),
// once the changes settle, so that a folder copied in is read once, whole,
// and no scan reads the folders again. Each system tells with its own API:
// ReadDirectoryChangesW on Windows (watch_windows.go), FSEvents on macOS
// (watch_darwin.go) and inotify on Linux (watch_linux.go). A folder that
// can't be watched, as one on a network drive, whose changes made from
// other computers the system doesn't hear of, is read as MixMusic starts
// and when asked, as before, and Settings tells why.

import (
	"context"
	"errors"
	"log"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/fzn0x/mixmusic/internal/settings"
)

// Why a folder can't be watched, in the words Settings tells them in.
var (
	ErrNetworkDrive   = errors.New("it's on a network drive")
	ErrRemovableDrive = errors.New("it's on a removable drive, which a watch would keep from being ejected")
	ErrWatchLimit     = errors.New("the system's limit on watched folders is reached")
	ErrCantWatch      = errors.New("this system doesn't tell of changes to folders")
)

// Changes are what a watch of a folder of music tells, from goroutines of
// its own.
type Changes struct {
	// Changed tells that what the file at path holds changed: it was
	// written, or its time of change was set. A folder's changes are its
	// files'.
	Changed func(path string)
	// Moved tells that path, a file or a folder, was made, renamed to or
	// from, or removed: a folder there is read whole.
	Moved func(path string)
	// Lost tells that the watch lost track of what changed, as when too
	// much changed at once for the system to keep: the folders are
	// scanned.
	Lost func()
	// Ended tells that the watch stopped by itself, and why: as when the
	// folder went, its drive was unplugged, or the system's limit on
	// watched folders was reached as folders were added to it.
	Ended func(err error)
}

// A WatchFolder watches folder, a folder of music, and the folders in it,
// telling of their changes until stop, which tells nothing after it
// returns. It fails where the folder can't be watched: on a network drive
// (ErrNetworkDrive) or a removable one (ErrRemovableDrive), past the
// system's limit on watched folders (ErrWatchLimit), or on a system that
// doesn't tell of changes (ErrCantWatch).
type WatchFolder func(folder string, tell Changes) (stop func(), err error)

// settle is how long the watch waits once the changes stop before the
// index reads them, and longest how long it waits at most while they go
// on: a folder copied in, file after file, is read once. retryEvery is how
// often a folder that went, as on a drive unplugged, is looked for, to be
// watched again once it's back. maxChanges bounds the changes kept: past
// it, the folders are scanned instead.
var (
	settle     = time.Second
	longest    = 5 * time.Second
	retryEvery = 30 * time.Second
	maxChanges = 10000
)

// WatcherOptions are how a Watcher watches.
type WatcherOptions struct {
	// Watch watches a folder, as the system does when it's nil.
	Watch WatchFolder
	// Rescan scans the folders of music, as when a watch lost track of
	// what changed, or a folder went or came back.
	Rescan func()
}

// Watcher watches the folders of music while MixMusic runs, and has the
// index read what changed in them. It is safe for concurrent use.
type Watcher struct {
	x      *Index
	watch  WatchFolder
	rescan func()
	ctx    context.Context
	cancel context.CancelFunc
	// kick wakes run: the folders to watch, or the changes, changed.
	kick chan struct{}
	wg   sync.WaitGroup

	mu sync.Mutex
	// want are the folders to watch, and folders those watched or
	// tried, by their paths.
	want    []string
	folders map[string]*watched
	// changes are the paths changed since the last update, each true
	// when it was made, renamed or removed; lost is set when a scan
	// must read them, between first and last, the times of the first
	// and the last change.
	changes     map[string]bool
	lost        bool
	first, last time.Time
	// updating is set while the index reads the changes, and check when
	// a folder of music may have gone.
	updating, check bool
}

// watched is a folder of music, as the watch has it.
type watched struct {
	// stop stops its watch, nil while it isn't watched: as it can't be,
	// as why tells, or while it isn't there, when it's looked for again.
	stop func()
	why  string
	// ended is why its watch stopped by itself, until run takes it in.
	ended error
}

// NewWatcher watches nothing, until Set names the folders to watch.
func NewWatcher(x *Index, opts WatcherOptions) *Watcher {
	w := &Watcher{x: x, watch: opts.Watch, rescan: opts.Rescan, kick: make(chan struct{}, 1),
		folders: map[string]*watched{}, changes: map[string]bool{}}
	if w.watch == nil {
		w.watch = watchFolder
	}
	if w.rescan == nil {
		w.rescan = func() {}
	}
	w.ctx, w.cancel = context.WithCancel(context.Background())
	w.wg.Add(1)
	go w.run()
	return w
}

// Set watches folders, and no others.
func (w *Watcher) Set(folders []string) {
	w.mu.Lock()
	w.want = slices.Clone(folders)
	w.mu.Unlock()
	w.poke()
}

// Close stops the watches, and waits for an update under way.
func (w *Watcher) Close() {
	w.cancel()
	w.wg.Wait()
}

// Unwatched tells why MixMusic can't watch folder for changes, as when
// it's on a network drive: "" while it watches it, and while the folder
// isn't there, as its scan tells.
func (w *Watcher) Unwatched(folder string) string {
	w.mu.Lock()
	defer w.mu.Unlock()
	for path, f := range w.folders {
		if settings.SamePath(path, folder) {
			return f.why
		}
	}
	return ""
}

// poke wakes run.
func (w *Watcher) poke() {
	select {
	case w.kick <- struct{}{}:
	default:
	}
}

// run follows the folders to watch, and has the index read the changes
// once they settle, until Close.
func (w *Watcher) run() {
	defer w.wg.Done()
	timer := time.NewTimer(time.Hour)
	timer.Stop()
	retry := time.NewTicker(retryEvery)
	defer retry.Stop()
	for {
		select {
		case <-w.ctx.Done():
			timer.Stop()
			w.stopAll()
			return
		case <-w.kick:
			w.follow()
		case <-timer.C:
			w.apply()
		case <-retry.C:
			w.lookAgain()
		}
		if d, ok := w.due(); ok {
			timer.Reset(d)
		} else {
			timer.Stop()
		}
	}
}

// due tells how long until the changes are read, false without any, or
// while an update reads those before.
func (w *Watcher) due() (time.Duration, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.updating || len(w.changes) == 0 && !w.lost {
		return 0, false
	}
	return max(min(time.Until(w.last.Add(settle)), time.Until(w.first.Add(longest))), 0), true
}

// apply has the index read the changes that settled, beside run: a scan
// when the watch lost track, or one of the folders changed itself.
func (w *Watcher) apply() {
	if d, ok := w.due(); !ok || d > 0 {
		return
	}
	w.mu.Lock()
	changes, lost := w.changes, w.lost
	w.changes, w.lost, w.first, w.last = map[string]bool{}, false, time.Time{}, time.Time{}
	w.updating = true
	w.mu.Unlock()
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		rescan := lost || w.x.update(w.ctx, changes)
		if rescan && w.ctx.Err() == nil {
			w.rescan()
		}
		w.mu.Lock()
		w.updating = false
		w.check = w.check || rescan
		w.mu.Unlock()
		w.poke()
	}()
}

// follow watches the folders Set named: it stops the watches of those no
// longer named, takes in those that ended, and those of folders that went,
// and starts those of the folders new to it.
func (w *Watcher) follow() {
	w.mu.Lock()
	var stops []func()
	var open, look []string
	for path, f := range w.folders {
		switch {
		case !slices.Contains(w.want, path):
			delete(w.folders, path)
		case f.ended != nil:
			// What it lost meanwhile, a scan reads; a folder that went is
			// looked for again, and one past the limit stays unwatched.
			if errors.Is(f.ended, ErrWatchLimit) {
				f.why = whyUnwatched(f.ended)
			}
			f.ended = nil
		case w.check && f.stop != nil:
			look = append(look, path)
			continue
		default:
			continue
		}
		if f.stop != nil {
			stops = append(stops, f.stop)
			f.stop = nil
		}
	}
	w.check = false
	for _, path := range w.want {
		if _, ok := w.folders[path]; !ok {
			open = append(open, path)
		}
	}
	w.mu.Unlock()
	for _, stop := range stops {
		stop()
	}
	// A folder of music that went, or was renamed, is watched again once
	// it's back.
	for _, path := range look {
		if _, err := os.Stat(path); err == nil {
			continue
		}
		w.mu.Lock()
		f := w.folders[path]
		var stop func()
		if f != nil {
			stop, f.stop = f.stop, nil
		}
		w.mu.Unlock()
		if stop != nil {
			stop()
		}
	}
	for _, path := range open {
		w.open(path)
	}
	if len(stops) > 0 || len(open) > 0 {
		w.x.changed()
	}
}

// open watches the folder at path, unless it isn't there, and reports
// whether it does.
func (w *Watcher) open(path string) bool {
	f := &watched{}
	w.mu.Lock()
	old := w.folders[path]
	w.folders[path] = f
	w.mu.Unlock()
	if old != nil && old.stop != nil {
		old.stop()
	}
	if _, err := os.Stat(path); err != nil {
		return false
	}
	stop, err := w.watch(path, w.changesOf(path, f))
	w.mu.Lock()
	defer w.mu.Unlock()
	if err != nil {
		f.why = whyUnwatched(err)
		log.Printf("mixmusic: watching %s for changes: %v", path, err)
		return false
	}
	f.stop = stop
	return true
}

// lookAgain watches the folders that weren't there, or whose watch
// stopped as they went, once they're back, and has them scanned.
func (w *Watcher) lookAgain() {
	w.mu.Lock()
	var back []string
	for path, f := range w.folders {
		if f.stop == nil && f.why == "" && f.ended == nil {
			back = append(back, path)
		}
	}
	w.mu.Unlock()
	watched := false
	for _, path := range back {
		if _, err := os.Stat(path); err == nil && w.open(path) {
			watched = true
		}
	}
	if watched {
		w.x.changed()
		w.rescan()
	}
}

// stopAll stops every watch, as the Watcher closes.
func (w *Watcher) stopAll() {
	w.mu.Lock()
	var stops []func()
	for path, f := range w.folders {
		if f.stop != nil {
			stops = append(stops, f.stop)
		}
		delete(w.folders, path)
	}
	w.mu.Unlock()
	for _, stop := range stops {
		stop()
	}
}

// changesOf is what the watch of the folder at path, as f has it, tells
// the Watcher.
func (w *Watcher) changesOf(path string, f *watched) Changes {
	return Changes{
		Changed: func(p string) { w.note(p, false) },
		Moved:   func(p string) { w.note(p, true) },
		Lost:    w.lose,
		Ended: func(err error) {
			w.mu.Lock()
			if w.folders[path] == f {
				f.ended = err
			}
			w.mu.Unlock()
			w.lose()
		},
	}
}

// note takes in that path changed: moved when it was made, renamed or
// removed. A file of a kind the library doesn't take, only written, as a
// log or a download's part, changes nothing.
func (w *Watcher) note(path string, moved bool) {
	if !moved && !Plays(path) {
		return
	}
	w.mu.Lock()
	switch {
	case w.lost:
	case len(w.changes) >= maxChanges:
		w.lost, w.changes = true, map[string]bool{}
	default:
		w.changes[path] = w.changes[path] || moved
	}
	w.noted()
	w.mu.Unlock()
	w.poke()
}

// lose takes in that a watch lost track of what changed: the folders are
// scanned once the changes settle.
func (w *Watcher) lose() {
	w.mu.Lock()
	w.lost, w.changes = true, map[string]bool{}
	w.noted()
	w.mu.Unlock()
	w.poke()
}

// noted keeps the time of a change; w.mu is held.
func (w *Watcher) noted() {
	now := time.Now()
	if w.first.IsZero() {
		w.first = now
	}
	w.last = now
}

// whyUnwatched words why a folder can't be watched, as Settings tells it.
func whyUnwatched(err error) string {
	for _, known := range []error{ErrNetworkDrive, ErrRemovableDrive, ErrWatchLimit, ErrCantWatch} {
		if errors.Is(err, known) {
			return err.Error()
		}
	}
	return "the system can't watch it"
}
