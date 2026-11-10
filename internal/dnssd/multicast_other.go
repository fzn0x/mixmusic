//go:build !unix && !windows

package dnssd

import "net"

// setMulticastInterface leaves the network multicast leaves by to the
// system, which routes it.
func setMulticastInterface(*net.UDPConn, net.IP) error { return nil }

// joinGroup leaves the group joined on the network the system chose.
func joinGroup(*net.UDPConn, net.IP) error { return nil }

// isReset reports whether err only tells that a port refused what was
// sent to it, which Windows alone tells.
func isReset(error) bool { return false }
