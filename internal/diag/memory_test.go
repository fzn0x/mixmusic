package diag

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestReadMemory reads the Go heap, and the process's memory where the
// system tells it: Linux its resident size, Windows that and its private
// bytes, macOS that and its footprint.
func TestReadMemory(t *testing.T) {
	m := ReadMemory()
	if m.HeapInUse == 0 || m.Sys < m.HeapInUse {
		t.Errorf("the Go heap: %+v", m)
	}
	switch runtime.GOOS {
	case "linux":
		if m.Resident == 0 {
			t.Errorf("no resident size: %+v", m)
		}
	case "windows":
		if m.Resident == 0 || m.Private == 0 {
			t.Errorf("no working set or private bytes: %+v", m)
		}
	case "darwin":
		if m.Resident == 0 || m.Footprint == 0 {
			t.Errorf("no resident size or footprint: %+v", m)
		}
	}
	if !strings.HasPrefix(m.String(), "Go heap ") || m.Process() < m.Sys {
		t.Errorf("%q, of %d bytes", m, m.Process())
	}
}

func TestMemoryString(t *testing.T) {
	m := Memory{HeapInUse: 120 << 20, Sys: 300 << 20, GCs: 45}
	if got := m.String(); got != "Go heap 120 MB in use, 300 MB from the system, 45 GCs" {
		t.Errorf("of Go alone: %q", got)
	}
	m.Resident, m.Footprint = 410<<20, 380<<20
	if got := m.String(); got != "Go heap 120 MB in use, 300 MB from the system, 45 GCs; resident 410 MB, footprint 380 MB" {
		t.Errorf("with the process's: %q", got)
	}
	if m.Process() != 410<<20 {
		t.Errorf("the process holds %d", m.Process())
	}
}

// fakeRecording is a recorder whose clock, memory and ticks the test
// gives.
type fakeRecording struct {
	*MemoryRecorder
	mu      sync.Mutex
	at      time.Time
	mem     Memory
	ticks   chan time.Time
	stopped int
}

func newFakeRecording(t *testing.T) *fakeRecording {
	f := &fakeRecording{
		MemoryRecorder: NewMemoryRecorder(t.TempDir()),
		at:             time.Date(2026, 11, 8, 15, 30, 0, 0, time.UTC),
		mem:            Memory{HeapInUse: 100 << 20, Sys: 200 << 20, GCs: 3, Resident: 300 << 20},
		ticks:          make(chan time.Time),
	}
	f.now = func() time.Time {
		f.mu.Lock()
		defer f.mu.Unlock()
		return f.at
	}
	f.read = func() Memory {
		f.mu.Lock()
		defer f.mu.Unlock()
		return f.mem
	}
	f.tick = func(d time.Duration) (<-chan time.Time, func()) {
		if d != MemoryEvery {
			t.Errorf("ticks every %v", d)
		}
		return f.ticks, func() {
			f.mu.Lock()
			f.stopped++
			f.mu.Unlock()
		}
	}
	t.Cleanup(f.Stop)
	return f
}

// minute moves the clock on a minute, with mem as the memory, and ticks.
func (f *fakeRecording) minute(mem Memory) {
	f.mu.Lock()
	f.at = f.at.Add(time.Minute)
	f.mem = mem
	at := f.at
	f.mu.Unlock()
	f.ticks <- at
}

// lines waits until memory.log holds n lines, and returns them.
func (f *fakeRecording) lines(t *testing.T, n int) []string {
	t.Helper()
	var lines []string
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(2 * time.Millisecond) {
		b, _ := os.ReadFile(filepath.Join(f.dir, memoryName))
		lines = strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
		if len(b) > 0 && len(lines) >= n {
			return lines
		}
		if time.Now().After(deadline) {
			t.Fatalf("memory.log holds %d lines, not %d: %q", len(lines), n, lines)
		}
	}
}

