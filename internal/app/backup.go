package app

// Backups (#592): Settings › Desktop › Backups backs MixMusic up by hand,
// and restores a backup, and the automatic backups go to a folder as often
// as the settings say. The backup package writes, checks and stages them;
// main swaps a restore staged in as MixMusic starts, and tells the app
// what it did, which the main window tells.

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/egoist/mygo"

	"github.com/fzn0x/mixmusic/internal/backup"
	"github.com/fzn0x/mixmusic/internal/downloads"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/store"
	"github.com/fzn0x/mixmusic/locales"
)

// backupsName is the file of the data folder that keeps what became of
// the automatic backups. No backup holds it.
const backupsName = "backups.json"

// backupsKept is what backupsName keeps: when the last automatic backup
// was made, to which file, and the fingerprint of what it holds; and when
// the settings last asked for one, and nothing had changed.
type backupsKept struct {
	Last    time.Time `json:"last,omitzero"`
	File    string    `json:"file,omitempty"`
	Sum     string    `json:"sum,omitempty"`
	Checked time.Time `json:"checked,omitzero"`
}

// backupState is the app's of the backups: the file of backupsName, once
// read; whether a backup is being written; why the last automatic one
// failed in this run, and a count of the failures, for the windows to tell
// each once; what the start swapped in, until the windows tell it; the
// last restore, while it can be undone, and whether a restore or its undo
// waits for the restart; and whether the start's and the quit's backups
// ran.
type backupState struct {
	mu      sync.Mutex
	file    *store.File[backupsKept]
	running bool
	failed  error
	fails   int
	applied *backup.Applied
	undo    *backup.Undo
	staged  bool
	started bool
	quit    bool
	// folder is the default folder of the automatic backups, once found.
	folder     string
	folderOnce sync.Once
}

// backupStartWait is how long after the start the automatic backup missed
// while MixMusic was closed is made, and backupEvery how often the app
// looks whether one is due.
const (
	backupStartWait = 30 * time.Second
	backupEvery     = time.Hour
)

// Backups backs MixMusic up, and restores its backups.
type Backups struct{ a *App }

// Backups returns the backups of the app.
func (a *App) Backups() *Backups { return &Backups{a} }

// keptBackups returns the file of backupsName, opening it at its first
// use. a.backups.mu is held.
func (a *App) keptBackups() *store.File[backupsKept] {
	bs := &a.backups
	if bs.file == nil {
		f, err := store.Open(filepath.Join(a.opts.DataDir, backupsName), backupsKept{})
		if err != nil {
			log.Printf("mixmusic: reading %s: %v", backupsName, err)
			f, _ = store.Open(filepath.Join(a.opts.DataDir, backupsName+".new"), backupsKept{})
		}
		bs.file = f
	}
	return bs.file
}

// BackupStatus is what the automatic backups and the restores are now.
type BackupStatus struct {
	// Every is how often MixMusic backs up, as settings' BackupEvery, and
	// Folder the folder the backups go to: "" in the Flatpak until the user
	// chooses one.
	Every, Folder string
	// Last is when the last automatic backup was made, if one was.
	Last time.Time
	// Running is set while a backup is being written.
	Running bool
	// Failed is why the last automatic backup of this run failed, until
	// one is made, and Fails counts the failures, for each to be told
	// once: the next start tries again.
	Failed error
	Fails  int
	// Undo is the last restore, while it can be undone.
	Undo *backup.Undo
	// Staged is set while a restore or its undo waits for MixMusic to
	// restart.
	Staged bool
}

// Status tells what the backups are now.
func (b *Backups) Status() BackupStatus {
	a := b.a
	s := a.settings.Get().Clean()
	bs := &a.backups
	bs.mu.Lock()
	kept := a.keptBackups().Get()
	st := BackupStatus{Every: s.BackupEvery, Folder: a.backupFolder(s), Last: kept.Last, Running: bs.running,
		Failed: bs.failed, Fails: bs.fails, Undo: bs.undo, Staged: bs.staged}
	bs.mu.Unlock()
	return st
}

