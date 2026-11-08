package app

import (
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/localfiles"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// waitLocal waits until ok, failing the test with what saw tells after
// ten seconds: the watch waits a second for the changes to settle, and
// CI's runners can be slow.
func waitLocal(t *testing.T, what string, ok func() bool, saw func() any) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); !ok(); time.Sleep(20 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("%s didn't happen: %v", what, saw())
		}
	}
}

// localTitles are the titles of the tracks of the music on this computer.
func localTitles(a *App) []string {
	var out []string
	for _, tr := range (&LocalFiles{a}).Tracks() {
		out = append(out, tr.Title)
	}
	slices.Sort(out)
	return out
}

// TestLocalWatch shows a file added to a folder of music, and one removed,
// once MixMusic started, as the system tells of them, without a scan.
func TestLocalWatch(t *testing.T) {
	a, music := localApp(t)
	a.startLocalFiles()
	saw := func() any { return localTitles(a) }
	waitLocal(t, "the scan at the start", func() bool { return len(localTitles(a)) == 2 }, saw)
	flacData, err := os.ReadFile(filepath.Join("..", "..", "flac", "testdata", "stereo-16.flac"))
	if err != nil {
		t.Fatal(err)
	}
	// Written again until it shows: the system may take a moment to watch
	// the folder after the start.
	added := filepath.Join(music, "Shore", "03 Gulls at Noon.flac")
	for deadline := time.Now().Add(10 * time.Second); !slices.Contains(localTitles(a), "03 Gulls at Noon"); {
		if time.Now().After(deadline) {
			t.Fatalf("the file added didn't show: %q", localTitles(a))
		}
		if err := os.WriteFile(added, flacData, 0o644); err != nil {
			t.Fatal(err)
		}
		waitUntil(3*time.Second, func() bool { return slices.Contains(localTitles(a), "03 Gulls at Noon") })
	}
	if err := os.Remove(filepath.Join(music, "Shore", "01 Slow Tide.flac")); err != nil {
		t.Fatal(err)
	}
	waitLocal(t, "the file removed", func() bool { return slices.Equal(localTitles(a), []string{"02 Low Water", "03 Gulls at Noon"}) }, saw)
	if why := (&LocalFiles{a}).Unwatched(music); why != "" {
		t.Errorf("the folder is unwatched: %s", why)
	}
}

// waitUntil waits until ok, or for at most d, and reports ok.
func waitUntil(d time.Duration, ok func() bool) bool {
	for deadline := time.Now().Add(d); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if ok() {
			return true
		}
	}
	return ok()
}

// testWatch is a watch of the test's: the folders watched, how each is
// told of, and whether its watch stopped; err, when set, fails each.
type testWatch struct {
	mu      sync.Mutex
	err     error
	tells   map[string]localfiles.Changes
	stopped map[string]bool
}

func (w *testWatch) watch(folder string, tell localfiles.Changes) (func(), error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.err != nil {
		return nil, w.err
	}
	w.tells[folder], w.stopped[folder] = tell, false
	return func() {
		w.mu.Lock()
		defer w.mu.Unlock()
		w.stopped[folder] = true
	}, nil
}

// watching reports whether folder is watched, and how it's told of.
func (w *testWatch) watching(folder string) (localfiles.Changes, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	tell, ok := w.tells[folder]
	return tell, ok && !w.stopped[folder]
}

func newTestWatch(err error) *testWatch {
	return &testWatch{err: err, tells: map[string]localfiles.Changes{}, stopped: map[string]bool{}}
}

// TestLocalWatchFollows watches the folders of the settings as they
// change, scans them when a watch lost track, and stops as the app
// closes.
func TestLocalWatchFollows(t *testing.T) {
	a, music := localApp(t)
	w := newTestWatch(nil)
	a.opts.WatchMusic = w.watch
	a.startLocalFiles()
	waitLocal(t, "the watch", func() bool { _, ok := w.watching(music); return ok }, func() any { return w.tells })

	other := filepath.Join(filepath.Dir(music), "other")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	st := a.settings.Get()
	st.LocalFiles = settings.LocalFiles{MusicFolders: []string{other}}
	if err := a.settings.Set(st); err != nil {
		t.Fatal(err)
	}
	a.apply(a.settings.Get())
	waitLocal(t, "the watch of the folder added, and the other's end", func() bool {
		_, old := w.watching(music)
		_, ok := w.watching(other)
		return ok && !old
	}, func() any { return w.stopped })

	l := &LocalFiles{a}
	waitLocal(t, "the scan of the folders", func() bool { return !l.Status().Scanning }, func() any { return l.Status() })
	v := l.Version()
	tell, _ := w.watching(other)
	tell.Lost()
	waitLocal(t, "the scan of a watch that lost track", func() bool { return l.Version() != v && !l.Status().Scanning },
		func() any { return l.Status() })

	a.closeLocalFiles()
	if _, ok := w.watching(other); ok {
		t.Error("the watch didn't stop as the app closed")
	}
}

// TestLocalUnwatched tells why a folder of music can't be watched, as on
// a network drive; before MixMusic starts, it tells nothing.
func TestLocalUnwatched(t *testing.T) {
	a, music := localApp(t)
	l := &LocalFiles{a}
	if why := l.Unwatched(music); why != "" {
		t.Errorf("before the start: %q", why)
	}
	a.opts.WatchMusic = newTestWatch(localfiles.ErrNetworkDrive).watch
	scanned(t, a)
	waitLocal(t, "the folder unwatched", func() bool { return l.Unwatched(music) == "it's on a network drive" },
		func() any { return l.Unwatched(music) })
	if n := len(l.Tracks()); n != 2 {
		t.Errorf("%d tracks", n)
	}
}
