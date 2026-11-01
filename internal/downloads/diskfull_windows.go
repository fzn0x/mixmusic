package downloads

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
