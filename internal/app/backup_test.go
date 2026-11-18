package app

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/backup"
)

// backupsIn lists the automatic backups of the folder dir.
func backupsIn(t *testing.T, dir string) []string {
	t.Helper()
	found, err := filepath.Glob(filepath.Join(dir, "MixMusic backup*.zip"))
	if err != nil {
		t.Fatal(err)
	}
	return found
}

// TestAutoBackupDaily backs up once a day: the first at once, none before
// a day passed, none a day later when nothing changed, and one once
// something did.
func TestAutoBackupDaily(t *testing.T) {
	a := fileApp(t, `{"backupEvery": "daily"}`)
	a.opts.BackupsDir = filepath.Join(t.TempDir(), "MixMusic backups")
	now := time.Date(2026, 11, 18, 9, 0, 0, 0, time.Local)
	if err := a.autoBackup(now, backupDue); err != nil {
		t.Fatal(err)
	}
	if got := backupsIn(t, a.opts.BackupsDir); len(got) != 1 {
		t.Fatalf("the first backup made %q", got)
	}
	if st := a.Backups().Status(); !st.Last.Equal(now) || st.Folder != a.opts.BackupsDir {
		t.Errorf("status %+v", st)
	}
	a.autoBackup(now.Add(time.Hour), backupDue)
	a.autoBackup(now.Add(25*time.Hour), backupDue)
	if got := backupsIn(t, a.opts.BackupsDir); len(got) != 1 {
		t.Fatalf("backups made with nothing changed: %q", got)
	}
	if err := os.WriteFile(filepath.Join(a.opts.DataDir, "positions.json"), []byte(`{"x":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	a.autoBackup(now.Add(26*time.Hour), backupDue)
	if got := backupsIn(t, a.opts.BackupsDir); len(got) != 1 {
		t.Fatalf("a backup made before a day passed since the last look: %q", got)
	}
	a.autoBackup(now.Add(50*time.Hour), backupDue)
	if got := backupsIn(t, a.opts.BackupsDir); len(got) != 2 {
		t.Fatalf("after a change, a day later: %q", got)
	}
}

// TestAutoBackupAtQuit backs up as MixMusic quits, once, and at a start
// only what the last quit didn't.
func TestAutoBackupAtQuit(t *testing.T) {
	a := fileApp(t, `{"backupEvery": "quit", "backupKeep": 1}`)
	a.opts.BackupsDir = t.TempDir()
	now := time.Now()
	a.autoBackup(now, backupDue)
	if got := backupsIn(t, a.opts.BackupsDir); len(got) != 1 || filepath.Base(got[0]) != backup.SingleName {
		t.Fatalf("the start's backup made %q", got)
	}
	os.WriteFile(filepath.Join(a.opts.DataDir, "positions.json"), []byte(`{"x":{}}`), 0o644)
	a.autoBackup(now.Add(time.Hour), backupDue)
	before, _ := os.Stat(filepath.Join(a.opts.BackupsDir, backup.SingleName))
	a.backupAtQuit()
	after, _ := os.Stat(filepath.Join(a.opts.BackupsDir, backup.SingleName))
	if before.ModTime().Equal(after.ModTime()) && before.Size() == after.Size() {
		t.Error("the quit backed nothing up")
	}
	if got := backupsIn(t, a.opts.BackupsDir); len(got) != 1 {
		t.Errorf("one file written over left %q", got)
	}
}

// TestAutoBackupFails tells why a backup to a folder that's gone failed,
// counts it, and tries no more until the next start, or the user asks.
func TestAutoBackupFails(t *testing.T) {
	gone := filepath.Join(t.TempDir(), "gone")
	a := fileApp(t, `{"backupEvery": "weekly", "backupFolder": "`+filepath.ToSlash(gone)+`"}`)
	err := a.autoBackup(time.Now(), backupDue)
	var fe *backup.FolderError
	if !errors.As(err, &fe) || !errors.Is(err, backup.ErrFolderMissing) {
		t.Fatalf("autoBackup: %v", err)
	}
	if want := "the folder " + fe.Folder + " isn't there"; ErrorWords(err) != want {
		t.Errorf("the windows say %q, want %q", ErrorWords(err), want)
	}
	if n, why := a.Backups().Failures(); n != 1 || why == nil {
		t.Errorf("failures %d, %v", n, why)
	}
	if err := a.autoBackup(time.Now(), backupDue); err != nil {
		t.Errorf("tried again in the same run: %v", err)
	}
	if err := os.MkdirAll(gone, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := a.autoBackup(time.Now(), backupNow); err != nil {
		t.Fatal(err)
	}
	if got := backupsIn(t, gone); len(got) != 1 {
		t.Errorf("the backup asked for made %q", got)
	}
	if _, why := a.Backups().Failures(); why != nil {
		t.Errorf("the failure stays: %v", why)
	}
}

// TestAutoBackupOff makes no backup while the backups are off, nor in the
// Flatpak before a folder is chosen, which it tells.
func TestAutoBackupOff(t *testing.T) {
	a := fileApp(t, "")
	a.opts.BackupsDir = t.TempDir()
	a.autoBackup(time.Now(), backupNow)
	a.backupAtQuit()
	if got := backupsIn(t, a.opts.BackupsDir); len(got) != 0 {
		t.Errorf("backups made while off: %q", got)
	}
	saved := inFlatpak
	inFlatpak = func() bool { return true }
	t.Cleanup(func() { inFlatpak = saved })
	b := fileApp(t, `{"backupEvery": "daily"}`)
	if err := b.autoBackup(time.Now(), backupDue); !errors.Is(err, errNoBackupFolder) {
		t.Errorf("the Flatpak without a folder: %v", err)
	}
}
