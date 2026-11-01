package app

import (
	"runtime"
	"sync"
	"time"

	"github.com/egoist/mygo"
)

// trayWindow is what closing the main window to the tray asks of it:
// *mygo.Window has it, and tests fake it.
type trayWindow interface {
	IsFullScreen() bool
	SetFullScreen(bool)
	Hide()
}

// leaveFullScreenWait is how long the main window gets to leave full
// screen before it hides in the tray all the same, once it is out.
const leaveFullScreenWait = 3 * time.Second

// The states of trayHide.
const (
	trayShown   = iota // nothing on its way
	trayLeaving        // asked to leave full screen, to hide once out
	trayHidden         // hid out of full screen, which it goes back to
)

// The full-screen events trayHide saw last.
const (
	seenNone    = iota // none since the window opened
	seenEntered        // OnEnterFullScreen
	seenLeft           // OnLeaveFullScreen
)

// trayHide hides the main window in the tray, as closing it does with
// Close to the tray on (#84). On macOS a window in full screen is alone on
// a Space of its own, which hiding it would leave empty and black: it
// leaves full screen first, and hides once AppKit's animation has ended
// (left). Shown again, it goes back to full screen (shown). Windows and
// Linux hide it at once.
//
// The zero value is ready. What it puts off runs on the main thread, as
// its window's events do, so that nothing shows the window between a look
// at the state and the hide. Window methods are never called with mu held:
// from another goroutine they wait for the main thread, which may be
// waiting for mu.
type trayHide struct {
	// goos is the system, runtime.GOOS when empty, and after runs fn on
	// the main thread after d when nil: tests set them.
	goos  string
	after func(d time.Duration, fn func())

	mu sync.Mutex
	// state is trayShown, trayLeaving or trayHidden; gen counts its
	// changes, so that what an older one put off finds it is stale.
	state, gen int
	// seen is the full-screen event that came last, and toggled is set
	// once the window leaving was asked to leave full screen.
	seen    int
	toggled bool
}

// spaces reports whether a window in full screen has a Space of its own.
func (h *trayHide) spaces() bool {
	goos := h.goos
	if goos == "" {
		goos = runtime.GOOS
	}
	return goos == "darwin"
}

func (h *trayHide) later(d time.Duration, fn func()) {
	if h.after != nil {
		h.after(d, fn)
		return
	}
	time.AfterFunc(d, func() { mygo.RunOnMain(fn) })
}

// set moves to state and returns the new generation.
func (h *trayHide) set(state int) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.state = state
	h.gen++
	return h.gen
}

// is reports whether the state is still state, at generation gen.
func (h *trayHide) is(state, gen int) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.state == state && h.gen == gen
}

// hide hides w, which closes to the tray: on macOS, out of full screen
// first. A close while it leaves full screen adds nothing. Main thread.
func (h *trayHide) hide(w trayWindow) {
	if h.pending() {
		return
	}
	if !h.spaces() || !w.IsFullScreen() {
		h.set(trayShown) // hidden windowed, it comes back windowed
		w.Hide()
		return
	}
	h.mu.Lock()
	h.state = trayLeaving
	h.gen++
	gen := h.gen
	// In full screen with OnLeaveFullScreen last, the window still goes
	// into full screen: AppKit doesn't take a toggle in the middle of its
	// animation, so it leaves once in.
	h.toggled = h.seen != seenLeft
	toggle := h.toggled
	h.mu.Unlock()
	if toggle {
		w.SetFullScreen(false)
	}
	h.later(leaveFullScreenWait, func() { h.waited(w, gen) })
}

// left is w's OnLeaveFullScreen: a window that left full screen to close
// to the tray hides now, on the next turn of the main loop, once AppKit is
// done with its notification. Main thread.
func (h *trayHide) left(w trayWindow) {
	h.mu.Lock()
	h.seen = seenLeft
	leaving := h.state == trayLeaving
	if leaving {
		h.state = trayHidden
		h.gen++
	}
	gen := h.gen
	h.mu.Unlock()
	if !leaving {
		return // left by the user
	}
	h.later(0, func() {
		if h.is(trayHidden, gen) {
			w.Hide()
		}
	})
}

// entered is w's OnEnterFullScreen: a window closed as it went into full
// screen leaves it now. Main thread.
func (h *trayHide) entered(w trayWindow) {
	h.mu.Lock()
	h.seen = seenEntered
	toggle := h.state == trayLeaving && !h.toggled
	if toggle {
		h.toggled = true
		h.gen++
	}
	gen := h.gen
	h.mu.Unlock()
	if !toggle {
		return
	}
	w.SetFullScreen(false)
	h.later(leaveFullScreenWait, func() { h.waited(w, gen) })
}

// waited runs once w had leaveFullScreenWait to leave full screen, without
// a word of it. Not yet asked to, as no event came, it is asked now, and
// gets another wait. Out, it hides; still in, it stays, as if never
// closed: a close that didn't take is better than a black Space. Main
// thread.
func (h *trayHide) waited(w trayWindow, gen int) {
	full := w.IsFullScreen()
	h.mu.Lock()
	if h.state != trayLeaving || h.gen != gen {
		h.mu.Unlock()
		return
	}
	toggle := !h.toggled && full
	switch {
	case toggle:
		h.toggled = true
	case full:
		h.state = trayShown
	default:
		h.state = trayHidden
	}
	h.gen++
	gen = h.gen
	h.mu.Unlock()
	switch {
	case toggle:
		w.SetFullScreen(false)
		h.later(leaveFullScreenWait, func() { h.waited(w, gen) })
	case !full:
		w.Hide()
	}
}

// shown is told that the window shows again, and reports whether it goes
// back to full screen: when it hid out of it, and wasn't visible before it
// showed, as the system can show it too. Shown as it leaves full screen, it
// stays out, and shown: the hide on its way is called off. Any goroutine.
func (h *trayHide) shown(wasVisible bool) (fullScreen bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	fullScreen = h.state == trayHidden && !wasVisible
	h.state = trayShown
	h.gen++
	return fullScreen
}

// pending reports whether the window leaves full screen to hide.
func (h *trayHide) pending() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.state == trayLeaving
}

// forget forgets the window, which closed.
func (h *trayHide) forget() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.state, h.seen, h.toggled = trayShown, seenNone, false
	h.gen++
}
