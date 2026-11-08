package localfiles

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// The songs here are made up, as in index_test.go.

// fastWatch shortens the watch's waits for a test, until its end: its
// watcher closes before they're set back, as it's made after.
func fastWatch(t *testing.T, wait, most time.Duration) {
	t.Helper()
	s, l, r := settle, longest, retryEvery
	settle, longest, retryEvery = wait, most, 100*time.Millisecond
	t.Cleanup(func() { settle, longest, retryEvery = s, l, r })
}

// heldTitles are the titles of the tracks the index holds, in order.
func heldTitles(x *Index) []string {
	got := titles(x.Tracks())
	slices.Sort(got)
	return got
}

// waitFor waits until ok, failing the test with what saw tells after
// ten seconds: CI's runners can be slow.
func waitFor(t *testing.T, what string, ok func() bool, saw func() any) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); !ok(); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("%s didn't happen: %v", what, saw())
		}
	}
}

// watching reports whether the watcher watches folder.
func (w *Watcher) watching(folder string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	f := w.folders[folder]
	return f != nil && f.stop != nil
}

// systemWatch is the system's watch, counting the changes it tells.
type systemWatch struct{ told atomic.Int32 }

func (s *systemWatch) watch(folder string, tell Changes) (func(), error) {
	changed, moved := tell.Changed, tell.Moved
	tell.Changed = func(p string) { s.told.Add(1); changed(p) }
	tell.Moved = func(p string) { s.told.Add(1); moved(p) }
	return watchFolder(folder, tell)
}

