package app

// Global shortcuts: keys of the user's choosing that run commands whichever
// window is in front (#80), registered beside the media keys.

import (
	"errors"
	"log"
	"maps"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
)

// Hotkeys registers global shortcuts, as mygo.GlobalShortcut does: fn runs
// on the main thread at each press of acc, an accelerator such as
// "Ctrl+Alt+P".
type Hotkeys interface {
	Register(acc string, fn func()) error
	Unregister(acc string)
}

// GlobalCommand is a command a global shortcut can run: its ID, that of
// the window's command of the same name, and its title.
type GlobalCommand struct{ ID, Title string }

// GlobalCommands are the commands global shortcuts can run, in the order
// Settings › Shortcuts lists them: of two commands given the same keys,
// the first registers them.
var GlobalCommands = []GlobalCommand{
	{"player.toggle", "Play or pause"},
	{"player.next", "Next track"},
	{"player.previous", "Previous track"},
	{"player.volumeUp", "Volume up"},
	{"player.volumeDown", "Volume down"},
	{"player.mute", "Mute"},
	{"player.like", "Like what plays"},
	{"app.show", "Show MixMusic"},
}

// globalVolumeStep is how much a press of Volume up or Volume down turns
// the volume, as the window's keys do.
const globalVolumeStep = 0.05

// globalRepeat is how soon another press of a global shortcut is taken for
// its key held down, where the system repeats the presses of a key held:
// X11 does, while Windows and macOS report one press.
const globalRepeat = 300 * time.Millisecond

// globalShortcuts is what the app registered of the user's global
// shortcuts, which mu guards.
type globalShortcuts struct {
	mu sync.Mutex
	// applied are the keys of the settings the last pass registered, by
	// command ID; registered the accelerators it registered, and problems
	// why the keys of the others didn't register.
	applied, registered, problems map[string]string
	// busy is set while a pass registers, and again when another pass was
	// asked for meanwhile, with retry if it retries what failed.
	busy, again, retry bool
	// hook sees the presses first.
	hook *func(id, keys string) bool
	// repeats tells whether the system repeats the presses of a key held,
	// and last is when the keys of each command were pressed last, at now.
	repeats bool
	last    map[string]time.Time
	now     func() time.Time
	// run runs the command of a press; tests see the presses there.
	run func(id string)
}

// hotkeys is what registers the global shortcuts: the options' Hotkeys, or
// MyGo once the app is ready, or nil before.
func (a *App) hotkeys() Hotkeys {
	if a.opts.Hotkeys != nil {
		return a.opts.Hotkeys
	}
	if mygo.App.IsReady() {
		return mygo.GlobalShortcut
	}
	return nil
}

// applyGlobalShortcuts registers the global shortcuts of the settings, once
// they can be: those whose keys changed since the last pass, and with retry
// those that didn't register too. It tells whether the problems changed.
//
// Every save of the settings asks, from the main thread, or the player's
// goroutine as it keeps the volume, which changes no keys: it returns at
// once then. A single pass registers at a time, holding no lock as MyGo
// waits for the main thread: a pass asked for meanwhile runs after it, on
// the goroutine of the first, from the settings as they are by then.
func (a *App) applyGlobalShortcuts(retry bool) (changed bool) {
	hk := a.hotkeys()
	if hk == nil {
		return false
	}
	g := &a.global
	want := wantedGlobal(a.settings.Get().GlobalShortcuts)
	g.mu.Lock()
	// While a pass registers, applied is what the settings were: the
	// settings may have gone back to them since the pass read them.
	if g.busy {
		g.again, g.retry = true, g.retry || retry
		g.mu.Unlock()
		return false
	}
	if !retry && maps.Equal(g.applied, want) {
		g.mu.Unlock()
		return false
	}
	g.busy = true
	g.mu.Unlock()
	for {
		changed = a.globalPass(hk, retry) || changed
		g.mu.Lock()
		if !g.again {
			g.busy = false
			g.mu.Unlock()
			return changed
		}
		retry = g.retry
		g.again, g.retry = false, false
		g.mu.Unlock()
	}
}

// wantedGlobal are the global shortcuts of the settings that this version
// runs: of its commands, with keys.
func wantedGlobal(set map[string]string) map[string]string {
	want := map[string]string{}
	for _, gc := range GlobalCommands {
		if keys := set[gc.ID]; keys != "" {
			want[gc.ID] = keys
		}
	}
	return want
}

