package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestRecordMemoryRow records MixMusic's memory use from Settings › About:
// a switch under the MixMusic group, off at first, which the keyboard
// reaches; while it's on, Save a heap profile saves one in the logs
// folder, which a toast names. A search for "memory" or "heap" finds it.
func TestRecordMemoryRow(t *testing.T) {
	s := newTestShell(t)
	t.Cleanup(func() { _ = s.svc.System.RecordMemory(false) })
	tt := testerOf(t, s, 1280, 2400)
	show(s, tt, "/settings/about")
	var above float32
	for _, text := range []string{"report a problem", recordMemoryTitle, recordMemoryAbout, "licenses"} {
		r, ok := tt.Find(text)
		if !ok || r.Y <= above {
			t.Fatalf("Settings › About shows %q out of place: %q", text, tt.Texts())
		}
		above = r.Y
	}
	if s.svc.System.RecordingMemory() || tt.HasText("save a heap profile") {
		t.Fatal("recording at first")
	}
	focused := func() bool { return tt.Focused(recordMemoryTitle) && !tt.Focused(recordMemoryAbout) }
	for i := 0; i < 200 && !focused(); i++ {
		tt.Key(0, ui.KeyTab)
	}
	if !focused() {
		t.Fatalf("Tab doesn't reach the switch: %q", tt.Texts())
	}
	tt.Key(0, ui.KeySpace)
	s.drain()
	tt.Frame()
	if !s.svc.System.RecordingMemory() {
		t.Fatal("the switch didn't start recording")
	}
	logs := s.svc.System.LogsFolder()
	if _, err := os.Stat(filepath.Join(logs, "memory.log")); err != nil {
		t.Errorf("no memory.log: %v", err)
	}
	click(t, s, tt, "save a heap profile")
	until(t, s, tt, "the profile", func() bool { return tt.HasText(".pprof in the logs folder") })
	files := s.svc.System.MemoryFiles()
	if len(files) != 2 || files[0] != "memory.log" || !strings.HasPrefix(files[1], "heap-") || !tt.HasText("Saved "+files[1]) {
		t.Errorf("the files %q; the window shows %q", files, tt.Texts())
	}
	// The click gave the button the focus.
	for i := 0; i < 200 && !focused(); i++ {
		tt.Key(0, ui.KeyTab)
	}
	tt.Key(0, ui.KeySpace)
	s.drain()
	tt.Frame()
	if s.svc.System.RecordingMemory() || tt.HasText("save a heap profile") {
		t.Errorf("turned off, still recording: %v", s.svc.System.RecordingMemory())
	}
	for _, q := range []string{"memory", "heap profile"} {
		show(s, tt, settingsPath("", q))
		if !tt.HasText(recordMemoryTitle) {
			t.Errorf("searching %q doesn't find the row: %q", q, tt.Texts())
		}
	}
	if !slices.Contains(s.svc.System.MemoryFiles(), "memory.log") {
		t.Error("memory.log went once off")
	}
}
