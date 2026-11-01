//go:build !windows

package downloads

import (
	"errors"
	"syscall"
)

// diskFull reports whether err tells that the disk is full, or the user's
// quota of it.
func diskFull(err error) bool {
	return errors.Is(err, syscall.ENOSPC) || errors.Is(err, syscall.EDQUOT)
}
