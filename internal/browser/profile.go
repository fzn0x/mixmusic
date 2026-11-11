package browser

import (
	"os"
	"path/filepath"
	"time"
)

// profilePrefix begins the names of the folders of the profiles made for
// a sign-in.
const profilePrefix = "mixmusic-signin-"

// home is the user's home folder. It is a variable for tests.
var home = os.UserHomeDir

// profileRoot returns the folder where the profile of b goes: the
// system's temporary folder, which the sandbox of a snap doesn't see, or
// the snap's own folder in ~/snap.
func profileRoot(b Browser) (string, error) {
	if b.Snap == "" {
		return os.TempDir(), nil
	}
	h, err := home()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, "snap", b.Snap, "common"), nil
}

// newProfile makes the folder of a new profile for b, readable by the user
// alone.
func newProfile(b Browser) (string, error) {
	root, err := profileRoot(b)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", err
	}
	return os.MkdirTemp(root, profilePrefix+"*")
}

// removeProfile deletes the folder of a profile. On Windows, a process of
// the browser that just ended may hold a file of it a moment longer, so it
// tries again for a few seconds; what's left then goes at the next start
// (RemoveLeftovers).
func removeProfile(dir string) {
	deadline := time.Now().Add(5 * time.Second)
	for os.RemoveAll(dir) != nil && time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
	}
}

// RemoveLeftovers deletes the profiles a MixMusic that stopped while the
// user signed in left: in the temporary folder, and in the snaps' folders.
func RemoveLeftovers() {
	globs := []string{filepath.Join(os.TempDir(), profilePrefix+"*")}
	if h, err := home(); err == nil {
		globs = append(globs, filepath.Join(h, "snap", "*", "common", profilePrefix+"*"))
	}
	for _, g := range globs {
		left, _ := filepath.Glob(g)
		for _, dir := range left {
			os.RemoveAll(dir)
		}
	}
}
