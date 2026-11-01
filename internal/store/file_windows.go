//go:build windows

package store

import (
	"io"
	"io/fs"
	"os"
	"syscall"
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
