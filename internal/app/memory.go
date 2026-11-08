package app

// Recording memory use (#384): Settings › About › Record memory use has
// MixMusic write how it uses its memory to memory.log in the logs folder
// each minute, and save heap profiles there (internal/diag), until it's
// turned off or MixMusic quits. It is off at each start: nothing keeps it.

import (
	"errors"
	"fmt"
	"log"
	"runtime"
	"strings"

	"github.com/fzn0x/mixmusic/internal/diag"
)

// errNoLogs is returned where MixMusic keeps no log.
var errNoLogs = errors.New("MixMusic keeps no log here")

// memoryRecorder returns the recorder of the logs folder, made at its
// first use, or nil where MixMusic keeps no log.
func (a *App) memoryRecorder() *diag.MemoryRecorder {
	if a.opts.LogsDir == "" {
		return nil
	}
	a.memoryOnce.Do(func() { a.memory = diag.NewMemoryRecorder(a.opts.LogsDir) })
	return a.memory
}

// RecordMemory turns recording MixMusic's memory use on or off: while on,
// memory.log in the logs folder gets a line of it each minute, and a heap
// profile once it passes diag.MemoryThreshold.
func (s *System) RecordMemory(on bool) error {
	r := s.a.memoryRecorder()
	if r == nil {
		return errNoLogs
	}
	if !on {
		if r.On() {
			r.Stop()
			log.Print("mixmusic: stopped recording memory use")
		}
		return nil
	}
	if r.On() {
		return nil
	}
	head := fmt.Sprintf("Recording, MixMusic %s on %s/%s", s.a.version(), runtime.GOOS, runtime.GOARCH)
	if err := r.Start(head); err != nil {
		return fmt.Errorf("can't record memory use: %w", err)
	}
	log.Print("mixmusic: recording memory use in memory.log")
	return nil
}

// RecordingMemory reports whether MixMusic records its memory use.
func (s *System) RecordingMemory() bool {
	r := s.a.memoryRecorder()
	return r != nil && r.On()
}

// SaveHeapProfile saves a profile of the Go heap in the logs folder, and
// returns its name.
func (s *System) SaveHeapProfile() (string, error) {
	r := s.a.memoryRecorder()
	if r == nil {
		return "", errNoLogs
	}
	name, err := r.SaveProfile()
	if err != nil {
		return "", fmt.Errorf("can't save a heap profile: %w", err)
	}
	log.Printf("mixmusic: saved a heap profile, %s", name)
	return name, nil
}

// MemoryFiles returns the names of the files recording memory use left in
// the logs folder: memory.log, and the heap profiles, oldest first.
func (s *System) MemoryFiles() []string {
	if s.a.opts.LogsDir == "" {
		return nil
	}
	return diag.MemoryFiles(s.a.opts.LogsDir)
}

// memoryNote tells a report of a problem of the files recording memory
// use left in the logs folder: "" when there are none.
func (s *System) memoryNote() string {
	files := s.MemoryFiles()
	if len(files) == 0 {
		return ""
	}
	return "Recording memory use left " + strings.Join(files, ", ") +
		" in the same folder: they hold only how MixMusic uses its memory, nothing of your music or accounts. Attach them too.\n"
}

// stopMemory stops recording memory use, as the app closes.
func (a *App) stopMemory() {
	if r := a.memoryRecorder(); r != nil {
		r.Stop()
	}
}
