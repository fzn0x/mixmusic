package portmap

// The default gateways, which PCP and NAT-PMP ask: the routers this
// computer sends to the internet through, as the system's table of routes
// tells them. Go has no call of its own for them: Linux tells them in
// /proc/net/route and /proc/net/ipv6_route, Windows by GetIpForwardTable2,
// and macOS by a dump of its routing table (gateway_*.go). What each tells
// is read here, apart from the system, so that tests read it everywhere.

import (
	"bufio"
	"bytes"
	"cmp"
	"encoding/binary"
	"encoding/hex"
	"net/netip"
	"slices"
	"strconv"
	"strings"
)

// DefaultGateways returns the default gateways of this computer, of IPv4
// then of IPv6, as its table of routes tells them.
func DefaultGateways() ([]netip.Addr, error) { return systemGateways() }

// route is a default route: its gateway and its metric.
type route struct {
	gw     netip.Addr
	metric int
}

// gatewaysOf returns the gateways of routes, lowest metric first, each
// once, those of IPv4 first.
func gatewaysOf(routes []route) []netip.Addr {
	slices.SortStableFunc(routes, func(a, b route) int {
		if a.gw.Is4() != b.gw.Is4() {
			if a.gw.Is4() {
				return -1
			}
			return 1
		}
		return cmp.Compare(a.metric, b.metric)
	})
	var out []netip.Addr
	for _, r := range routes {
		if r.gw.IsValid() && !r.gw.IsUnspecified() && !slices.Contains(out, r.gw) {
			out = append(out, r.gw)
		}
	}
	return out
}

// Flags of a route, in Linux's tables and in macOS's.
const (
	rtfUp      = 0x1
	rtfGateway = 0x2
)

// procRoute reads the default gateways in /proc/net/route, of IPv4: each
// line's destination and mask 0, and its gateway, as the system's integer
// in hex, in the order of the bytes of the computer it runs on.
func procRoute(b []byte, order binary.ByteOrder) []route {
	var out []route
	s := bufio.NewScanner(bytes.NewReader(b))
	for s.Scan() {
		f := strings.Fields(s.Text())
		if len(f) < 8 || f[1] != "00000000" || f[7] != "00000000" {
			continue
		}
		flags, err1 := strconv.ParseUint(f[3], 16, 32)
		gw, err2 := strconv.ParseUint(f[2], 16, 32)
		metric, _ := strconv.Atoi(f[6])
		if err1 != nil || err2 != nil || flags&(rtfUp|rtfGateway) != rtfUp|rtfGateway {
			continue
		}
		var a [4]byte
		order.PutUint32(a[:], uint32(gw))
		out = append(out, route{gw: netip.AddrFrom4(a), metric: metric})
	}
	return out
}

// procIPv6Route reads the default gateways in /proc/net/ipv6_route: each
// line's destination ::/0, its next hop, in hex, and its interface, the
// zone of a gateway's link-local address.
func procIPv6Route(b []byte) []route {
	var out []route
	s := bufio.NewScanner(bytes.NewReader(b))
	for s.Scan() {
		f := strings.Fields(s.Text())
		if len(f) < 10 || f[0] != strings.Repeat("0", 32) || f[1] != "00" {
			continue
		}
		hop, err1 := hex.DecodeString(f[4])
		flags, err2 := strconv.ParseUint(f[8], 16, 32)
		metric, _ := strconv.ParseInt(f[5], 16, 64)
		if err1 != nil || err2 != nil || len(hop) != 16 || flags&(rtfUp|rtfGateway) != rtfUp|rtfGateway {
			continue
		}
		gw := netip.AddrFrom16([16]byte(hop))
		if gw.IsLinkLocalUnicast() {
			gw = gw.WithZone(f[9])
		}
		out = append(out, route{gw: gw, metric: int(metric)})
	}
	return out
}

// Windows's MIB_IPFORWARD_TABLE2 (netioapi.h): the count of its rows, then
// the rows, each a MIB_IPFORWARD_ROW2 of rowSize bytes, at 8 bytes
// apart from the count. In a row: the interface's index, at 8; the
// destination's prefix, a SOCKADDR_INET at 12, and its length, at 40; the
// next hop, a SOCKADDR_INET at 44; the metric, at 84.
const (
	rowsAt       = 8
	rowSize      = 104
	afInet       = 2
	afInetWin6   = 23
	rowIndex     = 8
	rowPrefix    = 12
	rowPrefixLen = 40
	rowNextHop   = 44
	rowMetric    = 84
)

