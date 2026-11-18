package backup

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/fzn0x/mixmusic/internal/settings"
)

// runtimeGOOS is the system MixMusic runs on: tests change it.
var runtimeGOOS = runtime.GOOS

// SingleName is the file of the automatic backups kept as one, written
// over each time.
const SingleName = "MixMusic backup.zip"

// AutoName is the file of an automatic backup made at t:
// "MixMusic backup 2026-11-18 140509.zip".
func AutoName(t time.Time) string { return "MixMusic backup " + t.Format("2006-01-02 150405") + ".zip" }

// autoRe matches the names AutoName makes: the only files Prune deletes.
var autoRe = regexp.MustCompile(`^MixMusic backup \d{4}-\d{2}-\d{2} \d{6}\.zip$`)

// Due reports whether an automatic backup made as every tells, one of
// settings' Backup values, is due at now, the last one made at last:
// once a day, a week or a month after it, or as soon as there was none.
// Those made as MixMusic quits are due at each quit, and at a start that
// follows a quit without one, as the caller tells by what changed.
func Due(every string, last, now time.Time) bool {
	if every == settings.BackupOff {
		return false
	}
	if last.IsZero() || every == settings.BackupAtQuit {
		return true
	}
	switch every {
	case settings.BackupDaily:
		return !now.Before(last.Add(24 * time.Hour))
	case settings.BackupWeekly:
		return !now.Before(last.AddDate(0, 0, 7))
	case settings.BackupMonthly:
		return !now.Before(last.AddDate(0, 1, 0))
	}
	return false
}

// Prune deletes the automatic backups of the folder dir, the oldest first,
// until it holds keep of them: only files whose names AutoName makes.
func Prune(dir string, keep int) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var names []string
	for _, e := range entries {
		if e.Type().IsRegular() && autoRe.MatchString(e.Name()) {
			names = append(names, e.Name())
		}
	}
	// The names sort as their times do.
	slices.Sort(names)
	var errs []error
	for len(names) > keep {
		if err := os.Remove(filepath.Join(dir, names[0])); err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, err)
		}
		names = names[1:]
	}
	return errors.Join(errs...)
}

// The problems of the folder of the automatic backups, which a
// FolderError tells.
var (
	ErrFolderMissing  = errors.New("the folder isn't there")
	ErrFolderReadOnly = errors.New("the folder can't be written")
	ErrDiskFull       = errors.New("the disk is full")
	// ErrFolderLost: the Flatpak's sandbox no longer lets MixMusic reach
	// the folder, which the user chooses again.
	ErrFolderLost = errors.New("MixMusic can no longer reach the folder")
)

// FolderError is why an automatic backup couldn't be written to Folder:
// Kind, one of the ErrFolder errors or ErrDiskFull, of Err.
type FolderError struct {
	Folder string
	Kind   error
	Err    error
}

func (e *FolderError) Error() string {
	if e.Err == nil {
		return e.Kind.Error() + ": " + e.Folder
	}
	return e.Kind.Error() + ": " + e.Folder + ": " + e.Err.Error()
}

func (e *FolderError) Unwrap() []error { return []error{e.Kind, e.Err} }

// Auto writes an automatic backup of every part of src to the folder dir,
// and keeps the last keep of them, as settings' BackupKeep tells: all for
// 0, and one file written over for 1. It returns the backup's manifest,
// and its file. A folder that is missing, can't be written, is full, or
// that the Flatpak's sandbox lost, is a *FolderError.
func Auto(dir string, keep int, src Source) (Manifest, string, error) {
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return Manifest{}, "", folderError(dir, err)
	}
	if src.Now.IsZero() {
		src.Now = time.Now()
	}
	name := AutoName(src.Now)
	if keep == 1 {
		name = SingleName
	}
	path := filepath.Join(dir, name)
	m, err := Write(path, src, Parts)
	if err != nil {
		var be *Error
		if errors.As(err, &be) {
			return m, "", err
		}
		return m, "", folderError(dir, err)
	}
	switch {
	case keep == 1:
		err = Prune(dir, 0)
	case keep > 1:
		err = Prune(dir, keep)
	}
	return m, path, err
}

// folderError tells why the folder dir couldn't take a backup, of err.
func folderError(dir string, err error) error {
	kind := ErrFolderMissing
	switch {
	case err != nil && diskFull(err):
		kind = ErrDiskFull
	case err != nil && errors.Is(err, fs.ErrPermission):
		kind = ErrFolderReadOnly
	case err != nil && !errors.Is(err, fs.ErrNotExist):
		return &FolderError{Folder: dir, Kind: ErrFolderReadOnly, Err: err}
	}
	if kind != ErrDiskFull && os.Getenv("FLATPAK_ID") != "" && strings.HasPrefix(dir, "/run/user/") {
		// The documents portal's folders, which the sandbox lets go.
		kind = ErrFolderLost
	}
	return &FolderError{Folder: dir, Kind: kind, Err: err}
}