// globalPass unregisters the global shortcuts whose keys went or changed,
// and registers those not registered, in the commands' order. It tells
// whether the problems changed.
func (a *App) globalPass(hk Hotkeys, retry bool) bool {
	g := &a.global
	want := wantedGlobal(a.settings.Get().GlobalShortcuts)
	g.mu.Lock()
	if !retry && maps.Equal(g.applied, want) {
		g.mu.Unlock()
		return false
	}
	registered, before := maps.Clone(g.registered), g.problems
	g.mu.Unlock()
	if registered == nil {
		registered = map[string]string{}
	}
	// What the keys of each command register as: none for keys that can't
	// be global, or that a command before it has.
	accs, problems, taken := map[string]string{}, map[string]string{}, map[string]bool{}
	for _, gc := range GlobalCommands {
		keys := want[gc.ID]
		if keys == "" {
			continue
		}
		if p := GlobalKeysProblem(keys, runtime.GOOS); p != "" {
			problems[gc.ID] = p
			continue
		}
		acc, _ := globalAccelerator(keys, runtime.GOOS)
		if taken[acc] {
			problems[gc.ID] = "Another command has these keys"
			continue
		}
		accs[gc.ID], taken[acc] = acc, true
	}
	// The keys of a command that gave them up may be another's now.
	for id, acc := range registered {
		if accs[id] != acc {
			hk.Unregister(acc)
			delete(registered, id)
		}
	}
	for _, gc := range GlobalCommands {
		id, acc := gc.ID, accs[gc.ID]
		if acc == "" || registered[id] == acc {
			continue
		}
		if err := hk.Register(acc, func() { a.globalPressed(id) }); err != nil {
			log.Printf("mixmusic: global shortcut %s: %v", acc, err)
			problems[id] = globalShortcutError(err, runtime.GOOS)
			continue
		}
		registered[id] = acc
	}
	g.mu.Lock()
	g.applied, g.registered, g.problems = want, registered, problems
	g.mu.Unlock()
	return !maps.Equal(before, problems)
}

// globalShortcutError tells, in the words of Settings › Shortcuts, why the
// system refused the keys of a global shortcut, as err says.
func globalShortcutError(err error, goos string) string {
	switch {
	case errors.Is(err, errors.ErrUnsupported):
		// Wayland, where the desktop has no GlobalShortcuts portal.
		return "Your desktop doesn't offer global shortcuts"
	case goos == "linux" && strings.Contains(err.Error(), "desktop entry"):
		// The portal answers NotAllowed to an app it can't name.
		return "Global shortcuts need MixMusic installed with its desktop entry"
	}
	// MyGo tells no more than that the system refused them.
	return "These keys are already in use: pick others, or try again once they're free"
}

// globalPressed runs the command of the global shortcut of id, pressed, on
// the main thread, unless the hook takes the press. A press too soon after
// the last is the key held, where the system repeats them; the volume's
// keys go on turning it.
func (a *App) globalPressed(id string) {
	g := &a.global
	g.mu.Lock()
	keys, hook, run := g.applied[id], g.hook, g.run
	held := false
	if g.repeats && id != "player.volumeUp" && id != "player.volumeDown" {
		now := time.Now()
		if g.now != nil {
			now = g.now()
		}
		last, ok := g.last[id]
		held = ok && now.Sub(last) < globalRepeat
		if g.last == nil {
			g.last = map[string]time.Time{}
		}
		g.last[id] = now
	}
	g.mu.Unlock()
	switch {
	case held:
	case hook != nil && (*hook)(id, keys):
	case run != nil:
		run(id)
	default:
		a.globalCommand(id)
	}
}

// globalCommand runs the command of a global shortcut.
func (a *App) globalCommand(id string) {
	switch id {
	case "player.toggle":
		a.command(PlayerCommand{Command: CommandToggle})
	case "player.next":
		a.command(PlayerCommand{Command: CommandNext})
	case "player.previous":
		a.command(PlayerCommand{Command: CommandPrevious})
	case "player.volumeUp":
		a.command(PlayerCommand{Command: CommandVolumeBy, Value: globalVolumeStep})
	case "player.volumeDown":
		a.command(PlayerCommand{Command: CommandVolumeBy, Value: -globalVolumeStep})
	case "player.mute":
		a.command(PlayerCommand{Command: CommandMute})
	case "player.like":
		a.command(PlayerCommand{Command: CommandLike})
	case "app.show":
		// At once, in the press's callback: on Wayland the window shown
		// then takes the focus with the press's activation token.
		a.showMain()
	}
}

// OnGlobalShortcut has fn see each press of a global shortcut first, on
// the main thread, until cancel: the command's ID and its keys, as the
// settings write them. A press fn takes, returning true, runs no command:
// so the window that records the keys of shortcuts takes MixMusic's own
// global keys, which never reach it.
func (a *App) OnGlobalShortcut(fn func(id, keys string) bool) (cancel func()) {
	f := &fn
	g := &a.global
	g.mu.Lock()
	g.hook = f
	g.mu.Unlock()
	return func() {
		g.mu.Lock()
		if g.hook == f {
			g.hook = nil
		}
		g.mu.Unlock()
	}
}

