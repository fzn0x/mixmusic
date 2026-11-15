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

func TestNothingStartsAfterClose(t *testing.T) {
	h := newTestHost(t)
	h.install("late", manifest("late"), map[string]string{"index.js": `
export default (api) => api.ui.toast("started");`})
	h.Close()
	h.enable("late")
	if h.Running("late") {
		t.Error("a plugin started after Close")
	}
}

func TestCloseKeepsWhatStopFunctionsStore(t *testing.T) {
	h := newTestHost(t)
	h.install("keeper", manifest("keeper"), map[string]string{"index.js": `
export default (api) => () => { api.storage.set("last", "stopped"); api.storage.setJSON("count", 3) };`})
	h.enable("keeper")
	h.Close()
	values, err := h.a.Services().Addons.Storage("keeper")
	if err != nil || values["last"] != "stopped" || values["count"] != "3" {
		t.Errorf("after Close, the plugin keeps %q, %v", values, err)
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
