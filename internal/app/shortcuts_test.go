package app

import (
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"runtime"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/builtin"
	"github.com/fzn0x/mixmusic/internal/player"
)

// fakeHotkeys registers global shortcuts as MyGo does, for the presses the
// test makes: a second Register of the same accelerator fails, and so do
// those fail holds.
type fakeHotkeys struct {
	mu    sync.Mutex
	calls []string
	fns   map[string]func()
	fail  map[string]error
	// wait is how long Register takes, as MyGo waits for the main thread;
	// hold, if set, runs as Register starts.
	wait time.Duration
	hold func()
}

func newFakeHotkeys() *fakeHotkeys {
	return &fakeHotkeys{fns: map[string]func(){}, fail: map[string]error{}}
}

func (f *fakeHotkeys) Register(acc string, fn func()) error {
	time.Sleep(f.wait)
	f.mu.Lock()
	hold := f.hold
	f.mu.Unlock()
	if hold != nil {
		hold()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "register "+acc)
	if _, ok := f.fns[acc]; ok {
		return fmt.Errorf("mygo: shortcut %s is already registered", acc)
	}
	if err := f.fail[acc]; err != nil {
		return err
	}
	f.fns[acc] = fn
	return nil
}

func (f *fakeHotkeys) Unregister(acc string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "unregister "+acc)
	delete(f.fns, acc)
}

// take returns the calls since the last take.
func (f *fakeHotkeys) take() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	calls := f.calls
	f.calls = nil
	return calls
}

// registered are the accelerators registered, sorted.
func (f *fakeHotkeys) registered() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var accs []string
	for acc := range f.fns {
		accs = append(accs, acc)
	}
	slices.Sort(accs)
	return accs
}

// press presses the keys of acc.
func (f *fakeHotkeys) press(t *testing.T, acc string) {
	t.Helper()
	f.mu.Lock()
	fn := f.fns[acc]
	f.mu.Unlock()
	if fn == nil {
		t.Fatalf("%s isn't registered: %v", acc, f.registered())
	}
	fn()
}

// newShortcutApp is an app in a folder of its own whose global shortcuts
// register with a fake, and whose presses the test sees in ran.
func newShortcutApp(t *testing.T) (*App, *fakeHotkeys, *[]string) {
	t.Helper()
	dir := t.TempDir()
	hk := newFakeHotkeys()
	a, err := New(Options{DataDir: filepath.Join(dir, "data"), CacheDir: filepath.Join(dir, "cache"), Builtin: builtin.FS, Version: "test", Hotkeys: hk})
	if err != nil {
		t.Fatal(err)
	}
	ran := &[]string{}
	a.global.run = func(id string) { *ran = append(*ran, id) }
	return a, hk, ran
}

// setGlobal saves the settings with these global shortcuts.
func setGlobal(t *testing.T, a *App, global map[string]string) {
	t.Helper()
	p := &Preferences{a}
	s := p.Get()
	s.GlobalShortcuts = global
	if _, err := p.Set(s); err != nil {
		t.Fatal(err)
	}
}

// acc is the accelerator keys register as here.
func acc(t *testing.T, keys string) string {
	t.Helper()
	a, ok := globalAccelerator(keys, runtime.GOOS)
	if !ok {
		t.Fatalf("%q has no accelerator", keys)
	}
	return a
}

func TestGlobalAccelerator(t *testing.T) {
	for _, c := range []struct {
		keys          string
		others, macOS string
	}{
		{"Mod+Alt+P", "Ctrl+Alt+P", "Super+Alt+P"},
		{"Ctrl+Alt+P", "Ctrl+Alt+P", "Ctrl+Alt+P"},
		{"Meta+Alt+K", "Super+Alt+K", "Super+Alt+K"},
		{"Cmd+K", "Super+K", "Super+K"},
		{"Mod+Shift+ArrowRight", "Ctrl+Shift+Right", "Super+Shift+Right"},
		{"Shift+Alt+Mod+P", "Ctrl+Alt+Shift+P", "Super+Alt+Shift+P"},
		{"Mod+Alt+Space", "Ctrl+Alt+Space", "Super+Alt+Space"},
		{"Mod+Alt+PageDown", "Ctrl+Alt+PageDown", "Super+Alt+PageDown"},
		{"Mod+Alt+,", "Ctrl+Alt+,", "Super+Alt+,"},
		{"Mod+Alt+\\", "Ctrl+Alt+\\", "Super+Alt+\\"},
		{"F9", "F9", "F9"},
	} {
		for goos, want := range map[string]string{"windows": c.others, "linux": c.others, "darwin": c.macOS} {
			if got, ok := globalAccelerator(c.keys, goos); !ok || got != want {
				t.Errorf("globalAccelerator(%q, %s) = %q, %v, want %q", c.keys, goos, got, ok, want)
			}
		}
	}
	for _, keys := range []string{"", "Mod++", "Mod+", "Mod+Foo", "Hyper+K", "Mod+a", "Mod+Alt+F13"} {
		if got, ok := globalAccelerator(keys, "windows"); ok {
			t.Errorf("globalAccelerator(%q) = %q", keys, got)
		}
	}
}

