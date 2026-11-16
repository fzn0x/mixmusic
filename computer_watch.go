package main

import (
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/locales"
)

// unwatchedFolders tells, in Settings › Playback › Music on this computer,
// of each folder of music MixMusic can't watch for changes, as one on a
// network drive, and why: what changes in it shows after a rescan, or once
// MixMusic starts again. The others are watched, and show what changes in
// them within a few seconds.
func (s *shell) unwatchedFolders(c *ui.Context) {
	for _, folder := range s.set.MusicFolders {
		if why := s.svc.LocalFiles.Unwatched(folder); why != "" {
			s.hint(c, unwatchedLine(folder, why))
		}
	}
}

// unwatchedLine tells that MixMusic can't watch folder, and why.
func unwatchedLine(folder, why string) string {
	return locales.ComputerUnwatched(folder, why)
}