// GlobalShortcutProblem tells why the keys of the global shortcut of the
// command id didn't register, or "".
func (p *Preferences) GlobalShortcutProblem(id string) string {
	g := &p.a.global
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.problems[id]
}

// RetryGlobalShortcuts registers again the global shortcuts that didn't
// register, whose keys may be free now.
func (p *Preferences) RetryGlobalShortcuts() {
	if p.a.applyGlobalShortcuts(true) {
		notify()
	}
}

// globalKeys are the keys of a global shortcut: its modifiers, as the
// system has them, and its key, as MyGo names it.
type globalKeys struct {
	super, ctrl, alt, shift bool
	key                     string
}

// globalKeyNames are MyGo's names of the keys whose names in shortcuts
// differ.
var globalKeyNames = map[string]string{"ArrowLeft": "Left", "ArrowRight": "Right", "ArrowUp": "Up", "ArrowDown": "Down"}

// namedKeys are the keys of shortcuts that have names of more than a
// character, besides the arrows.
var namedKeys = map[string]bool{
	"Space": true, "Enter": true, "Escape": true, "Tab": true, "Backspace": true, "Delete": true,
	"Home": true, "End": true, "PageUp": true, "PageDown": true,
	"F1": true, "F2": true, "F3": true, "F4": true, "F5": true, "F6": true,
	"F7": true, "F8": true, "F9": true, "F10": true, "F11": true, "F12": true,
}

// parseGlobalKeys reads keys as the settings write shortcuts, "Mod+Alt+P",
// on goos: Mod is ⌘ on macOS and Ctrl elsewhere, and Meta or Cmd the
// Windows or Super key, ⌘ on macOS.
func parseGlobalKeys(keys, goos string) (globalKeys, bool) {
	var k globalKeys
	parts := strings.Split(keys, "+")
	name := parts[len(parts)-1]
	switch {
	case globalKeyNames[name] != "":
		k.key = globalKeyNames[name]
	case namedKeys[name]:
		k.key = name
	case len(name) == 1 && (name[0] >= 'A' && name[0] <= 'Z' || name[0] >= '0' && name[0] <= '9' || strings.Contains(",./;-=[]\\`'", name)):
		k.key = name
	default:
		// "Mod++" too, whose plus has no name in shortcuts.
		return globalKeys{}, false
	}
	for _, p := range parts[:len(parts)-1] {
		switch p {
		case "Mod":
			if goos == "darwin" {
				k.super = true
			} else {
				k.ctrl = true
			}
		case "Ctrl":
			k.ctrl = true
		case "Alt":
			k.alt = true
		case "Shift":
			k.shift = true
		case "Meta", "Cmd":
			k.super = true
		default:
			return globalKeys{}, false
		}
	}
	return k, true
}

// accelerator writes keys as MyGo's canonical accelerators are, which
// keys written two ways share: "Ctrl+Alt+P".
func (k globalKeys) accelerator() string {
	var parts []string
	for _, m := range []struct {
		on   bool
		name string
	}{{k.super, "Super"}, {k.ctrl, "Ctrl"}, {k.alt, "Alt"}, {k.shift, "Shift"}} {
		if m.on {
			parts = append(parts, m.name)
		}
	}
	return strings.Join(append(parts, k.key), "+")
}

// globalAccelerator is the accelerator MyGo registers keys as on goos.
func globalAccelerator(keys, goos string) (string, bool) {
	k, ok := parseGlobalKeys(keys, goos)
	if !ok {
		return "", false
	}
	return k.accelerator(), true
}

// GlobalKeysProblem tells why keys, written as the settings write
// shortcuts, can't be a global shortcut on goos, or "" when they can. A
// global shortcut takes its keys from every window, so it needs Ctrl, Alt
// or the Windows key, or on macOS ⌘ or ⌃, as ⌥ with a key types a
// character there; F1 to F12 may stand alone, but F12 on Windows.
func GlobalKeysProblem(keys, goos string) string {
	k, ok := parseGlobalKeys(keys, goos)
	switch {
	case !ok, k.key == "Escape", k.key == "Tab", k.key == "Backspace", k.key == "Delete":
		return "These keys can't be a global shortcut"
	case len(k.key) > 1 && k.key[0] == 'F':
		if goos == "windows" && k.key == "F12" && !k.super && !k.ctrl && !k.alt && !k.shift {
			return "Windows keeps F12 for itself"
		}
	case goos == "darwin":
		if !k.super && !k.ctrl {
			return "A global shortcut needs ⌘ or ⌃"
		}
	case !k.super && !k.ctrl && !k.alt:
		if goos == "windows" {
			return "A global shortcut needs Ctrl, Alt or the Windows key"
		}
		return "A global shortcut needs Ctrl, Alt or Super"
	}
	return ""
}
