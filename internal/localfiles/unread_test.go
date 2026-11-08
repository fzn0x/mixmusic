package localfiles

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/localfiles/localfilestest"
)

// unreadIndex is a test index of two folders of music in its folder,
// scanned: Kites, of a song of Ada Field, and Shore, of two of Orin Vale.
func unreadIndex(t *testing.T) (ti *testIndex, kites, shore string) {
	t.Helper()
	ti = newIndex(t)
	ti.write(t, "Kites/1.mp3", "Paper|Ada Field", time.Hour)
	ti.write(t, "Shore/1.mp3", "Slow Tide|Orin Vale", time.Hour)
	ti.write(t, "Shore/2.mp3", "Low Water|Orin Vale", time.Hour)
	ti.scan(t)
	if n := len(ti.Tracks()); n != 3 {
		t.Fatalf("%d tracks", n)
	}
	return ti, filepath.Join(ti.dir, "Kites"), filepath.Join(ti.dir, "Shore")
}

// sortedTitles are the titles of the index's tracks, in order.
func sortedTitles(ti *testIndex) []string {
	got := titles(ti.Tracks())
	slices.Sort(got)
	return got
}

// TestUnreadFolder keeps the songs of a folder in a folder of music that
// can't be read, as one MixMusic isn't allowed to, and tells of it, until
// a scan reads it: then a song whose file is gone goes.
func TestUnreadFolder(t *testing.T) {
	ti, _, shore := unreadIndex(t)
	allow := localfilestest.DenyRead(t, shore)
	ti.scan(t)
	if got, want := sortedTitles(ti), []string{"Low Water", "Paper", "Slow Tide"}; !reflect.DeepEqual(got, want) {
		t.Errorf("tracks %q, want %q", got, want)
	}
	st := ti.Status()
	if f := st.Folders[0]; f.Problem != "" || f.Tracks != 3 || !reflect.DeepEqual(f.Unread, []string{shore}) {
		t.Errorf("the folder: %+v", f)
	}
	// Its songs are looked at: on Windows, whose files have permissions of
	// their own, they play; elsewhere, they don't.
	song := filepath.Join(shore, "1.mp3")
	_, err := os.Stat(song)
	if missing := ti.Missing(TrackID(song)); missing != (err != nil) {
		t.Errorf("a song of the folder is missing: %v, but looked at: %v", missing, err)
	}
	if ti.Missing(TrackID(filepath.Join(ti.dir, "Kites", "1.mp3"))) {
		t.Error("a song of a folder read is missing")
	}

	allow()
	if err := os.Remove(song); err != nil {
		t.Fatal(err)
	}
	ti.scan(t)
	if got, want := sortedTitles(ti), []string{"Low Water", "Paper"}; !reflect.DeepEqual(got, want) {
		t.Errorf("read again: tracks %q, want %q", got, want)
	}
	if f := ti.Status().Folders[0]; f.Unread != nil || f.Tracks != 2 {
		t.Errorf("read again, the folder: %+v", f)
	}
}

// TestUnreadMusicFolder keeps the songs of a folder of music whose list of
// files can't be read, as missing, and says why.
func TestUnreadMusicFolder(t *testing.T) {
	ti, _, _ := unreadIndex(t)
	allow := localfilestest.DenyRead(t, ti.dir)
	ti.scan(t)
	f := ti.Status().Folders[0]
	if f.Problem != "MixMusic isn't allowed to read it" || f.Tracks != 3 || len(ti.Tracks()) != 3 {
		t.Errorf("the folder: %+v, %d tracks", f, len(ti.Tracks()))
	}
	if !ti.Missing(ti.Tracks()[0].ID) {
		t.Error("a song of the folder isn't missing")
	}
	allow()
	ti.scan(t)
	if f := ti.Status().Folders[0]; f.Problem != "" || f.Tracks != 3 {
		t.Errorf("read again: %+v", f)
	}
}

// failWalk walks as filepath.WalkDir does, but the folders of fail can't
// be read, as their errors tell, nor can the files of the folders of
// noInfo be looked at, and then is called after each file it comes to.
func failWalk(fail, noInfo map[string]error, then func(path string)) func(string, fs.WalkDirFunc) error {
	return func(root string, fn fs.WalkDirFunc) error {
		return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if why, ok := fail[path]; ok && err == nil && d.IsDir() {
				// As WalkDir tells of a folder whose list of files it
				// can't read: the folder, then its error.
				if err := fn(path, d, nil); err != nil {
					return err
				}
				if err := fn(path, d, &fs.PathError{Op: "open", Path: path, Err: why}); err != nil && err != filepath.SkipDir {
					return err
				}
				return filepath.SkipDir
			}
			if why, ok := noInfo[filepath.Dir(path)]; ok && err == nil && !d.IsDir() {
				d = noInfoEntry{d, &fs.PathError{Op: "lstat", Path: path, Err: why}}
			}
			err = fn(path, d, err)
			if then != nil && err == nil && d != nil && !d.IsDir() {
				then(path)
			}
			return err
		})
	}
}

// noInfoEntry is a file whose information can't be read.
type noInfoEntry struct {
	fs.DirEntry
	err error
}

func (e noInfoEntry) Info() (fs.FileInfo, error) { return nil, e.err }

// TestUnreadErrors keeps the songs of a folder whose list of files fails
// to read, as on a drive with a fault, and of one whose files can't be
// looked at, but drops those of a folder that's gone as the scan comes to
// it.
func TestUnreadErrors(t *testing.T) {
	ti, kites, shore := unreadIndex(t)
	ti.walk = failWalk(map[string]error{shore: errors.New("input/output error")}, map[string]error{kites: fs.ErrPermission}, nil)
	ti.scan(t)
	if got := sortedTitles(ti); len(got) != 3 {
		t.Errorf("tracks %q", got)
	}
	if f := ti.Status().Folders[0]; !reflect.DeepEqual(f.Unread, []string{kites, shore}) || f.Tracks != 3 {
		t.Errorf("the folder: %+v", f)
	}
	ti.walk = failWalk(map[string]error{shore: fs.ErrNotExist}, nil, nil)
	ti.scan(t)
	if got, want := sortedTitles(ti), []string{"Paper"}; !reflect.DeepEqual(got, want) {
		t.Errorf("a folder gone: tracks %q, want %q", got, want)
	}
	if f := ti.Status().Folders[0]; f.Unread != nil {
		t.Errorf("a folder gone is unread: %+v", f)
	}
}

// TestUnplugged keeps the songs of a folder of music whose drive is
// unplugged as the scan reads it, those it found already too.
func TestUnplugged(t *testing.T) {
	ti, _, _ := unreadIndex(t)
	unplugged := ti.dir + "-unplugged"
	ti.walk = failWalk(nil, nil, func(path string) {
		if _, err := os.Stat(unplugged); err != nil {
			if err := os.Rename(ti.dir, unplugged); err != nil {
				t.Error(err)
			}
		}
	})
	ti.scan(t)
	f := ti.Status().Folders[0]
	if !strings.HasPrefix(f.Problem, "It isn't there") || f.Tracks != 3 || len(ti.Tracks()) != 3 {
		t.Errorf("unplugged: %+v, %d tracks", f, len(ti.Tracks()))
	}
	if err := os.Rename(unplugged, ti.dir); err != nil {
		t.Fatal(err)
	}
	ti.walk = nil
	if n := ti.scan(t); n != 0 || len(ti.Tracks()) != 3 {
		t.Errorf("plugged in again: read %d files, %d tracks", n, len(ti.Tracks()))
	}
}
