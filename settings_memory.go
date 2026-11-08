package main

// Settings › About › Memory use (#384): Record memory use, which is off at
// each start, and while it's on, Save a heap profile. internal/app's
// memory.go keeps them in the logs folder.

import "github.com/egoist/mygo/ui"

func init() { addSettings("about", 15, (*shell).memorySettings) }

const (
	// recordMemoryTitle and recordMemoryAbout are the row of Record memory
	// use.
	recordMemoryTitle = "Record memory use"
	recordMemoryAbout = "Writes how MixMusic uses its memory to memory.log in the logs folder, once a minute, " +
		"and saves a heap profile there once it passes 1 GB, for a report of a problem. " +
		"They hold only how MixMusic uses its memory, nothing of your music or accounts. It's off again at the next start."
	// heapProfileAbout tells what Save a heap profile saves.
	heapProfileAbout = "Saves where MixMusic's Go code holds memory now, as heap-<time>.pprof beside memory.log. The newest five are kept."
)

// memorySettings record how MixMusic uses its memory, for a report of a
// problem: Settings › About › Memory use.
func (s *shell) memorySettings(c *ui.Context) {
	sys := s.svc.System
	s.group(c, "Memory use", func() {
		on := sys.RecordingMemory()
		s.row(c, recordMemoryTitle, recordMemoryAbout, func() {
			if s.switchOf(c.Key("record-memory"), &on).Label(recordMemoryTitle).Changed() {
				if err := sys.RecordMemory(on); err != nil {
					s.notes = append(s.notes, noticeOf(err))
				}
			}
		}, "memory ram heap profile leak usage report problem")
		if !sys.RecordingMemory() {
			return
		}
		s.row(c, "Heap profile", heapProfileAbout, func() {
			if s.button(c, "heap-profile", "save a heap profile", false).Clicked() {
				s.saveHeapProfile()
			}
			if s.button(c, "memory-logs", "open logs folder", false).Clicked() {
				if err := sys.OpenLogsFolder(); err != nil {
					s.notes = append(s.notes, noticeOf(err))
				}
			}
		}, "memory heap profile pprof")
	})
}

// saveHeapProfile saves a profile of the Go heap in the logs folder,
// beside the main thread, and tells its name in a toast.
func (s *shell) saveHeapProfile() {
	go func() {
		name, err := s.svc.System.SaveHeapProfile()
		s.update(func() {
			if err != nil {
				s.notes = append(s.notes, noticeOf(err))
				return
			}
			s.notes = append(s.notes, notice("Saved "+name+" in the logs folder"))
		})
	}()
}
