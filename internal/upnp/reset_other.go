//go:build !windows

package upnp

// isReset reports whether err only tells that a port refused what was
// sent to it, which Windows alone tells.
func isReset(error) bool { return false }
