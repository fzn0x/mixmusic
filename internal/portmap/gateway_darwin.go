package portmap

import (
	"net/netip"
	"syscall"
)

// systemGateways reads a dump of macOS's routing table, by sysctl.
func systemGateways() ([]netip.Addr, error) {
	b, err := syscall.RouteRIB(syscall.NET_RT_DUMP, 0)
	if err != nil {
		return nil, err
	}
	return gatewaysOf(routeDump(b)), nil
}
