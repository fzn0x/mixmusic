package portmap

import (
	"encoding/binary"
	"net/netip"
	"os"
)

// systemGateways reads Linux's tables of routes, which /proc tells.
func systemGateways() ([]netip.Addr, error) {
	var routes []route
	b, err := os.ReadFile("/proc/net/route")
	if err == nil {
		routes = procRoute(b, binary.NativeEndian)
	}
	if b6, err6 := os.ReadFile("/proc/net/ipv6_route"); err6 == nil {
		routes = append(routes, procIPv6Route(b6)...)
	}
	return gatewaysOf(routes), err
}
