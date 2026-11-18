package main

// Back up and restore (#592): the Backups group of Settings › Desktop
// backs MixMusic up by hand, restores a backup, undoes the last restore,
// and backs up automatically, to a folder, as often as it says; the
// command palette backs up and restores too. internal/app's backup.go and
// the backup package do the work; a restore stages its files, and
// MixMusic restarts to swap them in.

import (
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/backup"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/locales"
)

// backupView is what the backups keep between frames: the dialog of a
// backup by hand, and the parts it ticks; the backup opened for a restore,
// and what its dialog ticks and chooses; what adding its library would
// add, as counted for the folders of the key addedFor; and how many
// failures of the automatic backups the window told.
type backupView struct {
	backingUp bool
	parts     map[backup.Part]bool

	opening   bool
	restore   *app.Restore
	restoring bool
	rparts    map[backup.Part]bool
	merge     int
	folders   map[string]string
	download  bool
	staging   bool

	added    *backup.Counts
	addedFor string
	counting bool

	undoing   bool
	toldFails int
}

// restartApp restarts MixMusic, for the restore staged to swap in; tests
// replace it.
var restartApp = func(s *shell) { go s.svc.System.Restart() }

func init() { addSettings("desktop", 12, (*shell).backupSettings) }

// backupEvery are the choices of Back up automatically, in their order.
var backupEvery = []string{settings.BackupOff, settings.BackupDaily, settings.BackupWeekly, settings.BackupMonthly, settings.BackupAtQuit}

// backupEveryName names a choice of Back up automatically.
func backupEveryName(every string) string {
	switch every {
	case settings.BackupDaily:
		return locales.BackupAutoDaily()
	case settings.BackupWeekly:
		return locales.BackupAutoWeekly()
	case settings.BackupMonthly:
		return locales.BackupAutoMonthly()
	case settings.BackupAtQuit:
		return locales.BackupAutoQuit()
	}
	return locales.BackupAutoOff()
}

// backupKeepName names a choice of Keep, of settings.BackupKeeps.
func backupKeepName(keep int) string {
	switch keep {
	case 0:
		return locales.BackupKeepAll()
	case 1:
		return locales.BackupKeepOne()
	}
	return locales.BackupKeepLast(keep)
}

// backupSettings are the Backups group of Settings › Desktop.
func (s *shell) backupSettings(c *ui.Context) {
	b := &s.backups
	st := s.a.Backups().Status()
	k := &s.lk
	s.group(c, locales.BackupGroup(), func() {
		s.row(c, locales.BackupTitle(), locales.BackupAbout(), func() {
			if s.button(c, "backup-now", locales.BackupButton(), false).Disabled(st.Running).Clicked() {
				s.openBackupDialog()
			}
		}, "backup export save copy file zip another computer move")
		s.row(c, locales.BackupRestoreTitle(), locales.BackupRestoreAbout(), func() {
			label := locales.BackupRestoreButton()
			if st.Staged {
				label = locales.BackupRestarting()
			}
			if s.button(c, "backup-restore", label, false).Disabled(b.opening || st.Staged).Clicked() {
				s.chooseRestore()
			}
		}, "restore import load backup zip another computer move")
		if st.Undo != nil {
			s.row(c, locales.BackupUndoTitle(), locales.BackupUndoAbout(st.Undo.Restored), func() {
				if s.button(c, "backup-undo", locales.BackupUndoButton(), false).Clicked() {
					b.undoing = true
				}
			}, "undo restore backup")
		}
		every := backupEveryName(s.set.BackupEvery)
		names := make([]string, len(backupEvery))
		for i, e := range backupEvery {
			names[i] = backupEveryName(e)
		}
		describe := func() {
			switch {
			case st.Failed != nil:
				ui.Text(c, locales.BackupAutoFailed(errorText(st.Failed))).TextColor(k.danger)
			case s.set.BackupEvery == settings.BackupOff:
			case st.Last.IsZero():
				ui.Text(c, locales.BackupAutoNone())
			default:
				ui.Text(c, locales.BackupAutoLast(st.Last.Local()))
			}
		}
		s.rowWith(c, locales.BackupAutoTitle(), describe, func() {
			if s.selectOf(c.Key("backup-every"), &every, names).Width(240).Label(locales.BackupAutoTitle()).Changed() {
				if i := slices.Index(names, every); i >= 0 {
					s.setBackupEvery(backupEvery[i], st.Folder)
				}
			}
		}, "automatic backup schedule daily weekly monthly quit "+strings.Join(names, " "))
		if s.set.BackupEvery == settings.BackupOff {
			return
		}
		folder := st.Folder
		if folder == "" {
			folder = locales.BackupFolderNone()
		}
		s.row(c, locales.BackupFolderTitle(), folder, func() {
			if s.button(c, "backup-folder", locales.BackupFolderChange(), false).Clicked() {
				s.chooseBackupFolder(st.Folder, nil)
			}
		}, "backup folder onedrive icloud dropbox nextcloud sync")
		keep := backupKeepName(s.set.BackupKeep)
		var keeps []string
		for _, n := range settings.BackupKeeps {
			keeps = append(keeps, backupKeepName(n))
		}
		s.row(c, locales.BackupKeepTitle(), locales.BackupKeepAbout(), func() {
			if s.selectOf(c.Key("backup-keep"), &keep, keeps).Width(240).Label(locales.BackupKeepTitle()).Changed() {
				if i := slices.Index(keeps, keep); i >= 0 {
					s.set.BackupKeep = settings.BackupKeeps[i]
					s.save()
				}
			}
		}, "backup keep last delete old")
	})
}

