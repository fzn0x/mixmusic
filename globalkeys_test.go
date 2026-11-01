package main

import (
	"errors"
	"os"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
)

// testHotkeys registers global shortcuts for the presses the test makes,
// failing those fail holds.
type testHotkeys struct {
	mu   sync.Mutex
	fns  map[string]func()
	fail map[string]error
}

func newTestHotkeys() *testHotkeys {
	return &testHotkeys{fns: map[string]func(){}, fail: map[string]error{}}
}

func (h *testHotkeys) Register(acc string, fn func()) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := h.fail[acc]; err != nil {
		return err
	}
	h.fns[acc] = fn
	return nil
}

func (h *testHotkeys) Unregister(acc string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.fns, acc)
}

func (h *testHotkeys) registered() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	var accs []string
	for acc := range h.fns {
		accs = append(accs, acc)
	}
	slices.Sort(accs)
	return accs
}

// press presses the keys of acc, which must be registered.
func (h *testHotkeys) press(t *testing.T, acc string) {
	t.Helper()
	h.mu.Lock()
	fn := h.fns[acc]
	h.mu.Unlock()
	if fn == nil {
		t.Fatalf("%s isn't registered: %v", acc, h.registered())
	}
	fn()
}

// globalAcc is the accelerator MyGo registers Ctrl, or ⌘ on macOS, with
// these keys as.
func globalAcc(keys string) string {
	if runtime.GOOS == "darwin" {
		return "Super+" + keys
	}
	return "Ctrl+" + keys
}

// newGlobalShell is a test shell whose global shortcuts register with h,
// on Settings › Shortcuts.
func newGlobalShell(t *testing.T, h *testHotkeys) (*shell, *ui.Tester) {
	t.Helper()
	s := newTestShellOf(t, func(o *app.Options) { o.Hotkeys = h })
	// As the main window's start does.
	t.Cleanup(s.a.OnGlobalShortcut(s.globalPress))
	tt := testerOf(t, s, 1280, 4000)
	show(s, tt, "/settings/shortcuts")
	return s, tt
}