// Failures counts the automatic backups of this run that failed, and tells
// why the last did, for the windows to tell each once: cheap, for each
// frame.
func (b *Backups) Failures() (int, error) {
	bs := &b.a.backups
	bs.mu.Lock()
	defer bs.mu.Unlock()
	return bs.fails, bs.failed
}

// CanUndo reports whether the last restore can be undone now: cheap, for
// each frame.
func (b *Backups) CanUndo() bool {
	bs := &b.a.backups
	bs.mu.Lock()
	defer bs.mu.Unlock()
	return bs.undo != nil && !bs.staged
}

// DefaultFolder is the folder of the automatic backups unless the user
// chooses another: "MixMusic backups" in the Documents folder, or none in
// the Flatpak, which can't write there unless the user chooses it.
func (b *Backups) DefaultFolder() string { return b.a.defaultBackupFolder() }

func (a *App) defaultBackupFolder() string {
	bs := &a.backups
	bs.folderOnce.Do(func() { bs.folder = a.findBackupFolder() })
	return bs.folder
}

// findBackupFolder finds the default folder of the automatic backups.
func (a *App) findBackupFolder() string {
	if a.opts.BackupsDir != "" {
		return a.opts.BackupsDir
	}
	if inFlatpak() {
		return ""
	}
	docs, err := mygo.App.Path(mygo.PathDocuments)
	if err != nil || docs == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		docs = filepath.Join(home, "Documents")
	}
	return filepath.Join(docs, "MixMusic backups")
}

// backupFolder is the folder of the automatic backups of the settings s.
func (a *App) backupFolder(s settings.Settings) string {
	if s.BackupFolder != "" {
		return s.BackupFolder
	}
	return a.defaultBackupFolder()
}

// backupSource is what a backup made at now is made of.
func (a *App) backupSource(now time.Time) backup.Source {
	s := a.settings.Get().Clean()
	return backup.Source{DataDir: a.opts.DataDir, App: a.version(), MusicFolders: s.MusicFolders,
		DownloadsFolder: a.downloads.Folder(), Downloads: downloadList(a.downloads.Listing()), Now: now}
}

// downloadList is the list of a backup's Downloads part of the listing of
// the downloads.
func downloadList(l downloads.Listing) backup.DownloadList {
	out := backup.DownloadList{Tracks: l.Tracks, Collections: []backup.DownloadCollection{}}
	if out.Tracks == nil {
		out.Tracks = []media.Track{}
	}
	for _, c := range l.Collections {
		out.Collections = append(out.Collections, backup.DownloadCollection{ID: c.ID, Kind: c.Kind, Title: c.Title,
			Source: c.Source, Artwork: c.Artwork, Total: c.Total, Tracks: c.Items})
	}
	return out
}

// keepPending saves what waits a moment to be saved, as the queue and
// where the episodes were left, for a backup to hold it.
func (a *App) keepPending() {
	if p := a.goPlayer.Load(); p != nil {
		p.Flush()
	}
	if err := a.positions.Flush(); err != nil {
		log.Printf("mixmusic: keeping where episodes were left: %v", err)
	}
}

// backupRunning marks a backup being written, until done: ok is false
// when one already is.
func (a *App) backupRunning() (done func(), ok bool) {
	bs := &a.backups
	bs.mu.Lock()
	if bs.running {
		bs.mu.Unlock()
		return nil, false
	}
	bs.running = true
	bs.mu.Unlock()
	notify()
	return func() {
		bs.mu.Lock()
		bs.running = false
		bs.mu.Unlock()
		notify()
	}, true
}

// errBackupBusy is a backup asked for while another is written.
var errBackupBusy = errors.New("a backup is being written already")

