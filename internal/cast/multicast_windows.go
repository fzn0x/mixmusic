package cast

import (
	"net"
	"syscall"
)

// setMulticastInterface has the multicast conn sends leave by the network
// of the address ip, rather than the one the system routes them to.
func setMulticastInterface(conn *net.UDPConn, ip net.IP) error {
	raw, err := conn.SyscallConn()
	if err != nil {
		return err
	}
	var a [4]byte
	copy(a[:], ip.To4())
	var serr error
	if err := raw.Control(func(fd uintptr) {
		serr = syscall.SetsockoptInet4Addr(syscall.Handle(fd), syscall.IPPROTO_IP, syscall.IP_MULTICAST_IF, a)
	}); err != nil {
		return err
	}
	return serr
}
