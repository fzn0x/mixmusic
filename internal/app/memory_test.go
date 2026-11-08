package app

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestRecordMemory records MixMusic's memory use in the logs folder while
// it's on, with a heap profile when asked, which a report of a problem
// then tells of; a new start of MixMusic records nothing.
func TestRecordMemory(t *testing.T) {
	logs := filepath.Join(t.TempDir(), "logs")
	a := newQuietApp(t, func(o *Options) { o.LogsDir = logs })
	s := &System{a}
	if s.RecordingMemory() || len(s.MemoryFiles()) != 0 {
		t.Fatalf("recording before it was asked: %v, %q", s.RecordingMemory(), s.MemoryFiles())
	}
	if err := s.RecordMemory(true); err != nil {
		t.Fatal(err)
	}
	if !s.RecordingMemory() {
		t.Fatal("not recording once asked")
	}
	var lines []string
	for deadline := time.Now().Add(5 * time.Second); len(lines) < 2; time.Sleep(5 * time.Millisecond) {
		b, _ := os.ReadFile(filepath.Join(logs, "memory.log"))
		lines = strings.Split(strings.TrimSpace(string(b)), "\n")
		if time.Now().After(deadline) {
			t.Fatalf("memory.log holds %q", lines)
		}
	}
	if !strings.Contains(lines[0], "Recording, MixMusic test on ") || !strings.Contains(lines[1], "Go heap ") {
		t.Errorf("memory.log starts %q", lines)
	}
	name, err := s.SaveHeapProfile()
	if err != nil || !strings.HasPrefix(name, "heap-") || !strings.HasSuffix(name, ".pprof") {
		t.Fatalf("saved %q, %v", name, err)
	}
	if files := s.MemoryFiles(); !slices.Equal(files, []string{"memory.log", name}) {
		t.Errorf("the files %q", files)
	}

	// A report of a problem tells of them.
	t.Setenv("PATH", "")
	u, err := url.Parse(s.ReportURL(context.Background()))
	if err != nil {
		t.Fatal(err)
	}
	if more := u.Query().Get("more"); !strings.Contains(more, "Recording memory use left memory.log, "+name+" in the same folder") ||
		!strings.Contains(more, "nothing of your music or accounts") {
		t.Errorf("the report:\n%s", more)
	}

	if err := s.RecordMemory(false); err != nil || s.RecordingMemory() {
		t.Errorf("turned off: %v, recording %v", err, s.RecordingMemory())
	}
	// MixMusic starts again: it records nothing, until asked.
	if again := (&System{newQuietApp(t, func(o *Options) { o.LogsDir = logs })}); again.RecordingMemory() {
		t.Error("recording at the next start")
	}
}

// TestRecordMemoryWithoutLogs tells it can't record without a logs
// folder.
func TestRecordMemoryWithoutLogs(t *testing.T) {
	s := &System{newQuietApp(t, nil)}
	if err := s.RecordMemory(true); !errors.Is(err, errNoLogs) || s.RecordingMemory() {
		t.Errorf("recording without logs: %v, %v", err, s.RecordingMemory())
	}
	if _, err := s.SaveHeapProfile(); !errors.Is(err, errNoLogs) {
		t.Errorf("a profile without logs: %v", err)
	}
	if files := s.MemoryFiles(); files != nil {
		t.Errorf("the files %q", files)
	}
	if note := s.memoryNote(); note != "" {
		t.Errorf("the report's note %q", note)
	}
}