// setBackupEvery backs up automatically as every tells: in the Flatpak,
// which has no folder at first, once the user chose one.
func (s *shell) setBackupEvery(every, folder string) {
	if every != settings.BackupOff && folder == "" {
		s.chooseBackupFolder("", func() { s.setBackupEvery(every, s.set.BackupFolder) })
		return
	}
	s.set.BackupEvery = every
	s.save()
	s.a.Backups().BackUpNow()
}

// chooseBackupFolder asks for the folder of the automatic backups, from
// start, beside the main thread, and keeps the one chosen, then runs then.
func (s *shell) chooseBackupFolder(start string, then func()) {
	win := s.win
	go func() {
		paths, err := openFileDialog(mygo.OpenDialogOptions{Parent: win, Title: locales.BackupFolderDialog(),
			Directory: true, CreateDirectories: true, DefaultPath: start})
		s.update(func() {
			switch {
			case err != nil:
				s.notes = append(s.notes, noticeOf(err))
			case len(paths) > 0:
				s.set.BackupFolder = paths[0]
				s.save()
				s.a.Backups().BackUpNow()
				if then != nil {
					then()
				}
			}
		})
	}()
}

// backupCommands are the commands of the backups, for the command
// palette: back up, restore, and undo the last restore while it can be.
func (s *shell) backupCommands() []command {
	group := locales.CmdGroupSettings()
	cmds := []command{
		{"backup.now", locales.CmdBackupNow(), "", group, s.openBackupDialog},
		{"backup.restore", locales.CmdBackupRestore(), "", group, s.chooseRestore},
	}
	if s.a.Backups().CanUndo() {
		cmds = append(cmds, command{"backup.undo", locales.BackupUndoTitle(), "", group, func() { s.backups.undoing = true }})
	}
	return cmds
}

// openBackupDialog opens the dialog of a backup by hand, every part
// ticked.
func (s *shell) openBackupDialog() {
	b := &s.backups
	b.backingUp = true
	b.parts = map[backup.Part]bool{}
	for _, p := range backup.Parts {
		b.parts[p] = true
	}
}

// partName names a part of a backup.
func partName(p backup.Part) string {
	switch p {
	case backup.Library:
		return locales.BackupPartLibrary()
	case backup.Settings:
		return locales.BackupPartSettings()
	case backup.Addons:
		return locales.BackupPartAddons()
	case backup.Queue:
		return locales.BackupPartQueue()
	}
	return locales.BackupPartDownloads()
}

// partAbout tells what a part of a backup holds, as the dialog of a
// backup by hand says it.
func partAbout(p backup.Part) string {
	switch p {
	case backup.Library:
		return locales.BackupPartLibraryAbout()
	case backup.Settings:
		return locales.BackupPartSettingsAbout()
	case backup.Addons:
		return locales.BackupPartAddonsAbout()
	case backup.Queue:
		return locales.BackupPartQueueAbout()
	}
	return locales.BackupPartDownloadsAbout()
}

// partBox is a box that ticks a part, of key, titled title, with about
// under it, and what more adds below.
func (s *shell) partBox(c *ui.Context, key, title, about string, on *bool, more func()) {
	k := &s.lk
	ui.Column(c).Gap(4).Children(func() {
		s.pointerKeys(ui.Checkbox(c.Key(key), on, title))
		ui.Column(c).Margin(0, 0, 0, 28).Gap(8).MaxWidth(500).Children(func() {
			if about != "" {
				ui.Text(c, about).FontSize(k.smallSize()).TextColor(k.muted)
			}
			if more != nil {
				more()
			}
		})
	})
}

