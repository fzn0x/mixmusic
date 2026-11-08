package diag

// Recording memory use (#384): while the user has it on, memory.log in the
// logs folder gets a line of how MixMusic uses its memory each minute, and
// a heap profile, heap-<time>.pprof, is saved there when the user asks,
// and once as the memory passes a threshold. Capture mode's act:memory and
// act:heap read and save them the same way.

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	// memoryName is the file of the lines of memory use.
	memoryName = "memory.log"
	// maxMemorySize is the size at which memory.log starts anew, as a
	// recording starts: some two weeks of lines.
	maxMemorySize = 1 << 20
	// keepProfiles is how many heap profiles are kept.
	keepProfiles = 5
	// MemoryEvery is how often a recording writes a line.
	MemoryEvery = time.Minute
	// MemoryThreshold is the memory past which a recording saves a heap
	// profile of itself, once: of the process, as the system tells it, or
	// else of what Go holds.
	MemoryThreshold = 1 << 30
)

// Memory is how MixMusic uses its memory at a moment: the Go heap's, as
// the runtime tells it, and the process's, where the system tells it
// cheaply.
type Memory struct {
	// HeapInUse is the bytes of the Go heap's spans in use, and Sys those
	// Go holds from the system, the heap's and the rest.
	HeapInUse, Sys uint64
	// GCs counts the collections of the Go heap so far.
	GCs uint32
	// Resident is the process's resident size, the bytes of its pages in
	// memory, and 0 where the system doesn't tell it cheaply.
	Resident uint64
	// Footprint is macOS's physical footprint of the process, which
	// Activity Monitor shows as its Memory; Private is Windows' private
	// bytes of it, its memory no other process shares. Each is 0 on the
	// other systems.
	Footprint, Private uint64
}

// ReadMemory reads how MixMusic uses its memory now.
func ReadMemory() Memory {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	m := Memory{HeapInUse: ms.HeapInuse, Sys: ms.Sys, GCs: ms.NumGC}
	m.Resident, m.Footprint, m.Private = processMemory()
	return m
}

// String writes m as a line of a log: "Go heap 120 MB in use, 300 MB from
// the system, 45 GCs; resident 410 MB", and the footprint or the private
// bytes where the system tells them.
func (m Memory) String() string {
	s := fmt.Sprintf("Go heap %d MB in use, %d MB from the system, %d GCs", m.HeapInUse>>20, m.Sys>>20, m.GCs)
	var proc []string
	for _, v := range []struct {
		name string
		n    uint64
	}{{"resident", m.Resident}, {"footprint", m.Footprint}, {"private", m.Private}} {
		if v.n > 0 {
			proc = append(proc, fmt.Sprintf("%s %d MB", v.name, v.n>>20))
		}
	}
	if len(proc) > 0 {
		s += "; " + strings.Join(proc, ", ")
	}
	return s
}

// Process is the most the process holds, of what is known: its footprint,
// private bytes or resident size, or what Go holds from the system.
func (m Memory) Process() uint64 { return max(m.Footprint, m.Private, m.Resident, m.Sys) }

// WriteHeapProfile collects the Go heap's garbage, so that the profile
// tells what is in use, and writes a profile of the heap to path: where
// it was allocated, by how much, and nothing of what it holds.
func WriteHeapProfile(path string) error {
	runtime.GC()
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := pprof.WriteHeapProfile(f); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// MemoryFiles returns the names of the files recordings of memory use
// left in dir: memory.log, and the heap profiles, oldest first.
func MemoryFiles(dir string) []string {
	var files []string
	if info, err := os.Stat(filepath.Join(dir, memoryName)); err == nil && !info.IsDir() {
		files = append(files, memoryName)
	}
	for _, p := range profiles(dir) {
		files = append(files, filepath.Base(p))
	}
	return files
}

// profiles returns the heap profiles in dir, oldest first: their names
// sort by time.
func profiles(dir string) []string {
	found, _ := filepath.Glob(filepath.Join(dir, "heap-*.pprof"))
	slices.Sort(found)
	return found
}

// MemoryRecorder records how MixMusic uses its memory, while it's on, in
// a logs folder. Its methods are safe for concurrent use.
type MemoryRecorder struct {
	dir string
	// now and read tell the time and the memory, tick makes the ticks of
	// a recording, every d, until stop; tests replace them.
	now  func() time.Time
	read func() Memory
	tick func(d time.Duration) (c <-chan time.Time, stop func())

	mu sync.Mutex
	// on is set while it records, and stop ends the recording's
	// goroutine, which closes done.
	on   bool
	stop chan struct{}
	done chan struct{}
	// passed is set once the recording saved the profile of passing
	// MemoryThreshold.
	passed bool
}

// NewMemoryRecorder returns a recorder of the logs folder dir, which
// records nothing until Start.
func NewMemoryRecorder(dir string) *MemoryRecorder {
	return &MemoryRecorder{dir: dir, now: time.Now, read: ReadMemory, tick: func(d time.Duration) (<-chan time.Time, func()) {
		t := time.NewTicker(d)
		return t.C, t.Stop
	}}
}

// On reports whether it records.
func (r *MemoryRecorder) On() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.on
}

