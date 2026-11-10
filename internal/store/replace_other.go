//go:build !windows

package store

import "os"

// OpenShared opens the file at path to read it, as os.Open does:
// elsewhere than on Windows, another file renamed over it leaves it
// readable as it was until it's closed.
func OpenShared(path string) (*os.File, error) { return os.Open(path) }

// renameOver renames from over to, in root.
func renameOver(root *os.Root, from, to string) error { return root.Rename(from, to) }

// syncDir puts a rename in dir on the disk, as far as the system lets it.
func syncDir(dir string) {
	if d, err := os.Open(dir); err == nil {
		d.Sync()
		d.Close()
	}
}