// backupDialogs are the dialogs of the backups, and what the window tells
// of them: what the start swapped in, and each automatic backup that
// failed.
func (s *shell) backupDialogs(c *ui.Context) {
	if s.tellBackups() {
		// The toasts showed before: the next frame shows those told now.
		c.Invalidate()
	}
	s.backupDialog(c)
	s.restoreDialog(c)
	b := &s.backups
	if s.confirm(c, "backup-undo", &b.undoing, locales.BackupUndoConfirmTitle(), locales.BackupUndoConfirm(), locales.BackupUndoGo()) {
		go func() {
			err := s.a.Backups().StageUndo()
			s.update(func() {
				if err != nil {
					s.notes = append(s.notes, player.Notice{Message: locales.BackupUndoFailed(errorText(err)), Error: true})
					return
				}
				restartApp(s)
			})
		}()
	}
}

// tellBackups tells what the start swapped in, once, and each automatic
// backup that failed. It reports whether it told anything.
func (s *shell) tellBackups() bool {
	told := len(s.notes)
	bk := s.a.Backups()
	if n, err := bk.Failures(); n > s.backups.toldFails {
		s.backups.toldFails = n
		if err != nil {
			s.notes = append(s.notes, player.Notice{Message: locales.BackupAutoFailedNotice(errorText(err)), Error: true})
		}
	}
	ap := bk.TakeApplied()
	switch {
	case ap == nil:
	case ap.Err != nil && ap.Undo:
		s.notes = append(s.notes, player.Notice{Message: locales.BackupUndoFailed(errorText(ap.Err)), Error: true})
	case ap.Err != nil:
		s.notes = append(s.notes, player.Notice{Message: locales.BackupSwapFailed(errorText(ap.Err)), Error: true})
	case ap.Undone:
		s.notes = append(s.notes, notice(locales.BackupUndone()))
	case ap.Restored:
		s.notes = append(s.notes, notice(locales.BackupRestored(ap.Backup.Local())))
		if ap.Downloads != nil {
			if n := ap.Downloads.Songs(); n > 0 {
				s.notes = append(s.notes, notice(locales.BackupRestoredDownloading(n)))
			}
		}
	}
	return len(s.notes) > told
}

// backupDialog is the dialog of a backup by hand: the parts it holds,
// and what no backup holds.
func (s *shell) backupDialog(c *ui.Context) {
	b := &s.backups
	if !b.backingUp {
		return
	}
	k := &s.lk
	s.dialog(c, &b.backingUp, dialogOptions{key: "backup", title: locales.BackupDialogTitle(), width: 600}, func() {
		ui.Text(c, locales.BackupDialogIntro()).FontSize(k.bodySize()).MaxWidth(540)
		for _, p := range backup.Parts {
			on := b.parts[p]
			s.partBox(c, "backup-part-"+string(p), partName(p), partAbout(p), &on, nil)
			b.parts[p] = on
		}
		ui.Text(c, locales.BackupNever()).FontSize(k.smallSize()).TextColor(k.muted).MaxWidth(540)
		ui.Text(c, locales.BackupPrivate()).FontSize(k.smallSize()).TextColor(k.muted).MaxWidth(540)
	}, func() {
		if s.dialogButton(c, "backup-cancel", locales.ShortcutsCancel(), false).Clicked() {
			b.backingUp = false
		}
		var parts []backup.Part
		for _, p := range backup.Parts {
			if b.parts[p] {
				parts = append(parts, p)
			}
		}
		// Its name tells it from the row's button of the same words.
		save := s.dialogButton(c, "backup-save", locales.BackupButton(), true).Label(locales.BackupSaveTitle())
		if save.Disabled(len(parts) == 0).AutoFocus().Clicked() {
			b.backingUp = false
			s.saveBackup(parts)
		}
	})
}

// saveBackup asks where to save a backup of parts, beside the main
// thread, writes it there, and tells how it went.
func (s *shell) saveBackup(parts []backup.Part) {
	win := s.win
	go func() {
		path, err := saveFileDialog(mygo.SaveDialogOptions{Parent: win, Title: locales.BackupSaveTitle(), DefaultPath: backup.FileName(time.Now()),
			ButtonLabel: locales.BackupButton(), Filters: []mygo.FileFilter{{Name: locales.BackupFilter(), Extensions: []string{"zip"}}}, CreateDirectories: true})
		if err == nil && path == "" {
			return
		}
		if err == nil {
			if !strings.EqualFold(filepath.Ext(path), ".zip") {
				path += ".zip"
			}
			s.update(func() { s.notes = append(s.notes, notice(locales.BackupSaving())) })
			err = s.a.Backups().Write(path, parts)
		}
		s.update(func() {
			if err != nil {
				s.notes = append(s.notes, player.Notice{Message: locales.BackupFailed(errorText(err)), Error: true})
				return
			}
			s.actionToasts = append(s.actionToasts, actionToast{locales.BackupSaved(filepath.Base(path)), showInFolderLabel(), func() { showInFolder(path) }})
		})
	}()
}