func TestGlobalKeysProblem(t *testing.T) {
	for _, c := range []struct {
		keys string
		// ok lists the systems the keys can be a global shortcut on.
		ok []string
	}{
		{"K", nil},
		{"Shift+K", nil},
		{"Space", nil},
		{"Alt+K", []string{"windows", "linux"}},
		{"Ctrl+Alt+K", []string{"windows", "linux", "darwin"}},
		{"Mod+K", []string{"windows", "linux", "darwin"}},
		{"Meta+K", []string{"windows", "linux", "darwin"}},
		{"Mod+Alt+ArrowUp", []string{"windows", "linux", "darwin"}},
		{"F9", []string{"windows", "linux", "darwin"}},
		{"Shift+F5", []string{"windows", "linux", "darwin"}},
		{"F12", []string{"linux", "darwin"}},
		{"Shift+F12", []string{"windows", "linux", "darwin"}},
		{"Mod+Alt+Enter", []string{"windows", "linux", "darwin"}},
		{"Mod+Escape", nil},
		{"Mod+Tab", nil},
		{"Mod+Backspace", nil},
		{"Mod+Delete", nil},
		{"Mod+Foo", nil},
	} {
		for _, goos := range []string{"windows", "linux", "darwin"} {
			got := GlobalKeysProblem(c.keys, goos)
			if want := slices.Contains(c.ok, goos); (got == "") != want {
				t.Errorf("GlobalKeysProblem(%q, %s) = %q", c.keys, goos, got)
			}
		}
	}
	if got := GlobalKeysProblem("Shift+K", "windows"); got != "A global shortcut needs Ctrl, Alt or the Windows key" {
		t.Errorf("Windows: %q", got)
	}
	if got := GlobalKeysProblem("Shift+K", "linux"); got != "A global shortcut needs Ctrl, Alt or Super" {
		t.Errorf("Linux: %q", got)
	}
	if got := GlobalKeysProblem("Alt+K", "darwin"); got != "A global shortcut needs ⌘ or ⌃" {
		t.Errorf("macOS: %q", got)
	}
	if got := GlobalKeysProblem("F12", "windows"); got != "Windows keeps F12 for itself" {
		t.Errorf("F12: %q", got)
	}
}

