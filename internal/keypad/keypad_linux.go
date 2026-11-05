//go:build linux && (amd64 || arm64)

package keypad

// GTK hands a key press to the window before the widget with the focus,
// MyGo's surface: a handler of the window's key-press-event, connected
// after MyGo's own, which passes this key on, sees the press first and may
// take it. Only the window with the keyboard's focus gets keys. GObject is
// called with purego, as MyGo calls it.

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/ebitengine/purego"
)

const (
	gdkKeyPress = 8
	gdkKPAdd    = 0xffab

	gdkControlMask = 1 << 2
	// otherMods are Alt's mask and the Windows key's: Mod4 on X11, and
	// GDK's Super, Hyper and Meta.
	otherMods = 1<<3 | 1<<6 | 1<<26 | 1<<27 | 1<<28
)

// gdkEventKey is the start of GTK 3's GdkEventKey, as 64-bit systems lay
// it out.
type gdkEventKey struct {
	typ       int32
	window    uintptr
	sendEvent int8
	time      uint32
	state     uint32
	keyval    uint32
}

var (
	gSignalConnectData func(instance uintptr, signal string, handler, data, destroy uintptr, flags int32) uint64

	loadGObject = sync.OnceValue(func() error {
		lib, err := purego.Dlopen("libgobject-2.0.so.0", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			return fmt.Errorf("keypad: GObject: %w", err)
		}
		purego.RegisterLibFunc(&gSignalConnectData, lib, "g_signal_connect_data")
		return nil
	})
	// keyPress is keyPressed as a callback, which purego makes a limited
	// number of: once.
	keyPress = sync.OnceValue(func() uintptr { return purego.NewCallback(keyPressed) })

	// watchers are what runs at a press, by the number of their Watch,
	// which the handler is given: a window's address may come back.
	watchers  sync.Map
	lastWatch atomic.Uintptr
)

func watch(win uintptr, plus func() bool) (func(), error) {
	if win == 0 {
		return nil, errors.New("keypad: no such window")
	}
	if err := loadGObject(); err != nil {
		return nil, err
	}
	id := lastWatch.Add(1)
	watchers.Store(id, plus)
	if gSignalConnectData(win, "key-press-event", keyPress(), id, 0, 0) == 0 {
		watchers.Delete(id)
		return nil, errors.New("keypad: no key-press-event on the window")
	}
	// The handler stays, and passes every key on: the window may be gone
	// at stop, and its handlers with it.
	return func() { watchers.Delete(id) }, nil
}

// keyPressed is the handler of the window's key-press-event, on GTK's
// thread.
func keyPressed(_ uintptr, event *gdkEventKey, id uintptr) bool {
	if !plusPressed(event) {
		return false
	}
	plus, ok := watchers.Load(id)
	// Taken, the press goes no further: not to the surface, nor its input
	// method, which would type a "+".
	return ok && plus.(func() bool)()
}

// plusPressed tells whether event is a press of the keypad's + with Ctrl
// held, without Alt or the Windows key: held as it repeats too.
func plusPressed(event *gdkEventKey) bool {
	return event.typ == gdkKeyPress && event.keyval == gdkKPAdd &&
		event.state&gdkControlMask != 0 && event.state&otherMods == 0
}