// Write writes a backup of parts to path, as the user asked.
func (b *Backups) Write(path string, parts []backup.Part) error {
	done, ok := b.a.backupRunning()
	if !ok {
		return wordedLater(errBackupBusy.Error(), locales.BackupBusy, errBackupBusy)
	}
	defer done()
	b.a.keepPending()
	_, err := backup.Write(path, b.a.backupSource(time.Now()), parts)
	if err != nil {
		log.Printf("mixmusic: backing up to %s: %v", path, err)
	}
	return backupWords(err)
}

// watchBackups makes the automatic backups: a moment after the start, the
// one missed while MixMusic was closed, then each as it falls due.
func (a *App) watchBackups() {
	go func() {
		time.Sleep(backupStartWait)
		for {
			a.autoBackup(time.Now(), backupDue)
			time.Sleep(backupEvery)
		}
	}()
}

// backupAtQuit makes the backup of each quit, once, when the settings ask
// for it: flush runs as MixMusic quits.
func (a *App) backupAtQuit() {
	if a.settings.Get().Clean().BackupEvery == settings.BackupAtQuit {
		a.autoBackup(time.Now(), backupQuit)
	}
}

// backupWhy tells what has an automatic backup made.
type backupWhy int

const (
	// backupDue: the look, at the start and then each hour, for the one
	// due.
	backupDue backupWhy = iota
	// backupQuit: MixMusic quits.
	backupQuit
	// backupNow: the user turned the backups on, or chose their folder,
	// which takes one at once, whatever changed.
	backupNow
)

// autoBackup makes the automatic backup the settings ask for, as why
// tells: due at now, unless nothing changed since the last; as MixMusic
// quits; or at once. One that failed is tried again at the next start, or
// as the user asks. It returns why it failed.
func (a *App) autoBackup(now time.Time, why backupWhy) error {
	s := a.settings.Get().Clean()
	bs := &a.backups
	bs.mu.Lock()
	kept := a.keptBackups().Get()
	start := !bs.started
	if why == backupDue {
		bs.started = true
	}
	skip := s.BackupEvery == settings.BackupOff || bs.failed != nil && why != backupNow
	quitting := why == backupQuit
	switch {
	case why == backupNow:
	case quitting:
		skip = skip || bs.quit
		bs.quit = true
	case s.BackupEvery == settings.BackupAtQuit:
		// A start backs up what the last quit didn't, as when MixMusic
		// ended without quitting.
		skip = skip || !start
	default:
		skip = skip || !backup.Due(s.BackupEvery, latest(kept.Last, kept.Checked), now)
	}
	bs.mu.Unlock()
	if skip {
		return nil
	}
	done, ok := a.backupRunning()
	if !ok {
		return nil
	}
	defer done()
	if !quitting {
		a.keepPending()
	}
	src := a.backupSource(now)
	sum, err := backup.Sum(src, backup.Parts)
	if err == nil && sum == kept.Sum && why != backupNow {
		a.keepBackups(func(k *backupsKept) { k.Checked = now })
		return nil
	}
	folder := a.backupFolder(s)
	if err == nil {
		err = a.readyBackupFolder(folder, s.BackupFolder == "")
	}
	var m backup.Manifest
	var path string
	if err == nil {
		m, path, err = backup.Auto(folder, s.BackupKeep, src)
		if err != nil && path != "" {
			// Written, but the old ones didn't all go: it counts.
			log.Printf("mixmusic: deleting the oldest automatic backups: %v", err)
			err = nil
		}
	}
	if err != nil {
		log.Printf("mixmusic: backing up automatically to %s: %v", folder, err)
		err = backupWords(err)
		bs.mu.Lock()
		bs.failed = err
		bs.fails++
		bs.mu.Unlock()
		notify()
		return err
	}
	a.keepBackups(func(k *backupsKept) {
		*k = backupsKept{Last: m.Created, File: path, Sum: m.Sum(), Checked: now}
	})
	bs.mu.Lock()
	bs.failed = nil
	bs.mu.Unlock()
	notify()
	return nil
}

// latest is the later of two times.
func latest(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}

