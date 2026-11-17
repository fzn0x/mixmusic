package localfiles

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"
)

// linkTo makes a link at link to dir, skipping the test where the system
// doesn't let this user make one: Windows asks for Developer Mode or an
// administrator for a symbolic link.
func linkTo(t *testing.T, dir, link string) {
	t.Helper()
	if err := os.Symlink(dir, link); err != nil {
		if runtime.GOOS == "windows" || errors.Is(err, fs.ErrPermission) {
			t.Skipf("this user can't make a symbolic link here (Windows needs Developer Mode or an administrator): %v", err)
		}
		t.Fatal(err)
	}
}

// junctionTo makes a directory junction, which needs no privilege.
func junctionTo(t *testing.T, dir, link string) {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Skip("directory junctions are Windows'")
	}
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, dir).CombinedOutput(); err != nil {
		t.Skipf("can't make a junction: %v: %s", err, out)
	}
}

// TestScanLinkedFolder lists the tracks of a music folder that is a link,
// by the link's own path.
func TestScanLinkedFolder(t *testing.T) {
	for name, mk := range map[string]func(*testing.T, string, string){"symlink": linkTo, "junction": junctionTo} {
		t.Run(name, func(t *testing.T) {
			ti := newIndex(t)
			ti.write(t, "Harbour/01.mp3", "Slow Tide|The Harbour Lights|Shore||1", time.Hour)
			ti.write(t, "02.flac", "Low Water|The Harbour Lights|Shore||2", time.Hour)
			link := filepath.Join(t.TempDir(), "linked-music")
			mk(t, ti.dir, link)

			ti.scan(t, link)
			st := ti.Status()
			if len(st.Folders) != 1 || st.Folders[0].Tracks != 2 || st.Folders[0].Problem != "" {
				t.Fatalf("folders = %+v, want the link with 2 tracks", st.Folders)
			}
			if _, ok := ti.Entry(filepath.Join(link, "Harbour", "01.mp3")); !ok {
				t.Errorf("no entry under the link's own path")
			}
			if got, want := heldTitles(ti.Index), []string{"Low Water", "Slow Tide"}; !slices.Equal(got, want) {
				t.Errorf("titles = %v, want %v", got, want)
			}
		})
	}
}

// TestScanLinkedFolderTwice counts a folder once when it's listed by its
// path and by a link to it.
func TestScanLinkedFolderTwice(t *testing.T) {
	ti := newIndex(t)
	ti.write(t, "01.mp3", "Slow Tide|The Harbour Lights|Shore||1", time.Hour)
	link := filepath.Join(t.TempDir(), "again")
	linkTo(t, ti.dir, link)

	ti.scan(t, ti.dir, link)
	st := ti.Status()
	if st.Tracks != 1 {
		t.Errorf("tracks = %d, want the one file once", st.Tracks)
	}
	if len(st.Folders) != 2 || st.Folders[0].Tracks != 1 || st.Folders[1].Tracks != 1 {
		t.Errorf("folders = %+v, want both with 1 track", st.Folders)
	}
}

// TestScanLinkLoop ends where a link in a music folder points to its
// parent: links inside a folder aren't followed.
func TestScanLinkLoop(t *testing.T) {
	ti := newIndex(t)
	ti.write(t, "Harbour/01.mp3", "Slow Tide|The Harbour Lights|Shore||1", time.Hour)
	linkTo(t, ti.dir, filepath.Join(ti.dir, "Harbour", "up"))
	root := filepath.Join(t.TempDir(), "linked-music")
	linkTo(t, ti.dir, root)

	ti.scan(t, root)
	if got := ti.Status().Tracks; got != 1 {
		t.Errorf("tracks = %d, want 1", got)
	}
}

// TestWalkRootPaths tells the paths under the link, the link itself first.
func TestWalkRootPaths(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "l")
	linkTo(t, dir, link)
	var got []string
	if err := WalkRoot(link, func(p string, _ fs.DirEntry, err error) error {
		got = append(got, p)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if want := []string{link, filepath.Join(link, "a")}; !slices.Equal(got, want) {
		t.Errorf("paths = %v, want %v", got, want)
	}
}