// TestApplyGlobalShortcuts registers the keys of the settings as they
// change, and only then, and keeps why those that didn't register didn't.
func TestApplyGlobalShortcuts(t *testing.T) {
	a, hk, _ := newShortcutApp(t)
	p := &Preferences{a}
	if calls := hk.take(); len(calls) != 0 {
		t.Fatalf("a new install registers %q", calls)
	}
	setGlobal(t, a, map[string]string{"player.toggle": "Mod+Alt+P"})
	if calls := hk.take(); !slices.Equal(calls, []string{"register " + acc(t, "Mod+Alt+P")}) {
		t.Errorf("setting the keys called %q", calls)
	}
	// Saves of anything else, as the player's of the volume, call nothing.
	s := p.Get()
	s.Volume = 0.3
	if _, err := p.Set(s); err != nil {
		t.Fatal(err)
	}
	if calls := hk.take(); len(calls) != 0 {
		t.Errorf("saving the volume called %q", calls)
	}
	setGlobal(t, a, map[string]string{"player.toggle": "Mod+Alt+O"})
	if calls := hk.take(); !slices.Equal(calls, []string{"unregister " + acc(t, "Mod+Alt+P"), "register " + acc(t, "Mod+Alt+O")}) {
		t.Errorf("changing the keys called %q", calls)
	}

	// Keys in use elsewhere stay the user's, with the problem, until they
	// register as MixMusic tries again.
	inUse := acc(t, "Mod+Alt+N")
	hk.fail[inUse] = errors.New("mygo: cannot register \"Ctrl+Alt+N\": Hot key is already registered.")
	setGlobal(t, a, map[string]string{"player.toggle": "Mod+Alt+O", "player.next": "Mod+Alt+N"})
	if got := p.GlobalShortcutProblem("player.next"); got != "These keys are already in use: pick others, or try again once they're free" {
		t.Errorf("keys in use: %q", got)
	}
	if got := p.Get().GlobalShortcuts["player.next"]; got != "Mod+Alt+N" {
		t.Errorf("keys in use were dropped: %q", got)
	}
	hk.take()
	p.RetryGlobalShortcuts()
	if calls := hk.take(); !slices.Equal(calls, []string{"register " + inUse}) {
		t.Errorf("trying again called %q", calls)
	}
	delete(hk.fail, inUse)
	p.RetryGlobalShortcuts()
	if got := p.GlobalShortcutProblem("player.next"); got != "" || !slices.Contains(hk.registered(), inUse) {
		t.Errorf("trying once the keys are free: %q, %v", got, hk.registered())
	}

	// Keys a command before has, keys that can't be global and commands
	// this version doesn't know register nothing.
	hk.take()
	setGlobal(t, a, map[string]string{"player.toggle": "Mod+Alt+O", "player.next": "Mod+Alt+N", "player.like": "Mod+Alt+N",
		"player.mute": "K", "plugin.later": "Mod+Alt+L"})
	if calls := hk.take(); len(calls) != 0 {
		t.Errorf("keys that can't register called %q", calls)
	}
	if got := p.GlobalShortcutProblem("player.like"); got != "Another command has these keys" {
		t.Errorf("keys of a command before: %q", got)
	}
	if got := p.GlobalShortcutProblem("player.mute"); got != GlobalKeysProblem("K", runtime.GOOS) {
		t.Errorf("keys that can't be global: %q", got)
	}

	// The desktop's refusals.
	hk.fail[acc(t, "Mod+Alt+V")] = fmt.Errorf("mygo: global shortcuts need an X11 session or a desktop with the GlobalShortcuts portal: %w", errors.ErrUnsupported)
	setGlobal(t, a, map[string]string{"player.volumeUp": "Mod+Alt+V"})
	if got := p.GlobalShortcutProblem("player.volumeUp"); got != "Your desktop doesn't offer global shortcuts" {
		t.Errorf("no portal: %q", got)
	}
	if p.GlobalShortcutProblem("player.like") != "" || p.GlobalShortcutProblem("player.mute") != "" {
		t.Error("the problems of keys that went stay")
	}
	if _, err := p.Reset(); err != nil {
		t.Fatal(err)
	}
	if got := hk.registered(); len(got) != 0 || p.GlobalShortcutProblem("player.volumeUp") != "" {
		t.Errorf("reset settings leaves %v registered, problem %q", got, p.GlobalShortcutProblem("player.volumeUp"))
	}
}

func TestGlobalShortcutError(t *testing.T) {
	entry := errors.New("mygo: global shortcuts on Wayland need the app installed, with its desktop entry mixmusic.desktop: GDBus.Error:org.freedesktop.portal.Error.NotAllowed")
	if got := globalShortcutError(entry, "linux"); got != "Global shortcuts need MixMusic installed with its desktop entry" {
		t.Errorf("no desktop entry: %q", got)
	}
	grab := errors.New("mygo: Ctrl+Alt+P is already a global shortcut of another app")
	for _, goos := range []string{"linux", "windows", "darwin"} {
		if got := globalShortcutError(grab, goos); got != "These keys are already in use: pick others, or try again once they're free" {
			t.Errorf("%s: %q", goos, got)
		}
	}
}

// TestApplyGlobalShortcutsAtOnce saves the settings from goroutines at
// once, as the main thread and the player's goroutine do: the keys of the
// settings kept last end up registered, never registered twice.
func TestApplyGlobalShortcutsAtOnce(t *testing.T) {
	a, hk, _ := newShortcutApp(t)
	hk.wait = time.Millisecond
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			keys := map[string]string{"player.toggle": "Mod+Alt+P", "player.next": fmt.Sprintf("Mod+Alt+%d", i)}
			if i%2 == 1 {
				keys = map[string]string{"player.next": "Mod+Alt+P", "player.toggle": fmt.Sprintf("Mod+Alt+%d", i)}
			}
			setGlobal(t, a, keys)
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			a.applyGlobalShortcuts(false)
		}()
	}
	wg.Wait()
	want := []string{}
	for _, keys := range a.settings.Get().GlobalShortcuts {
		want = append(want, acc(t, keys))
	}
	slices.Sort(want)
	if got := hk.registered(); !slices.Equal(got, want) {
		t.Errorf("registered %v, the settings have %v", got, want)
	}
	// A second Register of keys registered would have failed.
	p := &Preferences{a}
	for _, gc := range GlobalCommands {
		if got := p.GlobalShortcutProblem(gc.ID); got != "" {
			t.Errorf("%s: %q", gc.ID, got)
		}
	}
}