// keepBackups changes what backupsName keeps.
func (a *App) keepBackups(fn func(k *backupsKept)) {
	bs := &a.backups
	bs.mu.Lock()
	defer bs.mu.Unlock()
	if err := a.keptBackups().Update(func(k *backupsKept) error { fn(k); return nil }); err != nil {
		log.Printf("mixmusic: keeping %s: %v", backupsName, err)
	}
}

// errNoBackupFolder is the Flatpak's, until the user chooses a folder.
var errNoBackupFolder = errors.New("no folder is chosen for the backups")

// readyBackupFolder makes the folder of the automatic backups when it is
// the default one, which the user didn't choose.
func (a *App) readyBackupFolder(folder string, byDefault bool) error {
	switch {
	case folder == "":
		return wordedLater(errNoBackupFolder.Error(), locales.BackupNoFolder, errNoBackupFolder)
	case byDefault:
		if err := os.MkdirAll(folder, 0o755); err != nil {
			return &backup.FolderError{Folder: folder, Kind: backup.ErrFolderReadOnly, Err: err}
		}
	}
	return nil
}

// BackUpNow makes an automatic backup at once, as the user turned them on,
// or chose their folder, whatever failed before or changed since: none
// while they're off.
func (b *Backups) BackUpNow() {
	bs := &b.a.backups
	bs.mu.Lock()
	bs.failed = nil
	bs.mu.Unlock()
	notify()
	go b.a.autoBackup(time.Now(), backupNow)
}

// Restore is a backup opened for its restore: what it holds, and what
// it came from.
type Restore struct {
	b *backup.Backup
	// Manifest is the backup's manifest, and Counts what it holds.
	Manifest backup.Manifest
	Counts   backup.Counts
	// Here are the folders of music of this computer; Moved those the
	// backup came from that aren't among them, and Folders the match of
	// each that Suggest found.
	Here    []string
	Moved   []string
	Folders map[string]string
	data    string
}

// Open opens the backup at path for its restore: checked, and unpacked,
// until Close or Stage.
func (b *Backups) Open(path string) (*Restore, error) {
	a := b.a
	bk, err := backup.Open(path, backup.Work(a.opts.DataDir))
	if err != nil {
		log.Printf("mixmusic: opening the backup %s: %v", path, err)
		return nil, backupWords(err)
	}
	m := bk.Manifest
	here := a.settings.Get().Clean().MusicFolders
	r := &Restore{b: bk, Manifest: m, Counts: bk.Counts(), Here: here, data: a.opts.DataDir,
		Moved: backup.Moved(m.System, m.MusicFolders, here), Folders: backup.Suggest(m.System, m.MusicFolders, here)}
	return r, nil
}

// Added counts what adding the backup's library to the one here adds, of
// its songs on this computer where folders moves them.
func (r *Restore) Added(folders map[string]string) (backup.Counts, error) {
	return r.b.Added(r.data, folders)
}

// Close removes what the backup unpacked.
func (r *Restore) Close() {
	if r != nil {
		r.b.Close()
	}
}

// Stage stages the restore of what c chose of the backup, which MixMusic
// swaps in as it restarts: the windows restart it then. The backup is
// spent.
func (b *Backups) Stage(r *Restore, c backup.Choice) error {
	b.a.keepPending()
	err := r.b.Stage(b.a.opts.DataDir, c)
	r.Close()
	if err != nil {
		log.Printf("mixmusic: staging the restore of %s: %v", r.b.Path, err)
		return backupWords(err)
	}
	b.a.backups.mu.Lock()
	b.a.backups.staged = true
	b.a.backups.mu.Unlock()
	notify()
	return nil
}

// StageUndo stages the undo of the last restore, which MixMusic swaps in
// as it restarts: the windows restart it then.
func (b *Backups) StageUndo() error {
	if err := backup.StageUndo(b.a.opts.DataDir); err != nil {
		log.Printf("mixmusic: staging the undo of the last restore: %v", err)
		return err
	}
	b.a.backups.mu.Lock()
	b.a.backups.staged, b.a.backups.undo = true, nil
	b.a.backups.mu.Unlock()
	notify()
	return nil
}

