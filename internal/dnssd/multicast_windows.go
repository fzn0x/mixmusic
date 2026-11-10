package dnssd

import (
	"errors"
	"net"
	"syscall"
)

// setMulticastInterface has the multicast conn sends leave by the network
// of the address ip, rather than the one the system routes them to.
func setMulticastInterface(conn *net.UDPConn, ip net.IP) error {
	return control(conn, func(fd uintptr) error {
		var a [4]byte
		copy(a[:], ip.To4())
		return syscall.SetsockoptInet4Addr(syscall.Handle(fd), syscall.IPPROTO_IP, syscall.IP_MULTICAST_IF, a)
	})
}

// joinGroup joins mDNS's group on the network of the address ip.
func joinGroup(conn *net.UDPConn, ip net.IP) error {
	return control(conn, func(fd uintptr) error {
		m := &syscall.IPMreq{}
		copy(m.Multiaddr[:], mdnsGroup.IP.To4())
		copy(m.Interface[:], ip.To4())
		return syscall.SetsockoptIPMreq(syscall.Handle(fd), syscall.IPPROTO_IP, syscall.IP_ADD_MEMBERSHIP, m)
	})
}

// wsaeconnreset is the error Windows gives a UDP socket's next read once a
// port it sent to refused what it sent.
const wsaeconnreset = syscall.Errno(10054)

// isReset reports whether err is WSAECONNRESET, which tells nothing of the
// socket itself.
func isReset(err error) bool { return errors.Is(err, wsaeconnreset) }

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
