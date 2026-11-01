package app

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo"

	"github.com/fzn0x/mixmusic/builtin"
	"github.com/fzn0x/mixmusic/internal/colorfile"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// pywalFile is a colors.json of pywal's, whose background is bg.
func pywalFile(bg string) string {
	return `{"wallpaper": "/x.jpg", "alpha": "100",
"special": {"background": "` + bg + `", "foreground": "#c5c8cc", "cursor": "#c5c8cc"},
"colors": {"color0": "` + bg + `", "color1": "#3e6a8a", "color2": "#5b7c99", "color3": "#7a8fa3", "color4": "#c2734b",
"color5": "#d68a5c", "color6": "#8fa9bf", "color7": "#c5c8cc", "color8": "#5d6064", "color9": "#3e6a8a", "color10": "#5b7c99",
"color11": "#7a8fa3", "color12": "#c2734b", "color13": "#d68a5c", "color14": "#8fa9bf", "color15": "#c5c8cc"}}`
}

// fakeWatch is a watch whose changes the test sends: nil for a change, an
// error to end it.
type fakeWatch struct {
	path    string
	ctx     context.Context
	changes chan error
	closed  chan struct{}
	once    sync.Once
}

func (w *fakeWatch) Next(ctx context.Context) ([]mygo.FileEvent, error) {
	select {
	case err := <-w.changes:
		if err != nil {
			return nil, err
		}
		return []mygo.FileEvent{{Op: mygo.FileWrite, Path: "."}}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (w *fakeWatch) Close() error {
	w.once.Do(func() { close(w.closed) })
	return nil
}

// colorTest is an app whose colour file a test follows, with watches it
// controls, and the times it told the windows.
type colorTest struct {
	t       *testing.T
	a       *App
	dir     string
	watches chan *fakeWatch
	told    atomic.Int32
	// run has the app "run", and follow the file as Start has it.
	run func()
}

// newColorTest makes an app whose settings follow the colour file that
// set makes, and tells the test of the watches it starts, once the app
// "runs".
func newColorTest(t *testing.T, set func(s *settings.Settings)) *colorTest {
	t.Helper()
	c := &colorTest{t: t, dir: t.TempDir(), watches: make(chan *fakeWatch, 16)}
	look, retry, ready, watch := colorFileLook, colorFileRetry, watchReady, watchFile
	t.Cleanup(func() { colorFileLook, colorFileRetry, watchReady, watchFile = look, retry, ready, watch })
	colorFileLook, colorFileRetry = 20*time.Millisecond, 5*time.Millisecond
	var running atomic.Bool
	watchReady = running.Load
	watchFile = func(ctx context.Context, path string) (fileWatch, error) {
		// As MyGo's, a watch of a file that isn't there fails.
		if _, err := os.Stat(path); err != nil {
			return nil, err
		}
		w := &fakeWatch{path: path, ctx: ctx, changes: make(chan error, 4), closed: make(chan struct{})}
		c.watches <- w
		return w, nil
	}
	data := filepath.Join(c.dir, "data")
	if err := os.MkdirAll(data, 0o755); err != nil {
		t.Fatal(err)
	}
	if set != nil {
		s := settings.Default()
		set(&s)
		writeJSON(t, filepath.Join(data, "settings.json"), s)
	}
	a, err := New(Options{DataDir: data, CacheDir: filepath.Join(c.dir, "cache"), Builtin: builtin.FS, Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	c.a = a
	t.Cleanup(a.Watch(func() { c.told.Add(1) }))
	t.Cleanup(func() {
		// The watch ends with the test, before the waits are put back.
		a.followColorFile(settings.Settings{})
		a.colorFile.running.Wait()
	})
	c.run = func() { running.Store(true); a.followColorFile(a.settings.Get().Clean()) }
	return c
}

func writeJSON(t *testing.T, path string, s settings.Settings) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := json.NewEncoder(f).Encode(s); err != nil {
		t.Fatal(err)
	}
}

func (c *colorTest) write(name, content string) string {
	c.t.Helper()
	p := filepath.Join(c.dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		c.t.Fatal(err)
	}
	return p
}

func (c *colorTest) status() ColorFileStatus { return c.a.Services().Preferences.ColorFile() }

// until waits for the status to be as ok says.
func (c *colorTest) until(what string, ok func(ColorFileStatus) bool) ColorFileStatus {
	c.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		st := c.status()
		if ok(st) {
			return st
		}
		if time.Now().After(deadline) {
			c.t.Fatalf("%s: the status stayed %+v", what, st)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// watch waits for the next watch the app starts.
func (c *colorTest) watch() *fakeWatch {
	c.t.Helper()
	select {
	case w := <-c.watches:
		return w
	case <-time.After(5 * time.Second):
		c.t.Fatal("no watch started")
		return nil
	}
}

func background(st ColorFileStatus) string { return st.Colors.Mode("dark")["background"] }

// TestColorFileFollows reads the file as MixMusic opens, before its first
// frame, then again each time it changes, telling the windows once for
// each change, and not at all for a write of the same colours.
func TestColorFileFollows(t *testing.T) {
	var c *colorTest
	dir := t.TempDir()
	path := filepath.Join(dir, "colors.json")
	if err := os.WriteFile(path, []byte(pywalFile("#0d1117")), 0o644); err != nil {
		t.Fatal(err)
	}
	c = newColorTest(t, func(s *settings.Settings) { s.ColorSource, s.ColorFile = settings.ColorsFile, path })
	if st := c.status(); background(st) != "#0d1117" || st.Path != path || st.Err != "" || st.ReadAt.IsZero() {
		t.Fatalf("New didn't read the file: %+v", st)
	}
	// Written before the watch starts: the read that follows its start
	// sees it.
	if err := os.WriteFile(path, []byte(pywalFile("#101820")), 0o644); err != nil {
		t.Fatal(err)
	}
	c.run()
	w := c.watch()
	if w.path != path {
		t.Errorf("watching %q", w.path)
	}
	c.until("the write before the watch", func(st ColorFileStatus) bool { return background(st) == "#101820" })
	told := c.told.Load()
	if err := os.WriteFile(path, []byte(pywalFile("#202830")), 0o644); err != nil {
		t.Fatal(err)
	}
	w.changes <- nil
	c.until("a write", func(st ColorFileStatus) bool { return background(st) == "#202830" })
	for deadline := time.Now().Add(5 * time.Second); c.told.Load() == told && time.Now().Before(deadline); {
		time.Sleep(2 * time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond)
	if n := c.told.Load() - told; n != 1 {
		t.Errorf("a write told the windows %d times", n)
	}
	told = c.told.Load()
	w.changes <- nil // the same colours, written again
	w.changes <- nil
	time.Sleep(50 * time.Millisecond)
	if n := c.told.Load() - told; n != 0 {
		t.Errorf("writing the same colours told the windows %d times", n)
	}
}

// TestColorFileHalfWritten reads a file that pywal emptied, then wrote,
// as a whole: the colours go from the old to the new, with no error
// between.
func TestColorFileHalfWritten(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "colors.json")
	whole := pywalFile("#202830")
	if err := os.WriteFile(path, []byte(pywalFile("#0d1117")), 0o644); err != nil {
		t.Fatal(err)
	}
	c := newColorTest(t, func(s *settings.Settings) { s.ColorSource, s.ColorFile = settings.ColorsFile, path })
	// The read that follows the watch's start, which the next change must
	// come after.
	if err := os.WriteFile(path, []byte(pywalFile("#151d25")), 0o644); err != nil {
		t.Fatal(err)
	}
	c.run()
	w := c.watch()
	c.until("the read as the watch started", func(st ColorFileStatus) bool { return background(st) == "#151d25" })
	// The next reads find the file empty, then half written, then whole.
	reads := []string{"", whole[:len(whole)/2], whole}
	var mu sync.Mutex
	var seen []ColorFileStatus
	read := readColors
	t.Cleanup(func() { readColors = read })
	readColors = func(p string) (colorfile.Colors, []byte, error) {
		mu.Lock()
		defer mu.Unlock()
		seen = append(seen, c.a.colorFile.peek())
		data := []byte(whole)
		if len(reads) > 0 {
			data, reads = []byte(reads[0]), reads[1:]
		}
		colors, err := colorfile.Parse(data)
		return colors, data, err
	}
	w.changes <- nil
	st := c.until("the whole file", func(st ColorFileStatus) bool { return background(st) == "#202830" })
	if st.Err != "" {
		t.Errorf("an error stayed: %q", st.Err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(seen) != 3 {
		t.Errorf("%d reads, want 3: empty, half written and whole", len(seen))
	}
	for _, st := range seen {
		if st.Err != "" || background(st) != "#151d25" {
			t.Errorf("while the file was being written, the status was %+v", st)
		}
	}
}

// peek is the status, as the windows would read it.
func (f *colorFile) peek() ColorFileStatus {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.st
}

// TestColorFileBroken keeps the colours read last while the file doesn't
// read, and says why; brings the theme's colours back when it goes away,
// and the file's when it comes back.
func TestColorFileBroken(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "colors.json")
	if err := os.WriteFile(path, []byte(pywalFile("#0d1117")), 0o644); err != nil {
		t.Fatal(err)
	}
	c := newColorTest(t, func(s *settings.Settings) { s.ColorSource, s.ColorFile = settings.ColorsFile, path })
	c.run()
	w := c.watch()
	read := c.until("the first read", func(st ColorFileStatus) bool { return !st.ReadAt.IsZero() }).ReadAt
	if err := os.WriteFile(path, []byte("{\"special\": oops"), 0o644); err != nil {
		t.Fatal(err)
	}
	w.changes <- nil
	st := c.until("garbage", func(st ColorFileStatus) bool { return st.Err != "" })
	if background(st) != "#0d1117" || !st.ReadAt.Equal(read) || st.Missing {
		t.Errorf("garbage dropped the colours read last: %+v", st)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	w.changes <- nil
	st = c.until("the file gone", func(st ColorFileStatus) bool { return st.Missing })
	if !st.Colors.Empty() || st.Err != "" {
		t.Errorf("a file gone kept its colours: %+v", st)
	}
	if err := os.WriteFile(path, []byte(pywalFile("#303840")), 0o644); err != nil {
		t.Fatal(err)
	}
	w.changes <- nil
	c.until("the file back", func(st ColorFileStatus) bool { return background(st) == "#303840" && !st.Missing })
}

// TestColorFileFolderGone looks for the file again, every little while,
// once its folder went, and follows it once it's back.
func TestColorFileFolderGone(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "wal")
	path := filepath.Join(dir, "colors.json")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(pywalFile("#0d1117")), 0o644); err != nil {
		t.Fatal(err)
	}
	c := newColorTest(t, func(s *settings.Settings) { s.ColorSource, s.ColorFile = settings.ColorsFile, path })
	// Windows removes no file that is open: the read as the watch starts
	// ends first.
	if err := os.WriteFile(path, []byte(pywalFile("#151d25")), 0o644); err != nil {
		t.Fatal(err)
	}
	c.run()
	w := c.watch()
	c.until("the read as the watch started", func(st ColorFileStatus) bool { return background(st) == "#151d25" })
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	w.changes <- io.EOF
	select {
	case <-w.closed:
	case <-time.After(5 * time.Second):
		t.Fatal("the watch of a folder gone wasn't closed")
	}
	c.until("the folder gone", func(st ColorFileStatus) bool { return st.Missing })
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(pywalFile("#404850")), 0o644); err != nil {
		t.Fatal(err)
	}
	c.until("the folder back", func(st ColorFileStatus) bool { return background(st) == "#404850" })
	c.watch() // followed again
}

// TestColorFileSettingChanged stops following the file the settings no
// longer name, and reads the one they do.
func TestColorFileSettingChanged(t *testing.T) {
	c := newColorTest(t, nil)
	first := c.write("first.json", pywalFile("#0d1117"))
	second := c.write("second.json", pywalFile("#505860"))
	set := func(path string) {
		s := c.a.Services().Preferences.Get()
		s.ColorSource, s.ColorFile = settings.ColorsFile, path
		if _, err := c.a.Services().Preferences.Set(s); err != nil {
			t.Fatal(err)
		}
	}
	c.run()
	set(first)
	w := c.watch()
	c.until("the first file", func(st ColorFileStatus) bool { return background(st) == "#0d1117" })
	set(second)
	select {
	case <-w.ctx.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("the first file's watch goes on")
	}
	if w2 := c.watch(); w2.path != second {
		t.Errorf("watching %q", w2.path)
	}
	c.until("the second file", func(st ColorFileStatus) bool { return background(st) == "#505860" && st.Path == second })
	s := c.a.Services().Preferences.Get()
	s.ColorSource = settings.ColorsTheme
	if _, err := c.a.Services().Preferences.Set(s); err != nil {
		t.Fatal(err)
	}
	if st := c.status(); !st.Colors.Empty() || st.Path != "" {
		t.Errorf("the theme's colours keep the file's: %+v", st)
	}
	// Changing another setting starts nothing.
	s.FontScale = 1.1
	if _, err := c.a.Services().Preferences.Set(s); err != nil {
		t.Fatal(err)
	}
	select {
	case w := <-c.watches:
		t.Errorf("a watch of %q started", w.path)
	case <-time.After(30 * time.Millisecond):
	}
}

// TestColorFileReload reads the file again on Reload add-ons, as the
// network shares that send no changes need.
func TestColorFileReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "colors.json")
	if err := os.WriteFile(path, []byte(pywalFile("#0d1117")), 0o644); err != nil {
		t.Fatal(err)
	}
	c := newColorTest(t, func(s *settings.Settings) { s.ColorSource, s.ColorFile = settings.ColorsFile, path })
	if err := os.WriteFile(path, []byte(pywalFile("#606870")), 0o644); err != nil {
		t.Fatal(err)
	}
	c.a.Services().Addons.Reload()
	if st := c.status(); background(st) != "#606870" {
		t.Errorf("Reload, before the app runs, read %+v", st)
	}
	c.run()
	c.watch()
	if err := os.WriteFile(path, []byte(pywalFile("#707880")), 0o644); err != nil {
		t.Fatal(err)
	}
	c.a.Services().Addons.Reload()
	c.until("Reload, as the app runs", func(st ColorFileStatus) bool { return background(st) == "#707880" })
}

// TestColorFilePywal reads pywal's file where pywal writes it, and finds it
// where it appears later: in ~/.cache/wal, where pywal 3.3.0 writes, though
// $XDG_CACHE_HOME is set.
func TestColorFilePywal(t *testing.T) {
	home, xdg := t.TempDir(), t.TempDir()
	t.Setenv("PYWAL_CACHE_DIR", "")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CACHE_HOME", xdg)
	c := newColorTest(t, func(s *settings.Settings) { s.ColorSource = settings.ColorsPywal })
	if st := c.status(); !st.Missing || st.Path != filepath.Join(xdg, "wal", "colors.json") {
		t.Fatalf("before pywal ran: %+v", st)
	}
	c.run()
	dir := filepath.Join(home, ".cache", "wal")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "colors.json"), []byte(pywalFile("#808890")), 0o644); err != nil {
		t.Fatal(err)
	}
	st := c.until("pywal's file", func(st ColorFileStatus) bool { return background(st) == "#808890" })
	if st.Path != filepath.Join(dir, "colors.json") || st.Missing {
		t.Errorf("pywal's file: %+v", st)
	}
}

