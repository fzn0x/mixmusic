package app

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLocalOpenLinkedFolder plays the music of a dropped folder that is a
// link, as of the folder it points to.
func TestLocalOpenLinkedFolder(t *testing.T) {
	a, music := localApp(t)
	link := filepath.Join(t.TempDir(), "linked-music")
	if err := os.Symlink(music, link); err != nil {
		t.Skipf("this user can't make a symbolic link here (Windows needs Developer Mode or an administrator): %v", err)
	}
	got := (&LocalFiles{a}).Open([]string{link})
	if len(got.Tracks) != 2 {
		t.Errorf("opened %+v, want the 2 tracks of the folder it links to", got)
	}
}