// chooseRestore asks for a backup to restore, beside the main thread,
// checks it, and opens its dialog, or tells why it can't be restored.
func (s *shell) chooseRestore() {
	b := &s.backups
	if b.opening || b.restore != nil || s.a.Backups().Status().Staged {
		return
	}
	win := s.win
	go func() {
		paths, err := openFileDialog(mygo.OpenDialogOptions{Parent: win, Title: locales.BackupOpenTitle(),
			Filters: []mygo.FileFilter{{Name: locales.BackupFilter(), Extensions: []string{"zip"}}}})
		if err != nil || len(paths) == 0 {
			if err != nil {
				s.update(func() { s.notes = append(s.notes, noticeOf(err)) })
			}
			return
		}
		path := paths[0]
		s.update(func() {
			b.opening = true
			s.notes = append(s.notes, notice(locales.BackupOpening()))
		})
		r, err := s.a.Backups().Open(path)
		s.update(func() {
			b.opening = false
			if err != nil {
				s.notes = append(s.notes, player.Notice{Message: locales.BackupRefused(filepath.Base(path), errorText(err)), Error: true})
				return
			}
			s.showRestore(r)
		})
	}()
}

// showRestore opens the dialog of the restore of r: every part it holds
// ticked, the library replaced, the folders of music matched as their
// names suggest.
func (s *shell) showRestore(r *app.Restore) {
	b := &s.backups
	b.restore, b.restoring, b.merge = r, true, 0
	b.rparts = map[backup.Part]bool{}
	for _, p := range r.Manifest.Parts {
		b.rparts[p] = true
	}
	b.download = r.Counts.Downloads > 0
	b.folders = maps.Clone(r.Folders)
	b.added, b.addedFor, b.counting = nil, "", false
}

// closeRestore closes the dialog of a restore, and the backup it opened.
func (s *shell) closeRestore() {
	b := &s.backups
	r := b.restore
	b.restore, b.restoring = nil, false
	go r.Close()
}

// systemName names the system of a backup, as Go names it.
func systemName(system string) string {
	switch system {
	case "windows":
		return "Windows"
	case "darwin":
		return "macOS"
	case "linux":
		return "Linux"
	}
	return system
}

// restoreDialog is the dialog of a restore: where the backup comes from,
// what it holds, by part, to tick; the library replaced or added to; the
// folders of music matched; and that what's ticked replaces what's here.
func (s *shell) restoreDialog(c *ui.Context) {
	b := &s.backups
	r := b.restore
	if r == nil {
		return
	}
	k := &s.lk
	m, n := r.Manifest, r.Counts
	ticked := func(p backup.Part) bool { return b.rparts[p] && m.Has(p) }
	if b.merge == 1 && ticked(backup.Library) {
		s.countAdded(r)
	}
	s.dialog(c, &b.restoring, dialogOptions{key: "restore", title: locales.BackupRestoreDialogTitle(), width: 640}, func() {
		ui.Text(c, locales.BackupRestoreFrom(m.Created.Local(), m.App, systemName(m.System))).FontSize(k.bodySize()).MaxWidth(580)
		for _, p := range backup.Parts {
			if !m.Has(p) {
				continue
			}
			on := b.rparts[p]
			switch p {
			case backup.Library:
				s.partBox(c, "restore-part-library", partName(p),
					locales.BackupRestoreLibraryHolds(n.Liked, n.Disliked, n.Mixes, n.Plays, n.Artists, n.Podcasts), &on, func() {
						if !on {
							return
						}
						s.segmented(c, "restore-merge", locales.BackupPartLibrary(), &b.merge, locales.BackupRestoreReplace(), locales.BackupRestoreMerge())
						if b.merge == 1 {
							text := locales.BackupRestoreCounting()
							if a := b.added; a != nil && !b.counting {
								text = locales.BackupRestoreAdds(a.Liked, a.Disliked, a.Mixes, a.Plays, a.Artists, a.Podcasts)
							}
							ui.Text(c, text).FontSize(k.smallSize()).TextColor(k.muted)
						}
					})
			case backup.Settings:
				s.partBox(c, "restore-part-settings", partName(p), locales.BackupRestoreSettingsAbout(), &on, nil)
			case backup.Addons:
				s.partBox(c, "restore-part-addons", partName(p), locales.BackupRestoreAddons(n.Addons), &on, nil)
			case backup.Queue:
				s.partBox(c, "restore-part-queue", partName(p), locales.BackupRestoreQueue(n.Queue), &on, nil)
			case backup.Downloads:
				if n.Downloads == 0 {
					continue
				}
				on = b.download
				s.partBox(c, "restore-part-downloads", locales.BackupRestoreDownload(n.Downloads), locales.BackupRestoreDownloadAbout(), &on, nil)
				b.download = on
				continue
			}
			b.rparts[p] = on
		}
		if len(r.Moved) > 0 && len(r.Here) > 0 && (ticked(backup.Library) || ticked(backup.Queue)) {
			s.restoreFolders(c, r)
		}
		ui.Text(c, locales.BackupRestoreWarning()).FontSize(k.smallSize()).TextColor(k.muted).MaxWidth(580)
	}, func() {
		if s.dialogButton(c, "restore-cancel", locales.ShortcutsCancel(), false).Clicked() {
			b.restoring = false
		}
		some := b.download && n.Downloads > 0
		for _, p := range backup.Parts {
			some = some || p != backup.Downloads && ticked(p)
		}
		label := locales.BackupRestoreGo()
		if b.staging {
			label = locales.BackupRestarting()
		}
		if s.dialogButton(c, "restore-go", label, true).Disabled(!some || b.staging || b.counting).AutoFocus().Clicked() {
			s.stageRestore()
		}
	})
	if !b.restoring && !b.staging {
		s.closeRestore()
	}
}