// TestGlobalShortcutRecorder records the keys of global shortcuts, which
// other commands give up, of the window's too, refuses keys that can't be
// global, and removes them.
func TestGlobalShortcutRecorder(t *testing.T) {
	h := newTestHotkeys()
	s, tt := newGlobalShell(t, h)
	if !tt.HasText("from any app") || !tt.HasText(globalHint(runtime.GOOS)) {
		t.Fatalf("no group of global shortcuts: %q", tt.Texts())
	}
	click(t, s, tt, "set the global shortcut of Play or pause")
	if s.recording != globalRecording+"player.toggle" || !tt.Focused("cancel the global shortcut of Play or pause") ||
		!tt.HasText("If nothing happens, these keys are in use elsewhere.") {
		t.Fatalf("set doesn't record the keys: recording %q", s.recording)
	}
	// Keys that can't be global say why, and the recording goes on.
	tt.Key(0, ui.KeyK)
	if why := app.GlobalKeysProblem("K", runtime.GOOS); !tt.HasText(why) || s.recording == "" || len(s.set.GlobalShortcuts) != 0 {
		t.Errorf("K alone: toast %v, recording %q, kept %v", tt.HasText(why), s.recording, s.set.GlobalShortcuts)
	}
	tt.Key(ui.Cmd|ui.Alt, ui.KeyP)
	if got := s.svc.Preferences.Get().GlobalShortcuts["player.toggle"]; got != "Mod+Alt+P" || s.recording != "" {
		t.Fatalf("recorded %q, recording %q", got, s.recording)
	}
	if got := h.registered(); !slices.Equal(got, []string{globalAcc("Alt+P")}) {
		t.Errorf("registered %v", got)
	}
	if !tt.HasText(shortcutLabel("Mod+Alt+P")) || !tt.HasText("remove") {
		t.Errorf("the row doesn't show the keys: %q", tt.Texts())
	}

	// Another command's global keys go to the command that records them.
	click(t, s, tt, "set the global shortcut of Next track")
	tt.Key(ui.Cmd|ui.Alt, ui.KeyP)
	if g := s.set.GlobalShortcuts; g["player.next"] != "Mod+Alt+P" || g["player.toggle"] != "" || !tt.HasText("Play or pause has no global shortcut now") {
		t.Errorf("the keys of Play or pause: %v", g)
	}
	// So do the window's: Shuffle's own keys.
	click(t, s, tt, "set the global shortcut of Mute")
	tt.Key(ui.Cmd, ui.KeyS)
	if keys, ok := s.set.Shortcuts["player.shuffle"]; !ok || keys != "" || !tt.HasText("Shuffle has no shortcut now") || s.set.GlobalShortcuts["player.mute"] != "Mod+S" {
		t.Errorf("Shuffle keeps its keys %q, %v; Mute has %q", keys, ok, s.set.GlobalShortcuts["player.mute"])
	}
	// And the window's shortcut of another command takes them back.
	click(t, s, tt, "change the shortcut of Repeat")
	tt.Key(ui.Cmd, ui.KeyS)
	if s.set.Shortcuts["player.repeat"] != "Mod+S" || s.set.GlobalShortcuts["player.mute"] != "" || !tt.HasText("Mute has no global shortcut now") {
		t.Errorf("Repeat %q, Mute global %q", s.set.Shortcuts["player.repeat"], s.set.GlobalShortcuts["player.mute"])
	}

	click(t, s, tt, "change the global shortcut of Next track")
	tt.Key(0, ui.KeyEscape)
	if s.recording != "" || s.set.GlobalShortcuts["player.next"] != "Mod+Alt+P" {
		t.Errorf("Escape: recording %q, keys %q", s.recording, s.set.GlobalShortcuts["player.next"])
	}
	click(t, s, tt, "change the global shortcut of Next track")
	tt.Key(0, ui.KeyBackspace)
	if _, ok := s.set.GlobalShortcuts["player.next"]; ok || len(h.registered()) != 0 {
		t.Errorf("Backspace leaves %v, registered %v", s.set.GlobalShortcuts, h.registered())
	}
	click(t, s, tt, "set the global shortcut of Show MixMusic")
	tt.Key(ui.Cmd|ui.Alt, ui.KeyM)
	click(t, s, tt, "remove the global shortcut of Show MixMusic")
	if len(s.set.GlobalShortcuts) != 0 || len(h.registered()) != 0 {
		t.Errorf("remove leaves %v, registered %v", s.set.GlobalShortcuts, h.registered())
	}

	click(t, s, tt, "set the global shortcut of Like what plays")
	tt.Key(ui.Cmd|ui.Alt, ui.KeyL)
	click(t, s, tt, "reset all")
	if len(s.set.Shortcuts) != 0 || len(s.set.GlobalShortcuts) != 0 || len(h.registered()) != 0 {
		t.Errorf("reset all leaves %v and %v, registered %v", s.set.Shortcuts, s.set.GlobalShortcuts, h.registered())
	}
}

// TestGlobalShortcutInUse tells, on the row, of keys the system refused,
// until they register as MixMusic tries again.
func TestGlobalShortcutInUse(t *testing.T) {
	h := newTestHotkeys()
	h.fail[globalAcc("Alt+V")] = errors.New(`mygo: cannot register "Ctrl+Alt+V": Hot key is already registered.`)
	s, tt := newGlobalShell(t, h)
	click(t, s, tt, "set the global shortcut of Volume up")
	tt.Key(ui.Cmd|ui.Alt, ui.KeyV)
	s.drain()
	tt.Frame()
	inUse := "These keys are already in use: pick others, or try again once they're free"
	if !tt.HasText(inUse) || !tt.HasText("try again") || s.set.GlobalShortcuts["player.volumeUp"] != "Mod+Alt+V" {
		t.Fatalf("keys in use: %q", tt.Texts())
	}
	h.mu.Lock()
	delete(h.fail, globalAcc("Alt+V"))
	h.mu.Unlock()
	click(t, s, tt, "try again")
	tt.Advance(5e9) // the toast goes
	if tt.HasText(inUse) || !slices.Equal(h.registered(), []string{globalAcc("Alt+V")}) {
		t.Errorf("try again: %q, registered %v", tt.Texts(), h.registered())
	}
}

