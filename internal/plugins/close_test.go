package plugins

import (
	"testing"
	"time"
)

// Close is how MixMusic stops its plugins as it quits: a plugin's stop
// function has run once it returns, and one that never ends keeps it
// stopWait at most.

func TestCloseRunsStopFunctions(t *testing.T) {
	h := newTestHost(t)
	h.install("bye", manifest("bye"), map[string]string{"index.js": `
export default (api) => () => api.ui.toast("stopped");`})
	h.enable("bye")
	h.Close()
	if !h.ui.said("stopped") {
		t.Error("Close returned before the plugin's stop function ran")
	}
	if h.Running("bye") {
		t.Error("the plugin runs after Close")
	}
}

func TestCloseBoundsStuckStopFunctions(t *testing.T) {
	stopWait = 50 * time.Millisecond
	t.Cleanup(func() { stopWait = 3 * time.Second })
	h := newTestHost(t)
	h.install("stuck", manifest("stuck"), map[string]string{"index.js": `
export default (api) => () => { for (;;) {} };`})
	h.enable("stuck")
	h.mu.Lock()
	pl := h.running["stuck"]
	h.mu.Unlock()
	closed := make(chan struct{})
	go func() {
		h.Close()
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("Close waits on a stop function that never ends")
	}
	select {
	case <-pl.loop.done:
	case <-time.After(5 * time.Second):
		t.Fatal("the stuck stop function wasn't interrupted")
	}
}
