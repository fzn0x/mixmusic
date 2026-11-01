package app

import (
	"fmt"
	"runtime"
	"slices"
	"sync"
	"testing"
	"time"
)

// fakeWindow records what trayHide asks of a window. Leaving and entering
// full screen take an animation, which the test ends by setting full.
type fakeWindow struct {
	mu    sync.Mutex
	full  bool
	calls []string
}

func (w *fakeWindow) IsFullScreen() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.full
}

func (w *fakeWindow) SetFullScreen(v bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.calls = append(w.calls, fmt.Sprintf("SetFullScreen(%v)", v))
}

func (w *fakeWindow) Hide() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.calls = append(w.calls, "Hide")
}

func (w *fakeWindow) setFull(v bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.full = v
}

func (w *fakeWindow) got() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return slices.Clone(w.calls)
}

// fakeTimers keeps what trayHide puts off, for the test to run.
type fakeTimers struct {
	waits []time.Duration
	fns   []func()
}

func (t *fakeTimers) after(d time.Duration, fn func()) {
	t.waits = append(t.waits, d)
	t.fns = append(t.fns, fn)
}

// run runs what was put off by d, in order, once; not what that puts off.
func (t *fakeTimers) run(d time.Duration) {
	for i, n := 0, len(t.fns); i < n; i++ {
		if fn := t.fns[i]; fn != nil && t.waits[i] == d {
			t.fns[i] = nil
			fn()
		}
	}
}

func newHide(goos string) (*trayHide, *fakeTimers) {
	t := &fakeTimers{}
	return &trayHide{goos: goos, after: t.after}, t
}

func wantCalls(t *testing.T, w *fakeWindow, calls ...string) {
	t.Helper()
	if got := w.got(); !slices.Equal(got, calls) {
		t.Fatalf("window got %q, want %q", got, calls)
	}
}

// closeInFullScreen closes w in full screen, and has it leave.
func closeInFullScreen(h *trayHide, timers *fakeTimers, w *fakeWindow) {
	w.setFull(true)
	h.hide(w)
	w.setFull(false)
	h.left(w)
	timers.run(0)
}

func TestTrayHideWindowed(t *testing.T) {
	h, _ := newHide("darwin")
	w := &fakeWindow{}
	h.hide(w)
	wantCalls(t, w, "Hide")
	if h.shown(false) {
		t.Error("a window hidden out of full screen goes to full screen")
	}
}

func TestTrayHideOtherSystemsHideAtOnce(t *testing.T) {
	for _, goos := range []string{"windows", "linux"} {
		h, _ := newHide(goos)
		w := &fakeWindow{full: true}
		h.hide(w)
		wantCalls(t, w, "Hide")
		if h.shown(false) {
			t.Errorf("%s: shown again, it goes back to full screen", goos)
		}
	}
}

func TestTrayHideLeavesFullScreenFirst(t *testing.T) {
	h, timers := newHide("darwin")
	w := &fakeWindow{full: true}
	h.hide(w)
	wantCalls(t, w, "SetFullScreen(false)")
	if !h.pending() {
		t.Error("no hide pending")
	}
	w.setFull(false)
	h.left(w)
	wantCalls(t, w, "SetFullScreen(false)") // not inside AppKit's notification
	timers.run(0)
	wantCalls(t, w, "SetFullScreen(false)", "Hide")
	timers.run(leaveFullScreenWait) // the wait, stale now
	wantCalls(t, w, "SetFullScreen(false)", "Hide")
	if !h.shown(false) {
		t.Error("shown again, it stays out of full screen")
	}
	if h.shown(false) {
		t.Error("shown a second time, it goes to full screen again")
	}
}

func TestTrayHideClosedTwice(t *testing.T) {
	h, timers := newHide("darwin")
	w := &fakeWindow{full: true}
	h.hide(w)
	h.hide(w)
	w.setFull(false)
	h.left(w)
	timers.run(0)
	wantCalls(t, w, "SetFullScreen(false)", "Hide")
}

func TestTrayHideShownWhileLeaving(t *testing.T) {
	h, timers := newHide("darwin")
	w := &fakeWindow{full: true}
	h.hide(w)
	if h.shown(true) {
		t.Error("shown as it leaves full screen, it goes back in")
	}
	w.setFull(false)
	h.left(w)
	timers.run(0)
	timers.run(leaveFullScreenWait)
	wantCalls(t, w, "SetFullScreen(false)")
}

func TestTrayHideShownBeforeTheHide(t *testing.T) {
	h, timers := newHide("darwin")
	w := &fakeWindow{full: true}
	h.hide(w)
	w.setFull(false)
	h.left(w)
	// Between the notification and the hide, the window is still on
	// screen, out of full screen: it stays so.
	if h.shown(true) {
		t.Error("shown on screen, it goes to full screen")
	}
	timers.run(0)
	wantCalls(t, w, "SetFullScreen(false)")
}

// TestTrayHideShownBySystem leaves a window the system showed, as AppKit's
// Window menu can, out of full screen.
func TestTrayHideShownBySystem(t *testing.T) {
	h, timers := newHide("darwin")
	w := &fakeWindow{}
	closeInFullScreen(h, timers, w)
	if h.shown(true) {
		t.Error("a window already on screen goes to full screen")
	}
}