// watchedIndex is a test index of two songs in its folder, scanned, whose
// folder the system watches, and the scans its watcher asked for, which
// it scans. It returns once the watch tells of changes: some systems take
// a moment to.
func watchedIndex(t *testing.T) (ti *testIndex, w *Watcher, rescans *atomic.Int32) {
	t.Helper()
	ti = newIndex(t)
	ti.write(t, "Shore/1.mp3", "Slow Tide|Orin Vale", time.Hour)
	ti.write(t, "Shore/2.mp3", "Low Water|Orin Vale", time.Hour)
	ti.scan(t)
	sys := &systemWatch{}
	rescans = &atomic.Int32{}
	w = NewWatcher(ti.Index, WatcherOptions{Watch: sys.watch, Rescan: func() {
		rescans.Add(1)
		ti.Scan(context.Background(), []string{ti.dir})
	}})
	t.Cleanup(w.Close)
	w.Set([]string{ti.dir})
	waitFor(t, "the watch", func() bool { return w.watching(ti.dir) }, func() any { return w.Unwatched(ti.dir) })
	// A file of no music, written until the watch tells of it.
	probe := filepath.Join(ti.dir, "probe.txt")
	for deadline := time.Now().Add(10 * time.Second); sys.told.Load() == 0; {
		if time.Now().After(deadline) {
			t.Fatal("the system told of no change")
		}
		if err := os.WriteFile(probe, []byte(time.Now().String()), 0o644); err != nil {
			t.Fatal(err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	return ti, w, rescans
}

// TestWatchFiles shows a file added, changed, renamed and removed in a
// folder of music, as the system tells, without a scan.
func TestWatchFiles(t *testing.T) {
	fastWatch(t, 50*time.Millisecond, time.Second)
	ti, _, rescans := watchedIndex(t)
	saw := func() any { return heldTitles(ti.Index) }

	added := filepath.Join(ti.dir, "Shore", "3.mp3")
	if err := os.WriteFile(added, []byte("Gulls at Noon|Orin Vale"), 0o644); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the file added", func() bool { _, ok := ti.Entry(added); return ok }, saw)

	if err := os.WriteFile(added, []byte("Gulls at Night|Orin Vale|Shore"), 0o644); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the file changed", func() bool { e, _ := ti.Entry(added); return e.Title == "Gulls at Night" }, saw)

	renamed := filepath.Join(ti.dir, "Shore", "03 Gulls.mp3")
	if err := os.Rename(added, renamed); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the file renamed", func() bool {
		_, gone := ti.Entry(added)
		e, ok := ti.Entry(renamed)
		return !gone && ok && e.Title == "Gulls at Night"
	}, saw)

	if err := os.Remove(filepath.Join(ti.dir, "Shore", "1.mp3")); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the file removed", func() bool { return reflect.DeepEqual(heldTitles(ti.Index), []string{"Gulls at Night", "Low Water"}) }, saw)
	if f := ti.Status().Folders[0]; f.Tracks != 2 || f.Skipped != 0 {
		t.Errorf("the folder: %+v", f)
	}
	if n := rescans.Load(); n != 0 {
		t.Errorf("%d scans", n)
	}
}

// TestWatchFolders reads a folder moved into a folder of music, follows it
// as it's renamed, and drops its files once it's removed; a hidden
// folder's files never show.
func TestWatchFolders(t *testing.T) {
	fastWatch(t, 50*time.Millisecond, time.Second)
	ti, _, rescans := watchedIndex(t)
	saw := func() any { return heldTitles(ti.Index) }

	// Made beside the folder, then moved in, as a folder copied in by a
	// file manager often is.
	outside := filepath.Join(filepath.Dir(ti.dir), "Kites")
	for i, title := range []string{"Paper Kites", "String Theory"} {
		path := filepath.Join(outside, "Disc 1", fmt.Sprintf("%d.mp3", i+1))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(title+"|Ada Field|Kites"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	kites := filepath.Join(ti.dir, "Kites")
	if err := os.Rename(outside, kites); err != nil {
		t.Fatal(err)
	}
	want := []string{"Low Water", "Paper Kites", "Slow Tide", "String Theory"}
	waitFor(t, "the folder moved in", func() bool { return reflect.DeepEqual(heldTitles(ti.Index), want) }, saw)

	renamed := filepath.Join(ti.dir, "Ada Field - Kites")
	if err := os.Rename(kites, renamed); err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(renamed, "Disc 1", "1.mp3")
	waitFor(t, "the folder renamed", func() bool {
		_, old := ti.Entry(filepath.Join(kites, "Disc 1", "1.mp3"))
		_, ok := ti.Entry(moved)
		return !old && ok && len(ti.Tracks()) == 4
	}, func() any { return ti.Tracks() })

	// A file written in the folder renamed shows: its folders are watched
	// by their new names.
	later := filepath.Join(renamed, "Disc 1", "3.mp3")
	if err := os.WriteFile(later, []byte("Tailwind|Ada Field|Kites"), 0o644); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "a file of the folder renamed", func() bool { _, ok := ti.Entry(later); return ok }, saw)

	hidden := filepath.Join(ti.dir, ".trash", "1.mp3")
	if err := os.MkdirAll(filepath.Dir(hidden), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hidden, []byte("Thrown Away|Ada Field"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(renamed); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the folder removed", func() bool { return reflect.DeepEqual(heldTitles(ti.Index), []string{"Low Water", "Slow Tide"}) }, saw)
	if n := rescans.Load(); n != 0 {
		t.Errorf("%d scans", n)
	}
}

// TestWatchBurst gathers a burst of files, as of a folder copied in, and
// reads them once it settles: in a few reads of the index, not one each.
func TestWatchBurst(t *testing.T) {
	fastWatch(t, 300*time.Millisecond, 5*time.Second)
	ti, _, rescans := watchedIndex(t)
	before, reads := ti.Version(), ti.reads.Load()
	const n = 40
	for i := range n {
		path := filepath.Join(ti.dir, "Live", fmt.Sprintf("%02d.mp3", i))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, fmt.Appendf(nil, "Set %02d|Orin Vale|Live", i), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	waitFor(t, "the burst", func() bool { return len(ti.Tracks()) == n+2 }, func() any { return len(ti.Tracks()) })
	if updates := ti.Version() - before; updates > 8 {
		t.Errorf("the burst was read in %d updates", updates)
	}
	if got := ti.reads.Load() - reads; got < n || got > 2*n {
		t.Errorf("the burst of %d files read %d files", n, got)
	}
	if f := ti.Status().Folders[0]; f.Tracks != n+2 {
		t.Errorf("the folder: %+v", f)
	}
	if got := rescans.Load(); got != 0 {
		t.Errorf("%d scans", got)
	}
}

// TestWatchFolderGone scans a folder of music that went, as on a drive
// unplugged, whose songs stay, and watches it again once it's back.
func TestWatchFolderGone(t *testing.T) {
	fastWatch(t, 50*time.Millisecond, time.Second)
	ti, w, rescans := watchedIndex(t)
	if err := os.RemoveAll(ti.dir); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the scan of the folder gone", func() bool {
		st := ti.Status()
		return rescans.Load() > 0 && !st.Scanning && st.Folders[0].Problem != "" && !w.watching(ti.dir)
	}, func() any { return []any{rescans.Load(), ti.Status(), w.watching(ti.dir)} })
	if got := heldTitles(ti.Index); !reflect.DeepEqual(got, []string{"Low Water", "Slow Tide"}) {
		t.Errorf("the songs of the folder gone: %q", got)
	}

	back := filepath.Join(ti.dir, "Kites", "1.mp3")
	if err := os.MkdirAll(filepath.Dir(back), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(back, []byte("Paper Kites|Ada Field"), 0o644); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the folder back", func() bool {
		return w.watching(ti.dir) && reflect.DeepEqual(heldTitles(ti.Index), []string{"Paper Kites"})
	}, func() any { return []any{heldTitles(ti.Index), w.watching(ti.dir)} })
	if f := ti.Status().Folders[0]; f.Problem != "" || f.Tracks != 1 {
		t.Errorf("the folder back: %+v", f)
	}
}

// fakeWatch is a watch of the test's: the folders it watches, how they're
// told of, and what each call of it answers.
type fakeWatch struct {
	mu      sync.Mutex
	err     error
	tells   map[string]Changes
	opened  map[string]int
	stopped map[string]int
}

func newFakeWatch(err error) *fakeWatch {
	return &fakeWatch{err: err, tells: map[string]Changes{}, opened: map[string]int{}, stopped: map[string]int{}}
}

func (f *fakeWatch) watch(folder string, tell Changes) (func(), error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.opened[folder]++
	if f.err != nil {
		return nil, f.err
	}
	f.tells[folder] = tell
	return func() {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.stopped[folder]++
	}, nil
}

// tell is how the watch of folder tells.
func (f *fakeWatch) tell(folder string) Changes {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.tells[folder]
}

// counts are how many times folder was watched, and its watches stopped.
func (f *fakeWatch) counts(folder string) (opened, stopped int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.opened[folder], f.stopped[folder]
}

// fakeWatched is a test index of a song in its folder, scanned, watched by
// a fake watch that answers err, and the scans its watcher asked for.
func fakeWatched(t *testing.T, err error) (*testIndex, *Watcher, *fakeWatch, *atomic.Int32) {
	t.Helper()
	ti := newIndex(t)
	ti.write(t, "1.mp3", "Slow Tide|Orin Vale", time.Hour)
	ti.scan(t)
	fake := newFakeWatch(err)
	rescans := &atomic.Int32{}
	w := NewWatcher(ti.Index, WatcherOptions{Watch: fake.watch, Rescan: func() { rescans.Add(1) }})
	t.Cleanup(w.Close)
	w.Set([]string{ti.dir})
	waitFor(t, "the watch", func() bool { n, _ := fake.counts(ti.dir); return n > 0 }, func() any { return fake.opened })
	return ti, w, fake, rescans
}

// TestWatchUnwatched tells why a folder can't be watched, and has the
// windows told of it; its changes show after a scan, as before.
func TestWatchUnwatched(t *testing.T) {
	for _, c := range []struct {
		err  error
		want string
	}{
		{ErrNetworkDrive, "it's on a network drive"},
		{ErrRemovableDrive, "it's on a removable drive, which a watch would keep from being ejected"},
		{fmt.Errorf("%w (fs.inotify.max_user_watches)", ErrWatchLimit), "the system's limit on watched folders is reached (fs.inotify.max_user_watches)"},
		{ErrCantWatch, "this system doesn't tell of changes to folders"},
		{errors.New("Access is denied."), "the system can't watch it"},
	} {
		fastWatch(t, 50*time.Millisecond, time.Second)
		ti := newIndex(t)
		ti.write(t, "1.mp3", "Slow Tide|Orin Vale", time.Hour)
		ti.scan(t)
		var told atomic.Int32
		stop := ti.Watch(func() { told.Add(1) })
		w := NewWatcher(ti.Index, WatcherOptions{Watch: newFakeWatch(c.err).watch})
		w.Set([]string{ti.dir})
		waitFor(t, "the folder unwatched", func() bool { return w.Unwatched(ti.dir) != "" && told.Load() > 0 },
			func() any { return []any{w.Unwatched(ti.dir), told.Load()} })
		if got := w.Unwatched(ti.dir); got != c.want {
			t.Errorf("%v: %q", c.err, got)
		}
		if got := w.Unwatched(filepath.Join(ti.dir, "other")); got != "" {
			t.Errorf("another folder: %q", got)
		}
		ti.write(t, "2.mp3", "Low Water|Orin Vale", time.Hour)
		ti.scan(t)
		if len(ti.Tracks()) != 2 {
			t.Errorf("%v: after a scan, %q", c.err, heldTitles(ti.Index))
		}
		stop()
		w.Close()
	}
}

// TestWatchTold reads what a watch tells once it settles, as one update:
// a file made, a folder made, a file of another folder, which it passes
// over, and a file only written.
func TestWatchTold(t *testing.T) {
	fastWatch(t, 50*time.Millisecond, time.Second)
	ti, _, fake, rescans := fakeWatched(t, nil)
	ti.write(t, "2.mp3", "Low Water|Orin Vale", time.Hour)
	ti.write(t, "Kites/1.mp3", "Paper Kites|Ada Field", time.Hour)
	ti.write(t, "Kites/2.mp3", "String Theory|Ada Field", time.Hour)
	elsewhere := filepath.Join(filepath.Dir(ti.dir), "elsewhere.mp3")
	if err := os.WriteFile(elsewhere, []byte("Far|Nobody"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := ti.Version()
	tell := fake.tell(ti.dir)
	tell.Moved(filepath.Join(ti.dir, "2.mp3"))
	tell.Moved(filepath.Join(ti.dir, "Kites"))
	tell.Changed(filepath.Join(ti.dir, "Kites")) // a folder written in: its files tell
	tell.Changed(elsewhere)
	want := []string{"Low Water", "Paper Kites", "Slow Tide", "String Theory"}
	waitFor(t, "the changes read", func() bool { return reflect.DeepEqual(heldTitles(ti.Index), want) },
		func() any { return heldTitles(ti.Index) })
	if n := ti.Version() - before; n != 1 {
		t.Errorf("read in %d updates", n)
	}

	// A file written without a change of its size or time of change isn't
	// read again; one whose time changed is.
	reads := ti.reads.Load()
	tell.Changed(filepath.Join(ti.dir, "1.mp3"))
	ti.write(t, "2.mp3", "Low Water|Orin Vale|Shore", time.Minute)
	tell.Changed(filepath.Join(ti.dir, "2.mp3"))
	waitFor(t, "the file written", func() bool { e, _ := ti.Entry(filepath.Join(ti.dir, "2.mp3")); return e.Album == "Shore" },
		func() any { return ti.Tracks() })
	if n := ti.reads.Load() - reads; n != 1 {
		t.Errorf("%d files read again", n)
	}
	if n := rescans.Load(); n != 0 {
		t.Errorf("%d scans", n)
	}
}

// TestWatchLost scans the folders when a watch lost track of what changed,
// and when more changed than the watch keeps.
func TestWatchLost(t *testing.T) {
	fastWatch(t, 50*time.Millisecond, time.Second)
	ti, _, fake, rescans := fakeWatched(t, nil)
	tell := fake.tell(ti.dir)
	tell.Lost()
	waitFor(t, "a scan", func() bool { return rescans.Load() == 1 }, func() any { return rescans.Load() })

	m := maxChanges
	maxChanges = 10
	t.Cleanup(func() { maxChanges = m })
	for i := range 20 {
		tell.Moved(filepath.Join(ti.dir, fmt.Sprintf("%d.mp3", i)))
	}
	waitFor(t, "a scan of too many changes", func() bool { return rescans.Load() == 2 }, func() any { return rescans.Load() })
}

// TestWatchEnded scans a folder whose watch ended, as when its drive was
// unplugged, and watches it again once it's back; one past the system's
// limit stays unwatched, and says so.
func TestWatchEnded(t *testing.T) {
	fastWatch(t, 50*time.Millisecond, time.Second)
	ti, w, fake, rescans := fakeWatched(t, nil)
	fake.tell(ti.dir).Ended(errors.New("the folder went"))
	waitFor(t, "the watch again", func() bool {
		opened, stopped := fake.counts(ti.dir)
		return opened == 2 && stopped == 1 && w.watching(ti.dir) && rescans.Load() >= 2
	}, func() any { return []any{fake.opened, fake.stopped, rescans.Load()} })

	fake.tell(ti.dir).Ended(fmt.Errorf("%w (fs.inotify.max_user_watches)", ErrWatchLimit))
	waitFor(t, "the folder unwatched", func() bool { return w.Unwatched(ti.dir) != "" && !w.watching(ti.dir) },
		func() any { return w.Unwatched(ti.dir) })
	time.Sleep(3 * retryEvery)
	if opened, _ := fake.counts(ti.dir); opened != 2 {
		t.Errorf("a folder past the limit was watched again: %d", opened)
	}
}

// TestWatchSet watches the folders named last, and stops the watches of
// the others, and all of them as it closes.
func TestWatchSet(t *testing.T) {
	ti := newIndex(t)
	a, b := filepath.Join(ti.dir, "a"), filepath.Join(ti.dir, "b")
	for _, dir := range []string{a, b} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	fake := newFakeWatch(nil)
	w := NewWatcher(ti.Index, WatcherOptions{Watch: fake.watch})
	w.Set([]string{a, b, filepath.Join(ti.dir, "not there")})
	waitFor(t, "both watched", func() bool { return w.watching(a) && w.watching(b) }, func() any { return fake.opened })
	if n, _ := fake.counts(filepath.Join(ti.dir, "not there")); n != 0 {
		t.Error("a folder that isn't there was watched")
	}
	w.Set([]string{b})
	waitFor(t, "a stopped", func() bool { _, stopped := fake.counts(a); return stopped == 1 && !w.watching(a) },
		func() any { return fake.stopped })
	w.Close()
	if _, stopped := fake.counts(b); stopped != 1 {
		t.Errorf("b stopped %d times as the watcher closed", stopped)
	}
}

// TestUpdate reads again only what changed at the paths told: a file new,
// changed or gone, a folder made or gone, but not a folder only written
// in, nor what's hidden or beyond the folders; a change of the folder of
// music itself, or of one gone, asks for a scan.
func TestUpdate(t *testing.T) {
	ti := newIndex(t)
	ti.write(t, "Shore/1.mp3", "Slow Tide|Orin Vale", time.Hour)
	ti.write(t, "Shore/2.mp3", "Low Water|Orin Vale", time.Hour)
	ti.write(t, "Kites/1.mp3", "Paper Kites|Ada Field", time.Hour)
	ti.scan(t)
	ctx := context.Background()
	path := func(name string) string { return filepath.Join(ti.dir, filepath.FromSlash(name)) }

	ti.write(t, "Shore/3.mp3", "Gulls at Noon|Orin Vale", time.Hour)
	ti.write(t, "Live/1.mp3", "Set One|Orin Vale", time.Hour)
	ti.write(t, "Live/.hidden/1.mp3", "Hidden|Orin Vale", time.Hour)
	ti.write(t, "Kites/2.mp3", "String Theory|Ada Field", time.Hour)
	ti.write(t, ".trash/1.mp3", "Thrown Away|Ada Field", time.Hour)
	os.Remove(path("Shore/2.mp3"))
	reads := ti.reads.Load()
	rescan := ti.update(ctx, map[string]bool{
		path("Shore/3.mp3"):  true,
		path("Shore/2.mp3"):  true,
		path("Live"):         true,
		path("Kites"):        false, // written in: Kites/2.mp3 isn't told
		path(".trash/1.mp3"): true,
		path("notes.txt"):    true,
		filepath.Join(filepath.Dir(ti.dir), "elsewhere.mp3"): true,
	})
	if rescan {
		t.Error("a scan asked for")
	}
	if got, want := heldTitles(ti.Index), []string{"Gulls at Noon", "Paper Kites", "Set One", "Slow Tide"}; !reflect.DeepEqual(got, want) {
		t.Errorf("tracks %q, want %q", got, want)
	}
	if n := ti.reads.Load() - reads; n != 2 {
		t.Errorf("read %d files, want 2", n)
	}
	if f := ti.Status().Folders[0]; f.Tracks != 4 {
		t.Errorf("the folder: %+v", f)
	}

	// A folder gone drops its files; nothing changed changes nothing.
	os.RemoveAll(path("Live"))
	if ti.update(ctx, map[string]bool{path("Live"): true}) || len(ti.Tracks()) != 3 {
		t.Errorf("the folder gone: %q", heldTitles(ti.Index))
	}
	v := ti.Version()
	if ti.update(ctx, map[string]bool{path("Shore/1.mp3"): false}) || ti.Version() != v {
		t.Error("a file unchanged changed the index")
	}

	if !ti.update(ctx, map[string]bool{ti.dir: true}) {
		t.Error("the folder of music renamed asks for no scan")
	}
	if ti.update(ctx, map[string]bool{ti.dir: false}) {
		t.Error("the folder of music written in asks for a scan")
	}
	moved := ti.dir + "-moved"
	if err := os.Rename(ti.dir, moved); err != nil {
		t.Fatal(err)
	}
	defer os.Rename(moved, ti.dir)
	if !ti.update(ctx, map[string]bool{path("Shore/1.mp3"): true}) || len(ti.Tracks()) != 3 {
		t.Errorf("the folder of music gone: %q", heldTitles(ti.Index))
	}
}
