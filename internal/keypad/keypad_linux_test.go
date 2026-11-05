//go:build linux && (amd64 || arm64)

package keypad

import (
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"

	"github.com/ebitengine/purego"
)

// TestPlusPressed takes a press of the keypad's + with Ctrl, with Shift or
// Num Lock too, and nothing else: its release, other keys, and Ctrl with
// Alt or the Windows key, which other shortcuts use.
func TestPlusPressed(t *testing.T) {
	const release, kpSubtract, shift, numLock = 9, 0xffad, 1 << 0, 1 << 4
	for _, c := range []struct {
		what   string
		typ    int32
		keyval uint32
		state  uint32
		want   bool
	}{
		{"Ctrl and +", gdkKeyPress, gdkKPAdd, gdkControlMask, true},
		{"Ctrl, Shift and +", gdkKeyPress, gdkKPAdd, gdkControlMask | shift, true},
		{"Ctrl and + with Num Lock on", gdkKeyPress, gdkKPAdd, gdkControlMask | numLock, true},
		{"+ let go", release, gdkKPAdd, gdkControlMask, false},
		{"+ alone", gdkKeyPress, gdkKPAdd, 0, false},
		{"Ctrl, Alt and +", gdkKeyPress, gdkKPAdd, gdkControlMask | 1<<3, false},
		{"Ctrl, Windows and + on X11", gdkKeyPress, gdkKPAdd, gdkControlMask | 1<<6, false},
		{"Ctrl, Super and +", gdkKeyPress, gdkKPAdd, gdkControlMask | 1<<26, false},
		{"Ctrl and the keypad's −", gdkKeyPress, kpSubtract, gdkControlMask, false},
	} {
		event := &gdkEventKey{typ: c.typ, keyval: c.keyval, state: c.state}
		if got := plusPressed(event); got != c.want {
			t.Errorf("%s: %v, want %v", c.what, got, c.want)
		}
	}
}

// GTK, to make a window and turn its main loop, as MyGo does.
var (
	gtkInitCheck            func(argc, argv uintptr) bool
	gtkWindowNew            func(kind int32) uintptr
	gtkWindowSetTitle       func(win uintptr, title string)
	gtkWindowSetDefaultSize func(win uintptr, width, height int32)
	gtkWidgetShowAll        func(widget uintptr)
	gtkWidgetDestroy        func(widget uintptr)
	gtkEventsPending        func() bool
	gtkMainIterationDo      func(blocking bool) bool
)

// TestWatch presses keys in a GTK window, as a keyboard does, with
// xdotool. Ctrl and the keypad's + go to plus, and no further while it
// takes them. Other keys, those plus leaves, and every key after stop, go
// on to a handler of the window connected later, as MyGo's surface gets
// them after the window.
func TestWatch(t *testing.T) {
	if _, err := Watch(0, func() bool { return true }); err == nil {
		t.Error("watched no window")
	}
	if os.Getenv("DISPLAY") == "" {
		t.Skip("no X display: run the test under xvfb-run")
	}
	if _, err := exec.LookPath("xdotool"); err != nil {
		t.Skip("no xdotool, which presses the keys")
	}
	// GTK keeps to the thread it started on.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	gtk, err := purego.Dlopen("libgtk-3.so.0", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		t.Skip(err)
	}
	for name, fn := range map[string]any{
		"gtk_init_check":              &gtkInitCheck,
		"gtk_window_new":              &gtkWindowNew,
		"gtk_window_set_title":        &gtkWindowSetTitle,
		"gtk_window_set_default_size": &gtkWindowSetDefaultSize,
		"gtk_widget_show_all":         &gtkWidgetShowAll,
		"gtk_widget_destroy":          &gtkWidgetDestroy,
		"gtk_events_pending":          &gtkEventsPending,
		"gtk_main_iteration_do":       &gtkMainIterationDo,
	} {
		purego.RegisterLibFunc(fn, gtk, name)
	}
	if !gtkInitCheck(0, 0) {
		t.Fatal("GTK can't open the display")
	}
	const title = "MixMusic keypad test"
	win := gtkWindowNew(0) // GTK_WINDOW_TOPLEVEL
	gtkWindowSetTitle(win, title)
	gtkWindowSetDefaultSize(win, 200, 200)
	gtkWidgetShowAll(win)
	defer gtkWidgetDestroy(win)

	// The callbacks run on this thread, in gtkMainIterationDo.
	take := true
	var calls, passed int
	stop, err := Watch(win, func() bool {
		calls++
		return take
	})
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	later := purego.NewCallback(func(_ uintptr, event *gdkEventKey, _ uintptr) bool {
		// The keys pressed, not Shift, Ctrl, Alt or Super.
		if event.keyval < 0xffe1 || event.keyval > 0xffee {
			passed++
		}
		return false
	})
	if gSignalConnectData(win, "key-press-event", later, 0, 0, 0) == 0 {
		t.Fatal("no key-press-event on the window")
	}

	// turn turns GTK's main loop until done, for 5 seconds at most.
	turn := func(done func() bool) bool {
		for end := time.Now().Add(5 * time.Second); !done(); time.Sleep(time.Millisecond) {
			if time.Now().After(end) {
				return false
			}
			for gtkEventsPending() {
				gtkMainIterationDo(false)
			}
		}
		return true
	}
	xdotool := func(args ...string) {
		t.Helper()
		type result struct {
			out []byte
			err error
		}
		ran := make(chan result, 1)
		go func() {
			out, err := exec.Command("xdotool", args...).CombinedOutput()
			ran <- result{out, err}
		}()
		var r *result
		if !turn(func() bool {
			select {
			case got := <-ran:
				r = &got
			default:
			}
			return r != nil
		}) {
			t.Fatalf("xdotool %v: still running", args)
		}
		if r.err != nil {
			t.Fatalf("xdotool %v: %v: %s", args, r.err, r.out)
		}
	}
	xdotool("search", "--sync", "--onlyvisible", "--name", "^"+title+"$", "windowfocus", "--sync")

	press := func(keys string, wantCalls, wantPassed int) {
		t.Helper()
		c0, p0 := calls, passed
		xdotool("key", keys)
		if !turn(func() bool { return calls > c0 || passed > p0 }) {
			t.Fatalf("%s: the window got no key", keys)
		}
		if calls-c0 != wantCalls || passed-p0 != wantPassed {
			t.Errorf("%s: plus ran %d times and the key went on %d, want %d and %d",
				keys, calls-c0, passed-p0, wantCalls, wantPassed)
		}
	}
	press("ctrl+KP_Add", 1, 0)
	press("ctrl+shift+KP_Add", 1, 0)
	press("KP_Add", 0, 1)
	press("ctrl+alt+KP_Add", 0, 1)
	press("ctrl+super+KP_Add", 0, 1)
	press("ctrl+KP_Subtract", 0, 1)
	take = false
	press("ctrl+KP_Add", 1, 1)
	take = true
	stop()
	press("ctrl+KP_Add", 0, 1)
}