// TestColorFileMatugen reads matugen.json in the data folder.
func TestColorFileMatugen(t *testing.T) {
	c := newColorTest(t, func(s *settings.Settings) { s.ColorSource = settings.ColorsMatugen })
	if st := c.status(); !st.Missing || st.Path != filepath.Join(c.dir, "data", MatugenFile) {
		t.Errorf("before matugen ran: %+v", st)
	}
}

// TestColorFilePaths refuses a path that isn't full, and reads nothing
// before a file is chosen.
func TestColorFilePaths(t *testing.T) {
	c := newColorTest(t, func(s *settings.Settings) { s.ColorSource, s.ColorFile = settings.ColorsFile, "colors.json" })
	if st := c.status(); !strings.Contains(st.Err, "isn't a full path") || st.Path != "colors.json" {
		t.Errorf("a path that isn't full: %+v", st)
	}
	c = newColorTest(t, func(s *settings.Settings) { s.ColorSource = settings.ColorsFile })
	if st := c.status(); st.Err != "" || st.Path != "" || st.Missing {
		t.Errorf("no file chosen: %+v", st)
	}
	c.run()
	select {
	case w := <-c.watches:
		t.Errorf("a watch of %q started", w.path)
	case <-time.After(30 * time.Millisecond):
	}
}
