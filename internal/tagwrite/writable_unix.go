//go:build unix

package tagwrite

import (
	"errors"
	"io/fs"
	"syscall"
)

// wOK and xOK ask access(2) whether a file can be written, and a folder's
// files reached.
const (
	wOK = 2
	xOK = 1
)

// writableFile tells whether this user may write the file, as its
// permissions and a read-only file system allow.
func writableFile(path string, info fs.FileInfo) error {
	if err := syscall.Access(path, wOK); err != nil {
		if errors.Is(err, syscall.EACCES) || errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EROFS) {
			return ErrReadOnly
		}
		return err
	}
	return nil
}

// writableDir tells whether this user may make a file in dir, as the safe
// write makes the new one beside the old.
func writableDir(dir string) error {
	if err := syscall.Access(dir, wOK|xOK); err != nil {
		if errors.Is(err, syscall.EACCES) || errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EROFS) {
			return ErrFolderReadOnly
		}
		return err
	}
	return nil
}
