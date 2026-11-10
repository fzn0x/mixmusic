package tagwrite

import (
	"errors"
	"io/fs"
	"os"
)

// writableFile tells whether the file is read-only, as its attribute
// says.
func writableFile(path string, info fs.FileInfo) error {
	if info.Mode().Perm()&0o200 == 0 {
		return ErrReadOnly
	}
	return nil
}

// writableDir tells whether a file can be made in dir, as the safe write
// makes the new one beside the old, by making one and removing it: its
// permissions are lists of who may do what, which only the system reads.
func writableDir(dir string) error {
	f, err := os.CreateTemp(dir, ".mixmusic-*.tmp")
	if err != nil {
		if errors.Is(err, fs.ErrPermission) {
			return ErrFolderReadOnly
		}
		return err
	}
	f.Close()
	os.Remove(f.Name())
	return nil
}
