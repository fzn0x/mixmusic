package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/backup"
	"github.com/fzn0x/mixmusic/internal/library"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/locales"
)

// backupMade is when the backups of these tests were made.
var backupMade = time.Date(2026, 11, 2, 9, 30, 0, 0, time.UTC)

// backupOf writes a backup of every part of a data folder of its own: two
// liked songs, a mix, three plays, a theme, a track in the queue and one
// downloaded. It returns its file.
func backupOf(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	lib := library.Data{Liked: testTracks[:2], Playlists: []library.Playlist{{ID: "m1", Title: "Road", Tracks: testTracks[2:3]}}}
	data, _ := json.Marshal(lib)
	s := settings.Default()
	s.Theme = "midnight"
	set, _ := json.Marshal(s)
	q, _ := json.Marshal(map[string]any{"tracks": testTracks[:1], "index": 0})
	files := map[string]string{
		"library.json":                      string(data),
		"plays.jsonl":                       `{"at":"2026-11-01T10:00:00Z","id":"yt:1","title":"Midnight City"}` + "\n" + `{"at":"2026-11-01T11:00:00Z","id":"yt:3","title":"Strobe"}` + "\n" + `{"at":"2026-11-01T12:00:00Z","id":"yt:4","title":"Windowlicker"}` + "\n",
		"settings.json":                     string(set),
		"addons/themes/midnight/theme.json": `{"id":"midnight"}`,
		"queue.json":                        string(q),
	}
	for name, text := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		os.MkdirAll(filepath.Dir(path), 0o755)
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(t.TempDir(), "MixMusic backup 2026-11-02.zip")
	_, err := backup.Write(path, backup.Source{DataDir: dir, App: "0.5.12",
		Downloads: backup.DownloadList{Tracks: testTracks[5:6]}, Now: backupMade}, backup.Parts)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

// backupDialogs answers the dialogs of files as the test says: the open
// dialog with open, the save dialog with save; restarts counts the
// restarts asked for, and shown the files shown in their folder.
func backupDialogs(t *testing.T, open func(mygo.OpenDialogOptions) ([]string, error), save func(mygo.SaveDialogOptions) (string, error)) (restarts *int, shown *[]string) {
	t.Helper()
	restarts, shown = new(int), new([]string)
	savedOpen, savedSave, savedRestart, savedShow := openFileDialog, saveFileDialog, restartApp, showInFolder
	t.Cleanup(func() {
		openFileDialog, saveFileDialog, restartApp, showInFolder = savedOpen, savedSave, savedRestart, savedShow
	})
	if open != nil {
		openFileDialog = open
	}
	if save != nil {
		saveFileDialog = save
	}
	restartApp = func(*shell) { *restarts++ }
	showInFolder = func(path string) { *shown = append(*shown, path) }
	return restarts, shown
}

// clickRole clicks the control of role labeled name.
func clickRole(t *testing.T, s *shell, tt *ui.Tester, role ui.Role, name string) {
	t.Helper()
	reveal(tt, name)
	e, ok := tt.AccessByRole(role, name)
	if !ok {
		t.Fatalf("no %v labeled %q; the window shows %q", role, name, tt.Texts())
	}
	tt.ClickAt(e.Bounds.X+e.Bounds.W/2, e.Bounds.Y+e.Bounds.H/2)
	s.drain()
	tt.Frame()
}

// TestBackupByHand backs up from Settings › Desktop: the dialog ticks
// every part, tells what no backup holds, and saves a file named for the
// day where the user chooses, of the parts left ticked; a toast shows it
// in its folder.
func TestBackupByHand(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mine")
	var asked mygo.SaveDialogOptions
	_, shown := backupDialogs(t, nil, func(o mygo.SaveDialogOptions) (string, error) { asked = o; return path, nil })
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 1000)
	show(s, tt, "/settings/desktop")
	if !reveal(tt, locales.BackupAbout()) {
		t.Fatalf("no Backups group: %q", tt.Texts())
	}
	click(t, s, tt, "Back up…")
	for _, want := range []string{"Back up MixMusic", "Library", "Settings", "Add-ons", "Queue and episode positions", "List of downloads", locales.BackupNever(), locales.BackupPrivate()} {
		if !tt.HasText(want) {
			t.Errorf("the dialog doesn't show %q: %q", want, tt.Texts())
		}
	}
	box, ok := tt.AccessByRole(ui.RoleCheckBox, "Add-ons")
	if !ok || !box.Checked {
		t.Fatalf("Add-ons isn't ticked: %+v", box)
	}
	clickRole(t, s, tt, ui.RoleCheckBox, "Add-ons")
	if b, ok := tt.AccessByRole(ui.RoleCheckBox, "Add-ons"); !ok || b.Checked {
		t.Fatalf("Add-ons is still ticked: %+v", b)
	}
	clickRole(t, s, tt, ui.RoleButton, "Save the backup")
	until(t, s, tt, "the backup", func() bool { return tt.HasText("Saved mine.zip") })
	if !strings.HasPrefix(asked.DefaultPath, "MixMusic backup ") || !strings.HasSuffix(asked.DefaultPath, ".zip") {
		t.Errorf("the file is offered as %q", asked.DefaultPath)
	}
	b, err := backup.Open(path+".zip", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if want := []backup.Part{backup.Library, backup.Settings, backup.Queue, backup.Downloads}; !slices.Equal(b.Manifest.Parts, want) {
		t.Errorf("the backup holds %q, want %q", b.Manifest.Parts, want)
	}
	click(t, s, tt, showInFolderLabel())
	if !slices.Equal(*shown, []string{path + ".zip"}) {
		t.Errorf("show showed %q", *shown)
	}
}

// TestBackupAutomatically chooses Daily in Back up automatically: a backup
// goes to the folder at once, the rows of the folder and of how many to
// keep show, and the row tells when the last was made. A folder that's
// gone fails, with a notice, and the row says why.
func TestBackupAutomatically(t *testing.T) {
	folder := t.TempDir()
	gone := filepath.Join(t.TempDir(), "gone")
	backupDialogs(t, func(mygo.OpenDialogOptions) ([]string, error) { return []string{gone}, nil }, nil)
	s := newTestShellOf(t, func(o *app.Options) { o.BackupsDir = folder })
	tt := testerOf(t, s, 1280, 1000)
	show(s, tt, "/settings/desktop")
	if tt.HasText("Folder for backups") {
		t.Error("the folder shows while automatic backups are off")
	}
	clickRole(t, s, tt, ui.RolePopUpButton, "Back up automatically")
	click(t, s, tt, "Daily")
	if got := s.svc.Preferences.Get().BackupEvery; got != settings.BackupDaily {
		t.Fatalf("backupEvery is %q", got)
	}
	until(t, s, tt, "the first backup", func() bool {
		found, _ := filepath.Glob(filepath.Join(folder, "MixMusic backup *.zip"))
		return len(found) == 1 && slices.ContainsFunc(tt.Texts(), func(txt string) bool { return strings.HasPrefix(txt, "Last backup: ") })
	})
	for _, want := range []string{"Folder for backups", folder, "Keep", "The last 5"} {
		if !reveal(tt, want) {
			t.Errorf("Settings › Desktop doesn't show %q: %q", want, tt.Texts())
		}
	}
	click(t, s, tt, "Change…")
	until(t, s, tt, "the backup to a folder that's gone", func() bool {
		return tt.HasText("Couldn't back up automatically: the folder " + gone + " isn't there")
	})
	if !reveal(tt, "The last automatic backup failed: the folder "+gone+" isn't there. MixMusic tries again at its next start.") {
		t.Errorf("the row doesn't say why: %q", tt.Texts())
	}
}

// TestRestoreBackup restores a backup: its dialog tells where it comes
// from and what each part holds, counts what adding its library adds, and
// stages what's ticked, then restarts MixMusic, which swaps it in as it
// starts.
func TestRestoreBackup(t *testing.T) {
	file := backupOf(t)
	restarts, _ := backupDialogs(t, func(o mygo.OpenDialogOptions) ([]string, error) { return []string{file}, nil }, nil)
	s := newTestShellOf(t, func(o *app.Options) {
		lib, _ := json.Marshal(library.Data{Liked: testTracks[:1]})
		os.WriteFile(filepath.Join(o.DataDir, "library.json"), lib, 0o644)
	})
	tt := testerOf(t, s, 1280, 1200)
	data := s.svc.System.Info().DataDir
	show(s, tt, "/settings/desktop")
	click(t, s, tt, "Restore…")
	until(t, s, tt, "the restore's dialog", func() bool { return tt.HasText("Restore a backup") })
	from := locales.BackupRestoreFrom(backupMade.Local(), "0.5.12", systemName(runtime.GOOS))
	for _, want := range []string{from, "2 liked songs, 0 dislikes, 1 mix, 3 plays, 0 followed artists and 0 followed podcasts",
		"1 add-on. Plugins come back switched off, to be trusted again.", "1 track in the queue, and where you left your episodes",
		"Download 1 song again", locales.BackupRestoreWarning()} {
		if !tt.HasText(want) {
			t.Errorf("the dialog doesn't show %q: %q", want, tt.Texts())
		}
	}
	click(t, s, tt, s.lk.words("Add to my library"))
	until(t, s, tt, "what it adds", func() bool {
		return tt.HasText("Adds 1 liked song, 0 dislikes, 1 mix, 3 plays, 0 followed artists and 0 followed podcasts.")
	})
	clickRole(t, s, tt, ui.RoleCheckBox, "Settings")
	click(t, s, tt, "Restore and restart")
	until(t, s, tt, "the restart", func() bool { return *restarts == 1 })
	if !backup.Staged(data) {
		t.Fatal("nothing is staged")
	}
	if tt.HasText("Restore a backup") {
		t.Error("the dialog stays open")
	}
	if !reveal(tt, "Restarting…") {
		t.Errorf("the Restore row doesn't tell of the restart: %q", tt.Texts())
	}
	// The next start.
	ap := backup.Apply(data)
	if ap.Err != nil || !ap.Restored {
		t.Fatalf("Apply: %+v", ap)
	}
	var lib library.Data
	raw, _ := os.ReadFile(filepath.Join(data, "library.json"))
	json.Unmarshal(raw, &lib)
	if len(lib.Liked) != 2 || len(lib.Playlists) != 1 {
		t.Errorf("the library after: %d liked, %d mixes", len(lib.Liked), len(lib.Playlists))
	}
	if raw, _ := os.ReadFile(filepath.Join(data, "settings.json")); strings.Contains(string(raw), "midnight") {
		t.Error("the settings, unticked, came")
	}
}

// TestRestoreRefused tells why a file can't be restored: one that isn't a
// backup, and the restore dialog doesn't open.
func TestRestoreRefused(t *testing.T) {
	notes := filepath.Join(t.TempDir(), "notes.zip")
	os.WriteFile(notes, []byte("not a backup"), 0o644)
	backupDialogs(t, func(mygo.OpenDialogOptions) ([]string, error) { return []string{notes}, nil }, nil)
	s := newTestShell(t)
	tt := tester(t, s)
	show(s, tt, "/settings/desktop")
	click(t, s, tt, "Restore…")
	until(t, s, tt, "the refusal", func() bool { return tt.HasText("Couldn't restore notes.zip: it isn't a MixMusic backup") })
	if tt.HasText("Restore a backup") {
		t.Error("the restore's dialog opened")
	}
}

// TestRestoreTold tells as MixMusic starts what it swapped in, and the
// downloads it downloads again; or why it couldn't, every file as it was.
func TestRestoreTold(t *testing.T) {
	list := backup.DownloadList{Tracks: testTracks[:2]}
	s := newTestShellOf(t, func(o *app.Options) {
		o.Restored = &backup.Applied{Restored: true, Backup: backupMade, Downloads: &list}
	})
	tt := tester(t, s)
	until(t, s, tt, "the restore's notices", func() bool {
		return tt.HasText(locales.BackupRestored(backupMade.Local())) && tt.HasText("Downloading 2 songs of the backup again")
	})
	if got := s.svc.Downloads.Snapshot().Status().Waiting; got != 2 {
		t.Errorf("%d tracks download again", got)
	}

	s = newTestShellOf(t, func(o *app.Options) { o.Restored = &backup.Applied{Err: errors.New("the disk is busy")} })
	tt = tester(t, s)
	until(t, s, tt, "the failure's notice", func() bool {
		return tt.HasText("Couldn't restore the backup, so every file stayed as it was: the disk is busy")
	})
}

// TestUndoRestore offers to undo the last restore, in Settings › Desktop
// and the command palette, and stages the undo once the user confirms,
// then restarts MixMusic.
func TestUndoRestore(t *testing.T) {
	file := backupOf(t)
	restarts, _ := backupDialogs(t, nil, nil)
	var data string
	s := newTestShellOf(t, func(o *app.Options) {
		data = o.DataDir
		b, err := backup.Open(file, backup.Work(o.DataDir))
		if err != nil {
			t.Fatal(err)
		}
		if err := b.Stage(o.DataDir, backup.Choice{Parts: []backup.Part{backup.Library}}); err != nil {
			t.Fatal(err)
		}
		b.Close()
		ap := backup.Apply(o.DataDir)
		o.Restored = &ap
	})
	tt := testerOf(t, s, 1280, 1000)
	if !slices.ContainsFunc(s.commands(), func(c command) bool { return c.id == "backup.undo" }) {
		t.Error("the palette doesn't offer the undo")
	}
	show(s, tt, "/settings/desktop")
	if !reveal(tt, "Undo the last restore") {
		t.Fatalf("no undo: %q", tt.Texts())
	}
	click(t, s, tt, "Undo")
	until(t, s, tt, "the question", func() bool { return tt.HasText("Undo the last restore?") })
	click(t, s, tt, "Undo and restart")
	until(t, s, tt, "the restart", func() bool { return *restarts == 1 })
	if !backup.Staged(data) {
		t.Error("the undo isn't staged")
	}
	if slices.ContainsFunc(s.commands(), func(c command) bool { return c.id == "backup.undo" }) {
		t.Error("the palette offers the undo again")
	}
}

// TestPseudoLocaleBackups shows the backups' rows and dialogs in en-XA:
// nothing but the names of folders and files is English.
func TestPseudoLocaleBackups(t *testing.T) {
	usePseudoLocale(t)
	file := backupOf(t)
	folder := t.TempDir()
	backupDialogs(t, func(mygo.OpenDialogOptions) ([]string, error) { return []string{file}, nil }, nil)
	s := newTestShellOf(t, func(o *app.Options) {
		o.BackupsDir = folder
		os.WriteFile(filepath.Join(o.DataDir, "settings.json"), []byte(`{"layout": "tiles", "backupEvery": "weekly"}`), 0o644)
	})
	tt := testerOf(t, s, 1280, 1400)
	tt.SetLocale("en-XA")
	show(s, tt, "/settings/desktop")
	xa := locales.In("en-XA")
	if !reveal(tt, xa.BackupKeepTitle()) {
		t.Fatalf("no Keep row: %q", tt.Texts())
	}
	checkPseudo(t, "Settings › Desktop › Backups", tt.Texts(), folder)
	s.openBackupDialog()
	tt.Frame()
	checkPseudo(t, "The dialog of a backup", tt.Texts(), folder)
	s.backups.backingUp = false
	s.chooseRestore()
	until(t, s, tt, "the restore's dialog", func() bool { return tt.HasText(xa.BackupRestoreDialogTitle()) })
	s.backups.merge = 1
	until(t, s, tt, "what it adds", func() bool { return tt.HasText(xa.BackupRestoreAdds(2, 0, 1, 3, 0, 0)) })
	checkPseudo(t, "The dialog of a restore", tt.Texts(), folder, filepath.Base(file))
}

// TestBackupCommands backs up and restores from the command palette.
func TestBackupCommands(t *testing.T) {
	s := newTestShell(t)
	var ids []string
	for _, c := range s.commands() {
		if strings.HasPrefix(c.id, "backup.") {
			ids = append(ids, c.id)
		}
	}
	if !slices.Equal(ids, []string{"backup.now", "backup.restore"}) {
		t.Errorf("the backups' commands are %q", ids)
	}
}