// forwardTable2 reads the default routes of a MIB_IPFORWARD_TABLE2 of n
// rows, its bytes.
func forwardTable2(b []byte, n int) []route {
	var out []route
	le := binary.LittleEndian
	for i := range n {
		row := b[rowsAt+i*rowSize:]
		if len(row) < rowSize || row[rowPrefixLen] != 0 {
			continue
		}
		hop := row[rowNextHop:]
		var gw netip.Addr
		switch le.Uint16(hop) {
		case afInet:
			gw = netip.AddrFrom4([4]byte(hop[4:8]))
		case afInetWin6:
			// SOCKADDR_IN6: the port, the flow, the address at 8, the
			// scope at 24.
			gw = netip.AddrFrom16([16]byte(hop[8:24]))
			if gw.IsLinkLocalUnicast() {
				gw = gw.WithZone(strconv.Itoa(int(le.Uint32(row[rowIndex:]))))
			}
		default:
			continue
		}
		out = append(out, route{gw: gw, metric: int(le.Uint32(row[rowMetric:]))})
	}
	return out
}

// macOS's routing messages (net/route.h): rt_msghdr, of hdrSize bytes, its
// length at 0, its version at 2, the interface's index at 4, its flags at
// 8 and which addresses follow at 12; then each address, a sockaddr whose
// length is its first byte, rounded up to 4 bytes, one of 0 taking 4.
const (
	darwinHdrSize = 92
	rtmVersion    = 5
	rtaDst        = 0x1
	rtaGateway    = 0x2
	rtaNetmask    = 0x4
	darwinInet6   = 30
	// rtfIfscope marks a route of one interface alone, as each tunnel
	// of a VPN has a default one: the system's own is the one without.
	rtfIfscope = 0x1000000
)

// routeDump reads the default routes of a dump of macOS's routing table,
// as sysctl's NET_RT_DUMP gives it.
func routeDump(b []byte) []route {
	var out []route
	le := binary.LittleEndian
	for len(b) >= darwinHdrSize {
		n := int(le.Uint16(b))
		if n < darwinHdrSize || n > len(b) {
			break
		}
		msg := b[:n]
		b = b[n:]
		if msg[2] != rtmVersion {
			continue
		}
		flags := le.Uint32(msg[8:])
		addrs := le.Uint32(msg[12:])
		if flags&(rtfUp|rtfGateway) != rtfUp|rtfGateway || flags&rtfIfscope != 0 || addrs&(rtaDst|rtaGateway) != rtaDst|rtaGateway {
			continue
		}
		var dst, gw, mask []byte
		rest := msg[darwinHdrSize:]
		for bit := uint32(1); bit <= rtaNetmask && len(rest) > 0; bit <<= 1 {
			if addrs&bit == 0 {
				continue
			}
			l := int(rest[0])
			step := (l + 3) &^ 3
			if l == 0 {
				step = 4
			}
			if step > len(rest) {
				break
			}
			sa := rest[:max(l, 0)]
			switch bit {
			case rtaDst:
				dst = sa
			case rtaGateway:
				gw = sa
			case rtaNetmask:
				mask = sa
			}
			rest = rest[step:]
		}
		ok, addr := defaultDst(dst, mask), sockaddr(gw)
		if !ok || !addr.IsValid() {
			continue
		}
		if addr.Is6() && addr.IsLinkLocalUnicast() {
			// KAME's link-local address carries its interface in its
			// third and fourth bytes.
			a := addr.As16()
			a[2], a[3] = 0, 0
			addr = netip.AddrFrom16(a).WithZone(strconv.Itoa(int(le.Uint16(msg[4:]))))
		}
		out = append(out, route{gw: addr})
	}
	return out
}

// sockaddr is the address of a sockaddr_in or sockaddr_in6 of macOS.
func sockaddr(sa []byte) netip.Addr {
	switch {
	case len(sa) >= 8 && sa[1] == afInet:
		return netip.AddrFrom4([4]byte(sa[4:8]))
	case len(sa) >= 24 && sa[1] == darwinInet6:
		return netip.AddrFrom16([16]byte(sa[8:24]))
	}
	return netip.Addr{}
}

// defaultDst reports whether a route of the destination dst and the mask
// mask is a default one: of 0.0.0.0 or ::, and a mask of zeros, which the
// kernel writes as short as its last byte that isn't zero, or as nothing.
func defaultDst(dst, mask []byte) bool {
	a := sockaddr(dst)
	if !a.IsValid() || !a.IsUnspecified() {
		return false
	}
	from := 4
	if a.Is6() {
		from = 8
	}
	for i := from; i < len(mask); i++ {
		if mask[i] != 0 {
			return false
		}
	}
	return true
}
