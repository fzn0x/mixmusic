package room

// The addresses a link, or a host, tells (#562): an invite link's, a
// room's that moves, and where it moved. Anyone may make up a link, and a
// host names what its guests dial, so a guest never connects there to
// this computer itself, nor to an address of no one, link-local,
// multicast, broadcast or unspecified, however the address's name
// resolves: the check runs as each connection starts, on the address it
// resolved to, so that a name that resolves elsewhere the next time (DNS
// rebinding) gets nowhere either. What discovery found on the network, or
// the user typed, the app dials as it is.

import (
	"errors"
	"net"
	"net/netip"
	"syscall"
)

// ErrFarAddr: an address a link or a host told is of this computer, or of
// no one.
var ErrFarAddr = errors.New("room: the address is of this computer, or of no one")

// FarDialer is the dialer of the addresses links and hosts tell.
func FarDialer() *net.Dialer { return &net.Dialer{Control: farControl} }

// farControl refuses a connection to address, as it was resolved to,
// unless FarOK.
func farControl(_, address string, _ syscall.RawConn) error {
	ap, err := netip.ParseAddrPort(address)
	if err != nil || !FarOK(ap.Addr()) {
		return ErrFarAddr
	}
	return nil
}

// FarOK reports whether ip may be dialled at an address a link or a host
// told: not this computer's loopback, nor a link-local, multicast,
// broadcast or unspecified address.
func FarOK(ip netip.Addr) bool {
	ip = ip.Unmap()
	switch {
	case !ip.IsValid(), ip.IsUnspecified(), ip.IsLoopback(), ip.IsLinkLocalUnicast(), ip.IsMulticast(),
		ip == netip.AddrFrom4([4]byte{255, 255, 255, 255}):
		return false
	}
	return true
}
