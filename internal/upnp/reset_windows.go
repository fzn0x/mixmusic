package upnp

import (
	"errors"
	"syscall"
)

// isReset reports whether err is WSAECONNRESET, which Windows gives a UDP
// socket's next read once a port it sent to refused what it sent: it tells
// nothing of the socket itself.
func isReset(err error) bool { return errors.Is(err, syscall.Errno(10054)) }