// TestTrayHideWindowedAfterFullScreen comes back windowed once last closed
// windowed, though it closed in full screen before.
func TestTrayHideWindowedAfterFullScreen(t *testing.T) {
	h, timers := newHide("darwin")
	w := &fakeWindow{}
	closeInFullScreen(h, timers, w)
	// Shown by the system, then closed windowed.
	h.hide(w)
	if h.shown(false) {
		t.Errorf("a window last closed windowed goes back to full screen: %q", w.got())
	}
}

func TestTrayHideWaitRunsOut(t *testing.T) {
	h, timers := newHide("darwin")
	w := &fakeWindow{full: true}
	h.hide(w)
	w.setFull(false) // out, but AppKit never said
	timers.run(leaveFullScreenWait)
	wantCalls(t, w, "SetFullScreen(false)", "Hide")
	if !h.shown(false) {
		t.Error("shown again, it stays out of full screen")
	}

	h, timers = newHide("darwin")
	w = &fakeWindow{full: true}
	h.hide(w) // AppKit ignores it
	timers.run(leaveFullScreenWait)
	wantCalls(t, w, "SetFullScreen(false)") // still in full screen: it stays
	if h.pending() || h.shown(true) {
		t.Error("the close that didn't take is still on its way")
	}
	h.hide(w) // closing again tries again
	wantCalls(t, w, "SetFullScreen(false)", "SetFullScreen(false)")
}

func TestTrayHideStaleWait(t *testing.T) {
	h, timers := newHide("darwin")
	w := &fakeWindow{full: true}
	h.hide(w)
	first := timers.fns[0]
	timers.fns[0] = nil
	w.setFull(false)
	h.left(w)
	timers.run(0)
	h.shown(false)
	h.entered(w)
	w.setFull(true) // back in full screen
	h.hide(w)       // and closed again
	first()         // the first close's wait
	wantCalls(t, w, "SetFullScreen(false)", "Hide", "SetFullScreen(false)")
	if !h.pending() {
		t.Error("the first close's wait called off the second")
	}
}

func TestTrayHideLeftByHand(t *testing.T) {
	h, timers := newHide("darwin")
	w := &fakeWindow{}
	h.left(w)
	timers.run(0)
	wantCalls(t, w)
	if h.shown(false) {
		t.Error("left by hand, it goes back to full screen")
	}
}

// TestTrayHideClosedAsItEntered leaves full screen once in, when closed
// in the animation into it: one toggle, not one in the animation and
// another after.
func TestTrayHideClosedAsItEntered(t *testing.T) {
	h, timers := newHide("darwin")
	w := &fakeWindow{}
	h.left(w)       // out of full screen, by hand
	w.setFull(true) // and in again, still animating
	h.hide(w)
	wantCalls(t, w)
	if !h.pending() {
		t.Error("no hide pending")
	}
	h.entered(w)
	wantCalls(t, w, "SetFullScreen(false)")
	w.setFull(false)
	h.left(w)
	timers.run(0)
	timers.run(leaveFullScreenWait)
	wantCalls(t, w, "SetFullScreen(false)", "Hide")
}

// TestTrayHideClosedAsItEnteredUnheard asks the window to leave once the
// wait runs out, when AppKit tells of neither end of the animation.
func TestTrayHideClosedAsItEnteredUnheard(t *testing.T) {
	h, timers := newHide("darwin")
	w := &fakeWindow{}
	h.left(w)
	w.setFull(true)
	h.hide(w)
	timers.run(leaveFullScreenWait)
	wantCalls(t, w, "SetFullScreen(false)")
	if !h.pending() {
		t.Error("the hide was called off before the window was asked to leave")
	}
	// Still in full screen at the second wait: it stays.
	timers.run(leaveFullScreenWait)
	wantCalls(t, w, "SetFullScreen(false)")
	if h.pending() {
		t.Error("the close that didn't take is still on its way")
	}
}

func TestTrayHideEnteredByHand(t *testing.T) {
	h, _ := newHide("darwin")
	w := &fakeWindow{full: true}
	h.entered(w)
	wantCalls(t, w)
}

func TestTrayHideForget(t *testing.T) {
	h, timers := newHide("darwin")
	w := &fakeWindow{full: true}
	h.hide(w)
	h.forget()
	w.setFull(false)
	h.left(w)
	timers.run(0)
	timers.run(leaveFullScreenWait)
	wantCalls(t, w, "SetFullScreen(false)")
	if h.shown(false) {
		t.Error("a window that closed goes back to full screen")
	}
}

func TestTrayHideZeroValue(t *testing.T) {
	var h trayHide
	if h.spaces() != (runtime.GOOS == "darwin") {
		t.Errorf("spaces %v on %s", h.spaces(), runtime.GOOS)
	}
}

// TestTrayHideRace has the timers and showMain's goroutines run beside
// the main thread's calls, for -race.
func TestTrayHideRace(t *testing.T) {
	h := &trayHide{goos: "darwin", after: func(d time.Duration, fn func()) { go fn() }}
	w := &fakeWindow{full: true}
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 200 {
				h.shown(i%2 == 0)
				h.pending()
			}
		}()
	}
	for range 200 {
		w.setFull(true)
		h.hide(w)
		h.entered(w)
		w.setFull(false)
		h.left(w)
	}
	wg.Wait()
}
