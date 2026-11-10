//go:build unix

package dnssd

import (
	"net"
	"syscall"
)

// setMulticastInterface has the multicast conn sends leave by the network
// of the address ip, rather than the one the system routes them to.
func setMulticastInterface(conn *net.UDPConn, ip net.IP) error {
	return control(conn, func(fd uintptr) error {
		var a [4]byte
		copy(a[:], ip.To4())
		return syscall.SetsockoptInet4Addr(int(fd), syscall.IPPROTO_IP, syscall.IP_MULTICAST_IF, a)
	})
}

// joinGroup joins mDNS's group on the network of the address ip.
func joinGroup(conn *net.UDPConn, ip net.IP) error {
	return control(conn, func(fd uintptr) error {
		m := &syscall.IPMreq{}
		copy(m.Multiaddr[:], mdnsGroup.IP.To4())
		copy(m.Interface[:], ip.To4())
		return syscall.SetsockoptIPMreq(int(fd), syscall.IPPROTO_IP, syscall.IP_ADD_MEMBERSHIP, m)
	})
}

// isReset reports whether err only tells that a port refused what was
// sent to it, which Windows alone tells.
func isReset(error) bool { return false }

func control(conn *net.UDPConn, fn func(fd uintptr) error) error {
	raw, err := conn.SyscallConn()
	if err != nil {
		return err
	}
	var serr error
	if err := raw.Control(func(fd uintptr) { serr = fn(fd) }); err != nil {
		return err
	}
	return serr
}