// TestMemoryRecorder records a line at once and at each minute, saves a
// profile once the memory passed the threshold, and once only, and one
// when asked; it stops, and starts again.
func TestMemoryRecorder(t *testing.T) {
	f := newFakeRecording(t)
	if f.On() {
		t.Fatal("on before it started")
	}
	if err := f.Start("Recording, MixMusic 9.9.9 on test"); err != nil {
		t.Fatal(err)
	}
	lines := f.lines(t, 2)
	want := []string{
		"2026-11-08T15:30:00Z Recording, MixMusic 9.9.9 on test",
		"2026-11-08T15:30:00Z Go heap 100 MB in use, 200 MB from the system, 3 GCs; resident 300 MB",
	}
	if !f.On() || !slices.Equal(lines, want) {
		t.Fatalf("memory.log: %q", lines)
	}
	f.minute(Memory{HeapInUse: 150 << 20, Sys: 250 << 20, GCs: 4, Resident: 400 << 20})
	if got := f.lines(t, 3)[2]; got != "2026-11-08T15:31:00Z Go heap 150 MB in use, 250 MB from the system, 4 GCs; resident 400 MB" {
		t.Errorf("a minute after: %q", got)
	}
	if files := MemoryFiles(f.dir); !slices.Equal(files, []string{memoryName}) {
		t.Errorf("before the threshold, the files %q", files)
	}

	// Past the threshold, once.
	over := Memory{HeapInUse: 900 << 20, Sys: 1000 << 20, GCs: 9, Resident: 1100 << 20}
	f.minute(over)
	lines = f.lines(t, 5)
	if lines[4] != "2026-11-08T15:32:00Z Passed 1024 MB: saved heap-20261108-153200.pprof" {
		t.Errorf("past the threshold: %q", lines[3:])
	}
	f.minute(over)
	f.lines(t, 6)
	if files := MemoryFiles(f.dir); !slices.Equal(files, []string{memoryName, "heap-20261108-153200.pprof"}) {
		t.Errorf("the files %q", files)
	}
	if info, err := os.Stat(filepath.Join(f.dir, "heap-20261108-153200.pprof")); err != nil || info.Size() == 0 {
		t.Errorf("the profile: %v", err)
	}

	// Asked.
	name, err := f.SaveProfile()
	if err != nil || name != "heap-20261108-153300.pprof" {
		t.Fatalf("saved %q, %v", name, err)
	}
	if got := f.lines(t, 7)[6]; got != "2026-11-08T15:33:00Z Saved heap-20261108-153300.pprof" {
		t.Errorf("saving: %q", got)
	}

	f.Stop()
	lines = f.lines(t, 8)
	if f.On() || lines[7] != "2026-11-08T15:33:00Z Stopped" || f.stopped != 1 {
		t.Errorf("stopped: on %v, %q, ticks stopped %d times", f.On(), lines[7:], f.stopped)
	}
	f.Stop() // stopped already

	// Again, it passes the threshold again.
	f.mu.Lock()
	f.at, f.mem = f.at.Add(time.Hour), over
	f.mu.Unlock()
	if err := f.Start("Recording"); err != nil {
		t.Fatal(err)
	}
	if lines := f.lines(t, 11); lines[10] != "2026-11-08T16:33:00Z Passed 1024 MB: saved heap-20261108-163300.pprof" {
		t.Errorf("started again: %q", lines[8:])
	}
}

// TestMemoryRecorderKeeps keeps the newest five profiles, and starts a
// memory.log of a megabyte anew.
func TestMemoryRecorderKeeps(t *testing.T) {
	f := newFakeRecording(t)
	for range 7 {
		f.at = f.at.Add(time.Second)
		if _, err := f.SaveProfile(); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"heap-20261108-153003.pprof", "heap-20261108-153004.pprof", "heap-20261108-153005.pprof",
		"heap-20261108-153006.pprof", "heap-20261108-153007.pprof"}
	if got := MemoryFiles(f.dir); !slices.Equal(got, want) {
		t.Errorf("kept %q", got)
	}
	big := strings.Repeat("an old line of the memory used\n", maxMemorySize/30)
	if err := os.WriteFile(filepath.Join(f.dir, memoryName), []byte(big), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := f.Start("Recording"); err != nil {
		t.Fatal(err)
	}
	if lines := f.lines(t, 2); len(lines) != 2 || lines[0] != "2026-11-08T15:30:07Z Recording" {
		t.Errorf("a memory.log of a megabyte: %d lines, from %q", len(lines), lines[0])
	}
}
