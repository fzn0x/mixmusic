//go:build !windows

package tagwrite

import (
	"errors"
	"syscall"
)

// diskFull reports whether err tells that the disk is full, or the user's
// quota of it.
func diskFull(err error) bool {
	return errors.Is(err, syscall.ENOSPC) || errors.Is(err, syscall.EDQUOT)
}

// inUse reports whether err tells that another program has the file open
// in a way that keeps it from being replaced: elsewhere than on Windows,
// none does.
func inUse(err error) bool { return false }
