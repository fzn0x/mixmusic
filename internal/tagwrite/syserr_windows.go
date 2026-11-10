package tagwrite

import (
	"errors"
	"syscall"
)

// diskFull reports whether err tells that the disk is full:
// ERROR_HANDLE_DISK_FULL or ERROR_DISK_FULL on Windows.
func diskFull(err error) bool {
	var errno syscall.Errno
	return errors.As(err, &errno) && (errno == 39 || errno == 112)
}

// inUse reports whether err tells that another program has the file open
// in a way that keeps it from being replaced: ERROR_SHARING_VIOLATION or
// ERROR_LOCK_VIOLATION.
func inUse(err error) bool {
	var errno syscall.Errno
	return errors.As(err, &errno) && (errno == 32 || errno == 33)
}
