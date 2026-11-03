//go:build !unix && !windows

package cast

import "net"

// setMulticastInterface leaves the network multicast leaves by to the
// system, which routes it.
func setMulticastInterface(*net.UDPConn, net.IP) error { return nil }
