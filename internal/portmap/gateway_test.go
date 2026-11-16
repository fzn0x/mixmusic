package portmap

import (
	"encoding/binary"
	"net/netip"
	"slices"
	"testing"
)

// TestProcRoute reads Linux's /proc/net/route, of a computer of little
// endian bytes: the default routes' gateways, lowest metric first.
func TestProcRoute(t *testing.T) {
	const table = "Iface\tDestination\tGateway \tFlags\tRefCnt\tUse\tMetric\tMask\t\tMTU\tWindow\tIRTT\n" +
		"wlan0\t00000000\tFE01A8C0\t0003\t0\t0\t600\t00000000\t0\t0\t0\n" +
		"eth0\t0002A8C0\t00000000\t0001\t0\t0\t100\t00FFFFFF\t0\t0\t0\n" +
		"eth0\t00000000\t0102A8C0\t0003\t0\t0\t100\t00000000\t0\t0\t0\n" +
		"tun0\t00000000\t00000000\t0001\t0\t0\t50\t00000000\t0\t0\t0\n"
	got := gatewaysOf(procRoute([]byte(table), binary.LittleEndian))
	want := []netip.Addr{netip.MustParseAddr("192.168.2.1"), netip.MustParseAddr("192.168.1.254")}
	if !slices.Equal(got, want) {
		t.Fatalf("gateways %v, want %v", got, want)
	}
}

// TestProcIPv6Route reads Linux's /proc/net/ipv6_route: the default
// route's gateway, a link-local address, of its interface.
func TestProcIPv6Route(t *testing.T) {
	const table = "fe800000000000000000000000000000 40 00000000000000000000000000000000 00 00000000000000000000000000000000 00000100 00000001 00000000 00000001 eth0\n" +
		"00000000000000000000000000000000 00 00000000000000000000000000000000 00 fe800000000000000000000000000001 00000400 00000001 00000000 00000003 eth0\n" +
		"00000000000000000000000000000000 00 00000000000000000000000000000000 00 00000000000000000000000000000000 ffffffff 00000001 00000000 00200200 lo\n"
	got := gatewaysOf(procIPv6Route([]byte(table)))
	if want := []netip.Addr{netip.MustParseAddr("fe80::1%eth0")}; !slices.Equal(got, want) {
		t.Fatalf("gateways %v, want %v", got, want)
	}
}

// TestForwardTable2 reads a table of Windows's GetIpForwardTable2: the
// default routes of IPv4 and IPv6, but a route of a network.
func TestForwardTable2(t *testing.T) {
	le := binary.LittleEndian
	b := make([]byte, rowsAt+3*rowSize)
	le.PutUint32(b, 3)
	row := func(i int) []byte { return b[rowsAt+i*rowSize:] }
	// A default route of IPv4, through 192.168.1.1, of metric 25.
	r := row(0)
	le.PutUint16(r[rowPrefix:], afInet)
	le.PutUint16(r[rowNextHop:], afInet)
	copy(r[rowNextHop+4:], []byte{192, 168, 1, 1})
	le.PutUint32(r[rowMetric:], 25)
	// One of IPv6, through fe80::1 of the interface 7.
	r = row(1)
	le.PutUint32(r[rowIndex:], 7)
	le.PutUint16(r[rowPrefix:], afInetWin6)
	le.PutUint16(r[rowNextHop:], afInetWin6)
	a := netip.MustParseAddr("fe80::1").As16()
	copy(r[rowNextHop+8:], a[:])
	// 192.168.1.0/24, on the link.
	r = row(2)
	le.PutUint16(r[rowPrefix:], afInet)
	copy(r[rowPrefix+4:], []byte{192, 168, 1, 0})
	r[rowPrefixLen] = 24
	le.PutUint16(r[rowNextHop:], afInet)
	got := gatewaysOf(forwardTable2(b, 3))
	want := []netip.Addr{netip.MustParseAddr("192.168.1.1"), netip.MustParseAddr("fe80::1%7")}
	if !slices.Equal(got, want) {
		t.Fatalf("gateways %v, want %v", got, want)
	}
}

// routeMsg is a routing message of macOS, of the flags flags, the
// interface index, and the sockaddrs of its destination, gateway and mask.
func routeMsg(flags uint32, index uint16, sas ...[]byte) []byte {
	le := binary.LittleEndian
	b := make([]byte, darwinHdrSize)
	b[2] = rtmVersion
	le.PutUint16(b[4:], index)
	le.PutUint32(b[8:], flags)
	le.PutUint32(b[12:], rtaDst|rtaGateway|rtaNetmask)
	for _, sa := range sas {
		b = append(b, sa...)
		for len(sa)%4 != 0 || len(sa) == 0 {
			b = append(b, 0)
			sa = append(sa, 0)
		}
	}
	le.PutUint16(b, uint16(len(b)))
	return b
}

func sin(a ...byte) []byte { return append([]byte{16, afInet, 0, 0}, append(a, make([]byte, 8)...)...) }

func sin6(a netip.Addr) []byte {
	b := make([]byte, 28)
	b[0], b[1] = 28, darwinInet6
	a16 := a.As16()
	copy(b[8:], a16[:])
	return b
}

// TestRouteDump reads a dump of macOS's routing table: its default routes,
// of IPv4 and IPv6, whose link-local gateway carries its interface, but a
// route of a network, half the internet, as a VPN takes, and the default
// route of one tunnel alone, as each of a VPN's has.
func TestRouteDump(t *testing.T) {
	var dump []byte
	dump = append(dump, routeMsg(rtfUp|rtfGateway, 4, sin(0, 0, 0, 0), sin(192, 168, 1, 1), nil)...)
	dump = append(dump, routeMsg(rtfUp, 4, sin(192, 168, 1, 0), append([]byte{20, 18, 4, 0}, make([]byte, 16)...), []byte{7, 0, 0, 0, 255, 255, 255})...)
	dump = append(dump, routeMsg(rtfUp|rtfGateway, 9, sin(0, 0, 0, 0), sin(10, 8, 0, 1), []byte{5, 0, 0, 0, 128})...)
	dump = append(dump, routeMsg(rtfUp|rtfGateway, 4, sin6(netip.IPv6Unspecified()), sin6(netip.MustParseAddr("fe80:4::1")), nil)...)
	dump = append(dump, routeMsg(rtfUp|rtfGateway|rtfIfscope, 12, sin6(netip.IPv6Unspecified()), sin6(netip.MustParseAddr("fe80:c::")), nil)...)
	dump = append(dump, routeMsg(rtfUp|rtfGateway|rtfIfscope, 4, sin(0, 0, 0, 0), sin(192, 168, 1, 1), nil)...)
	got := gatewaysOf(routeDump(dump))
	want := []netip.Addr{netip.MustParseAddr("192.168.1.1"), netip.MustParseAddr("fe80::1%4")}
	if !slices.Equal(got, want) {
		t.Fatalf("gateways %v, want %v", got, want)
	}
}

// TestDefaultGateways reads this computer's table of routes: each gateway
// it tells is an address.
func TestDefaultGateways(t *testing.T) {
	gws, err := DefaultGateways()
	if err != nil {
		t.Skipf("this system's gateways aren't read: %v", err)
	}
	for _, gw := range gws {
		if !gw.IsValid() || gw.IsUnspecified() {
			t.Errorf("gateway %v", gw)
		}
	}
	t.Logf("gateways %v", gws)
}
