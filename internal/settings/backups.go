package settings

import (
	"path/filepath"
	"slices"
)

// How often MixMusic backs itself up (#592), as BackupEvery tells.
const (
	// BackupOff makes no backup but those the user makes, as at first.
	BackupOff = ""
	// BackupDaily, BackupWeekly and BackupMonthly back up once a day, a
	// week or a month: one missed while MixMusic was closed is made as it
	// starts.
	BackupDaily   = "daily"
	BackupWeekly  = "weekly"
	BackupMonthly = "monthly"
	// BackupAtQuit backs up each time MixMusic quits.
	BackupAtQuit = "quit"
)

// BackupKeeps are how many automatic backups the folder may keep, as
// BackupKeep tells: 1 is one file written over, and 0 all of them.
var BackupKeeps = []int{3, 5, 10, 15, 30, 0, 1}

// cleanBackups returns how often to back up, as every tells, off for
// anything else; folder, cleaned, or "" for one that isn't absolute; and
// keep, or 5 for a number BackupKeeps doesn't hold.
func cleanBackups(every, folder string, keep int) (string, string, int) {
	switch every {
	case BackupDaily, BackupWeekly, BackupMonthly, BackupAtQuit:
	default:
		every = BackupOff
	}
	if folder = CleanPath(folder); folder != "" {
		if filepath.IsAbs(folder) {
			folder = filepath.Clean(folder)
		} else {
			folder = ""
		}
	}
	if !slices.Contains(BackupKeeps, keep) {
		keep = 5
	}
	return every, folder, keep
}