// TestGlobalShortcutsBackMeanwhile takes the keys away while the pass that
// registers them waits: the pass that follows unregisters them, though the
// settings are as they were before it.
func TestGlobalShortcutsBackMeanwhile(t *testing.T) {
	a, hk, _ := newShortcutApp(t)
	entered, release := make(chan struct{}), make(chan struct{})
	hk.hold = func() {
		hk.mu.Lock()
		hk.hold = nil
		hk.mu.Unlock()
		close(entered)
		<-release
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		setGlobal(t, a, map[string]string{"player.toggle": "Mod+Alt+P"})
	}()
	<-entered
	setGlobal(t, a, map[string]string{})
	close(release)
	<-done
	if got := hk.registered(); len(got) != 0 {
		t.Errorf("keys taken away meanwhile stay registered: %v", got)
	}
}

// TestGlobalShortcutPressed runs the command of the keys pressed, unless
// the hook takes them, and once for a key held, where the system repeats
// the presses of keys held.
func TestGlobalShortcutPressed(t *testing.T) {
	a, hk, ran := newShortcutApp(t)
	setGlobal(t, a, map[string]string{"player.toggle": "Mod+Alt+P", "player.next": "Mod+Alt+N", "player.volumeUp": "Mod+Alt+ArrowUp"})
	hk.press(t, acc(t, "Mod+Alt+P"))
	if !slices.Equal(*ran, []string{"player.toggle"}) {
		t.Errorf("pressing the keys ran %q", *ran)
	}

	var hooked []string
	take := true
	cancel := a.OnGlobalShortcut(func(id, keys string) bool {
		hooked = append(hooked, id+" "+keys)
		return take
	})
	*ran = nil
	hk.press(t, acc(t, "Mod+Alt+P"))
	if !slices.Equal(hooked, []string{"player.toggle Mod+Alt+P"}) || len(*ran) != 0 {
		t.Errorf("the hook took %q, and the commands ran %q", hooked, *ran)
	}
	take = false
	hk.press(t, acc(t, "Mod+Alt+P"))
	if !slices.Equal(*ran, []string{"player.toggle"}) {
		t.Errorf("a press the hook leaves ran %q", *ran)
	}
	cancel()
	hooked, *ran = nil, nil
	hk.press(t, acc(t, "Mod+Alt+N"))
	if len(hooked) != 0 || !slices.Equal(*ran, []string{"player.next"}) {
		t.Errorf("after cancel the hook saw %q, the commands ran %q", hooked, *ran)
	}

	// Where the system repeats the presses of a key held.
	now := time.Unix(1e9, 0)
	a.global.repeats, a.global.now = true, func() time.Time { return now }
	*ran = nil
	for _, step := range []time.Duration{0, 100, 250, 400, 0} {
		now = now.Add(step * time.Millisecond)
		hk.press(t, acc(t, "Mod+Alt+N"))
		hk.press(t, acc(t, "Mod+Alt+ArrowUp"))
	}
	next, up := 0, 0
	for _, id := range *ran {
		switch id {
		case "player.next":
			next++
		case "player.volumeUp":
			up++
		}
	}
	if next != 2 || up != 5 {
		t.Errorf("a key held ran Next track %d times, want 2, and Volume up %d of 5", next, up)
	}
}

// TestGlobalCommands has the player do what the keys pressed ask.
func TestGlobalCommands(t *testing.T) {
	a := newQuietApp(t, nil)
	p := a.GoPlayer()
	p.SetVolume(0.5)
	a.globalCommand("player.volumeUp")
	a.globalCommand("player.volumeUp")
	a.globalCommand("player.volumeDown")
	waitPlayer(t, p, "the volume up a step", func(s player.State) bool { return math.Abs(s.Volume-0.55) < 1e-9 })
	a.globalCommand("player.mute")
	waitPlayer(t, p, "muted", func(s player.State) bool { return s.Muted })
}

// TestGlobalShortcutsWaitForMyGo registers nothing before MyGo runs, the
// app's real registrar, which would panic.
func TestGlobalShortcutsWaitForMyGo(t *testing.T) {
	a := newQuietApp(t, nil)
	p := &Preferences{a}
	s := p.Get()
	s.GlobalShortcuts = map[string]string{"player.toggle": "Mod+Alt+P"}
	if _, err := p.Set(s); err != nil {
		t.Fatal(err)
	}
	p.RetryGlobalShortcuts()
	if got := p.GlobalShortcutProblem("player.toggle"); got != "" {
		t.Errorf("before MyGo runs: %q", got)
	}
}
