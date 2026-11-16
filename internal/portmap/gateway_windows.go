package portmap

import (
	"net/netip"
	"syscall"
	"unsafe"
)

var (
	iphlpapi           = syscall.NewLazyDLL("iphlpapi.dll")
	getIPForwardTable2 = iphlpapi.NewProc("GetIpForwardTable2")
	freeMibTable       = iphlpapi.NewProc("FreeMibTable")
)

// systemGateways reads Windows's table of routes, of IPv4 and IPv6, by
// GetIpForwardTable2.
func systemGateways() ([]netip.Addr, error) {
	if err := getIPForwardTable2.Find(); err != nil {
		return nil, err
	}
	var table unsafe.Pointer
	// AF_UNSPEC: both families.
	if r, _, _ := getIPForwardTable2.Call(0, uintptr(unsafe.Pointer(&table))); r != 0 {
		return nil, syscall.Errno(r)
	}
	defer freeMibTable.Call(uintptr(table))
	n := int(*(*uint32)(table))
	b := unsafe.Slice((*byte)(table), rowsAt+n*rowSize)
	return gatewaysOf(forwardTable2(b, n)), nil
}
