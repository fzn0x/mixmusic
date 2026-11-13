package main

// Settings › About › Memory use (#384): Record memory use, which is off at
// each start, and while it's on, Save a heap profile. internal/app's
// memory.go keeps them in the logs folder.

import (
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/locales"
)

func init() { addSettings("about", 15, (*shell).memorySettings) }

// memorySettings record how MixMusic uses its memory, for a report of a
// problem: Settings › About › Memory use.
func (s *shell) memorySettings(c *ui.Context) {
	sys := s.svc.System
	s.group(c, locales.MemoryUse(), func() {
		on := sys.RecordingMemory()
		s.row(c, locales.MemoryRecordTitle(), locales.MemoryRecordAbout(), func() {
			if s.switchOf(c.Key("record-memory"), &on).Label(locales.MemoryRecordTitle()).Changed() {
				if err := sys.RecordMemory(on); err != nil {
					s.notes = append(s.notes, noticeOf(err))
				}
			}
		}, "memory ram heap profile leak usage report problem")
		if !sys.RecordingMemory() {
			return
		}
		s.row(c, locales.MemoryHeapTitle(), locales.MemoryHeapAbout(), func() {
			if s.button(c, "heap-profile", locales.MemorySaveHeap(), false).Clicked() {
				s.saveHeapProfile()
			}
			if s.button(c, "memory-logs", locales.MemoryOpenLogs(), false).Clicked() {
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
			s.notes = append(s.notes, notice(locales.MemorySaved(name)))
		})
	}()
}