// TestGlobalShortcutPressRecords hands MixMusic's own global keys, which
// never reach the window, to the recorder, and leaves them to their
// command otherwise.
func TestGlobalShortcutPressRecords(t *testing.T) {
	h := newTestHotkeys()
	s, tt := newGlobalShell(t, h)
	click(t, s, tt, "set the global shortcut of Play or pause")
	tt.Key(ui.Cmd|ui.Alt, ui.KeyP)
	if s.globalPress("player.toggle", "Mod+Alt+P") {
		t.Error("a press while nothing records runs no command")
	}

	click(t, s, tt, "set the global shortcut of Previous track")
	h.press(t, globalAcc("Alt+P"))
	tt.Frame()
	if g := s.set.GlobalShortcuts; g["player.previous"] != "Mod+Alt+P" || g["player.toggle"] != "" || s.recording != "" {
		t.Errorf("the press recorded %v, recording %q", g, s.recording)
	}
	click(t, s, tt, "change the shortcut of Mute")
	h.press(t, globalAcc("Alt+P"))
	if s.set.Shortcuts["player.mute"] != "Mod+Alt+P" || s.set.GlobalShortcuts["player.previous"] != "" {
		t.Errorf("the window's recorder: Mute %q, Previous track's global %q", s.set.Shortcuts["player.mute"], s.set.GlobalShortcuts["player.previous"])
	}

	// A recording the page left without ending takes no presses.
	s.recording = globalRecording + "player.next"
	show(s, tt, "/")
	if s.globalPress("player.toggle", "Mod+Alt+P") || s.recording != "" {
		t.Errorf("away from the page, the press was taken: recording %q", s.recording)
	}
}

// TestGlobalShortcutGivenAway gives a command none of the keys of its own
// shortcut that another command has as its global shortcut.
func TestGlobalShortcutGivenAway(t *testing.T) {
	s := newTestShellOf(t, func(o *app.Options) { o.Hotkeys = newTestHotkeys() })
	s.set.GlobalShortcuts = map[string]string{"player.mute": "Mod+S"}
	if got := s.shortcutOf(command{id: "player.shuffle", shortcut: "Mod+S"}); got != "" {
		t.Errorf("Shuffle keeps Mute's global keys: %q", got)
	}
	if got := s.shortcutOf(command{id: "player.mute", shortcut: "M"}); got != "M" {
		t.Errorf("Mute's own keys: %q", got)
	}
}

func TestGlobalHint(t *testing.T) {
	for _, goos := range []string{"windows", "linux", "darwin"} {
		if hint := globalHint(goos); strings.Contains(strings.ToLower(hint), "other app") || strings.Contains(hint, "another app") {
			t.Errorf("%s: %q", goos, hint)
		}
	}
	if !strings.Contains(globalHint("linux"), "Wayland") {
		t.Error("Linux's hint doesn't tell of Wayland")
	}
}

// TestRenderGlobalShortcuts draws Settings › Shortcuts with a global
// shortcut set, and one the system refused, to MIXMUSIC_RENDER.
func TestRenderGlobalShortcuts(t *testing.T) {
	dir := os.Getenv("MIXMUSIC_RENDER")
	if dir == "" {
		t.Skip("MIXMUSIC_RENDER names no folder")
	}
	h := newTestHotkeys()
	h.fail[globalAcc("Alt+N")] = errors.New("in use")
	s := newTestShellOf(t, func(o *app.Options) { o.Hotkeys = h })
	st := s.svc.Preferences.Get()
	st.GlobalShortcuts = map[string]string{"player.toggle": "Mod+Alt+P", "player.next": "Mod+Alt+N"}
	if _, err := s.svc.Preferences.Set(st); err != nil {
		t.Fatal(err)
	}
	s.refresh()
	tt := testerOf(t, s, 1280, 1500)
	show(s, tt, "/settings/shortcuts")
	renderTo(t, s, tt, dir, "settings-shortcuts")
}