// restoreFolders matches each folder of music the backup came from that
// isn't here to one here, or none.
func (s *shell) restoreFolders(c *ui.Context, r *app.Restore) {
	b := &s.backups
	k := &s.lk
	ui.Column(c).Gap(8).Children(func() {
		ui.Text(c, locales.BackupRestoreFolders()).FontSize(k.bodySize()).FontWeight(600)
		ui.Text(c, locales.BackupRestoreFoldersAbout()).FontSize(k.smallSize()).TextColor(k.muted).MaxWidth(580)
		none := locales.BackupRestoreFolderNone()
		options := append([]string{none}, r.Here...)
		for i, from := range r.Moved {
			chosen := b.folders[from]
			if chosen == "" {
				chosen = none
			}
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Text(c, from).FontSize(k.smallSize()).Grow(1).MinWidth(0).MaxLines(2)
				if s.selectOf(c.Key("restore-folder-"+count(i)), &chosen, options).Width(260).Label(locales.BackupRestoreFolderFor(from)).Changed() {
					if chosen == none {
						chosen = ""
					}
					b.folders[from] = chosen
				}
			})
		}
	})
}

// foldersKey is a key of folders, which tells whether what adding the
// library adds was counted for them.
func foldersKey(folders map[string]string) string {
	var sb strings.Builder
	for _, from := range slices.Sorted(maps.Keys(folders)) {
		fmt.Fprintf(&sb, "%q=%q;", from, folders[from])
	}
	return sb.String()
}

// countAdded counts what adding the backup's library adds, beside the main
// thread, once for each match of the folders.
func (s *shell) countAdded(r *app.Restore) {
	b := &s.backups
	key := foldersKey(b.folders)
	if b.counting || b.added != nil && b.addedFor == key {
		return
	}
	b.counting = true
	folders := maps.Clone(b.folders)
	go func() {
		added, err := r.Added(folders)
		s.update(func() {
			b.counting = false
			if b.restore != r {
				return
			}
			b.addedFor = key
			if err != nil {
				added = backup.Counts{}
			}
			b.added = &added
		})
	}()
}

// stageRestore stages the restore of what the dialog ticked, beside the
// main thread, then restarts MixMusic to swap it in.
func (s *shell) stageRestore() {
	b := &s.backups
	r := b.restore
	choice := backup.Choice{Merge: b.merge == 1, Folders: maps.Clone(b.folders), Download: b.download}
	for _, p := range backup.Parts {
		if b.rparts[p] && r.Manifest.Has(p) {
			choice.Parts = append(choice.Parts, p)
		}
	}
	b.staging = true
	go func() {
		err := s.a.Backups().Stage(r, choice)
		s.update(func() {
			b.staging, b.restore, b.restoring = false, nil, false
			if err != nil {
				s.notes = append(s.notes, player.Notice{Message: locales.BackupRestoreFailed(errorText(err)), Error: true})
				return
			}
			restartApp(s)
		})
	}()
}