// Start starts recording, with a line of head, then a line of the memory
// at once and every MemoryEvery after, until Stop. A memory.log of a
// megabyte or more starts anew.
func (r *MemoryRecorder) Start(head string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.on {
		return nil
	}
	if err := os.MkdirAll(r.dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(r.dir, memoryName)
	if info, err := os.Stat(path); err == nil && info.Size() >= maxMemorySize {
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	if err := r.write(head); err != nil {
		return err
	}
	r.on, r.passed = true, false
	r.stop, r.done = make(chan struct{}), make(chan struct{})
	c, stopTicks := r.tick(MemoryEvery)
	stop, done := r.stop, r.done
	go func() {
		defer close(done)
		defer stopTicks()
		r.record()
		for {
			select {
			case <-stop:
				return
			case <-c:
				r.record()
			}
		}
	}()
	return nil
}

// Stop stops recording, and waits for the line being written.
func (r *MemoryRecorder) Stop() {
	r.mu.Lock()
	if !r.on {
		r.mu.Unlock()
		return
	}
	r.on = false
	close(r.stop)
	done := r.done
	r.mu.Unlock()
	<-done
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.write("Stopped"); err != nil {
		log.Printf("mixmusic: recording memory use: %v", err)
	}
}

// record writes a line of the memory now, and saves a profile once it
// passed MemoryThreshold, once in a recording.
func (r *MemoryRecorder) record() {
	m := r.read()
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.on {
		return
	}
	if err := r.write(m.String()); err != nil {
		log.Printf("mixmusic: recording memory use: %v", err)
		return
	}
	if r.passed || m.Process() < MemoryThreshold {
		return
	}
	r.passed = true
	name, err := r.saveProfile()
	if err != nil {
		log.Printf("mixmusic: saving a heap profile: %v", err)
		return
	}
	if err := r.write(fmt.Sprintf("Passed %d MB: saved %s", MemoryThreshold>>20, name)); err != nil {
		log.Printf("mixmusic: recording memory use: %v", err)
	}
}

// write adds a line of text to memory.log, after the time. Its caller
// holds mu.
func (r *MemoryRecorder) write(text string) error {
	f, err := os.OpenFile(filepath.Join(r.dir, memoryName), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f, "%s %s\n", r.now().Format(time.RFC3339), text); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// SaveProfile saves a profile of the Go heap in the logs folder, and
// returns its name, as heap-20261108-153000.pprof. The newest five are
// kept.
func (r *MemoryRecorder) SaveProfile() (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	name, err := r.saveProfile()
	if err == nil && r.on {
		if err := r.write("Saved " + name); err != nil {
			log.Printf("mixmusic: recording memory use: %v", err)
		}
	}
	return name, err
}

// saveProfile is SaveProfile; its caller holds mu.
func (r *MemoryRecorder) saveProfile() (string, error) {
	if err := os.MkdirAll(r.dir, 0o755); err != nil {
		return "", err
	}
	name := "heap-" + r.now().Format("20060102-150405") + ".pprof"
	if err := WriteHeapProfile(filepath.Join(r.dir, name)); err != nil {
		return "", err
	}
	kept := profiles(r.dir)
	for len(kept) > keepProfiles {
		os.Remove(kept[0])
		kept = kept[1:]
	}
	return name, nil
}
