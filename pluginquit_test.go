package main

import (
	"os"
	"path/filepath"
	"testing"
)

// As MixMusic quits, the app saves what waits to be saved, and stops the
// plugins first: their stop functions run, and what they keep is kept.
func TestPluginsStopAtQuit(t *testing.T) {
	s := newTestShell(t)
	dir := filepath.Join(s.svc.System.Info().DataDir, "addons", "plugins", "bye")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"plugin.json": `{"id": "bye", "name": "Bye", "version": "1.0.0", "author": "A", "main": "index.js", "api": 2}`,
		"index.js":    `export default (api) => () => api.storage.set("quit", "stopped");`,
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s.svc.Addons.Reload()
	h := pluginHost(s.a, s.p)
	h.Sync([]string{"bye"}, s.svc.Addons.List().Plugins)
	if !h.Running("bye") {
		t.Fatalf("the plugin doesn't run: %s", h.Failure("bye"))
	}
	// What MixMusic saves as it quits, as the test's cleanup does again.
	s.a.Close()
	if h.Running("bye") {
		t.Error("the plugin runs after the quit")
	}
	values, err := s.svc.Addons.Storage("bye")
	if err != nil || values["quit"] != "stopped" {
		t.Errorf("the plugin's stop function kept %q, %v", values, err)
	}
}
