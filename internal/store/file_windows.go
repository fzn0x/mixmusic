//go:build windows

package store

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"syscall"
	"time"
)

// openShared opens path to read it, sharing it with the programs that save
// over it meanwhile, as editors do: by writing it, deleting it, renaming it
// away, or renaming another file over it with POSIX semantics. os.Open
// shares no deletes, and those saves would fail while MixMusic reads the
// file. A rename over it by MoveFileEx, as os.Rename's, fails while the
// file is open at all, which no share mode allows: reads are short.
func openShared(path string) (*os.File, error) {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: path, Err: err}
	}
	h, err := syscall.CreateFile(name, syscall.GENERIC_READ,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: path, Err: err}
	}
	return os.NewFile(uintptr(h), path), nil
}

// ReadFile reads the file at path, as os.ReadFile does, through a handle
// that lets an editor save over the file meanwhile.
func ReadFile(path string) ([]byte, error) {
	f, err := openShared(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

// How long rename tries again, over all, while another program has the
// file open: renameTries tries, renameWait apart.
var (
	renameTries = 6
	renameWait  = 20 * time.Millisecond
)

// The errors of a file that another program has open.
const (
	errorAccessDenied     = syscall.Errno(5)
	errorSharingViolation = syscall.Errno(32)
	errorLockViolation    = syscall.Errno(33)
)

// rename renames from over to. MoveFileEx, which os.Rename calls, can't
// replace a file another program has open, as an editor, a search indexer
// or an antivirus may for a moment: it tries again for about 100 ms.
func rename(from, to string) error {
	var err error
	for try := range renameTries {
		if try > 0 {
			time.Sleep(renameWait)
		}
		if err = os.Rename(from, to); err == nil || !busy(err) {
			return err
		}
	}
	return err
}

// busy tells whether err is that of a file another program has open.
func busy(err error) bool {
	return errors.Is(err, errorAccessDenied) || errors.Is(err, errorSharingViolation) || errors.Is(err, errorLockViolation)
}