// TakeApplied returns what the start swapped in, once: for the main window
// to tell.
func (b *Backups) TakeApplied() *backup.Applied {
	bs := &b.a.backups
	bs.mu.Lock()
	defer bs.mu.Unlock()
	ap := bs.applied
	bs.applied = nil
	return ap
}

// takeRestored keeps what the start swapped in, for the windows to tell,
// and downloads again what the restore asked for; and notes whether the
// last restore can be undone, and whether one waits for the restart.
func (a *App) takeRestored() {
	if u, ok := backup.LastRestore(a.opts.DataDir); ok {
		a.backups.undo = &u
	}
	a.backups.staged = backup.Staged(a.opts.DataDir)
	ap := a.opts.Restored
	if ap == nil || !ap.Restored && !ap.Undone && ap.Err == nil {
		return
	}
	if ap.Err != nil {
		log.Printf("mixmusic: swapping in the restore staged: %v", ap.Err)
	}
	a.backups.mu.Lock()
	a.backups.applied = ap
	a.backups.mu.Unlock()
	if l := ap.Downloads; l != nil {
		n := a.downloadAgain(*l)
		log.Printf("mixmusic: downloading %d tracks of the backup again", n)
	}
}

// downloadAgain downloads again the tracks of a backup's list of downloads:
// those downloaded on their own, and the collections downloaded whole,
// which go on taking in what is added to them. It returns how many tracks
// it added.
func (a *App) downloadAgain(l backup.DownloadList) int {
	q := a.quality()
	n := a.downloads.Add(l.Tracks, q)
	for _, c := range l.Collections {
		col := downloads.Collection{ID: c.ID, Kind: c.Kind, Title: c.Title, Source: c.Source, Artwork: c.Artwork, Total: c.Total}
		n += a.downloads.AddCollection(col, c.Tracks, q)
	}
	return n
}

// backupWords is err, of a backup that couldn't be written or opened, with
// the words the windows show of it.
func backupWords(err error) error {
	var be *backup.Error
	var fe *backup.FolderError
	switch {
	case err == nil:
		return nil
	case errors.As(err, &be):
		return wordedLater(err.Error(), func() string { return backupProblem(be) }, err)
	case errors.As(err, &fe):
		return wordedLater(err.Error(), func() string { return folderProblem(fe) }, err)
	}
	return err
}

// backupProblem is why a backup was refused, as the windows say it.
func backupProblem(e *backup.Error) string {
	switch e.Problem {
	case backup.NotBackup:
		return locales.BackupErrNotBackup()
	case backup.Damaged:
		if e.Entry != "" {
			return locales.BackupErrDamagedEntry(e.Entry)
		}
		return locales.BackupErrDamaged()
	case backup.Newer:
		return locales.BackupErrNewer(e.App)
	case backup.TooLarge:
		return locales.BackupErrTooLarge(backup.MaxPacked >> 20)
	case backup.TooLargeUnpacked:
		return locales.BackupErrTooLargeUnpacked(backup.MaxUnpacked >> 30)
	case backup.Outside:
		return locales.BackupErrOutside(e.Entry)
	}
	return e.Error()
}

// folderProblem is why the folder of the automatic backups took none, as
// the windows say it.
func folderProblem(e *backup.FolderError) string {
	switch {
	case errors.Is(e.Kind, backup.ErrDiskFull):
		return locales.BackupErrDiskFull()
	case errors.Is(e.Kind, backup.ErrFolderLost):
		return locales.BackupErrFolderLost(e.Folder)
	case errors.Is(e.Kind, backup.ErrFolderReadOnly):
		return locales.BackupErrFolderReadOnly(e.Folder)
	}
	return locales.BackupErrFolderMissing(e.Folder)
}
