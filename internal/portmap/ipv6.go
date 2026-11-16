package portmap

import (
	"net"
	"net/netip"
)

// PublicIPv6 returns the IPv6 addresses of this computer that the internet
// may reach: global unicast ones of the networks that are up, but unique
// local ones (fc00::/7), the documentation's (2001:db8::/32), Teredo's
// (2001::/32) and 6to4's (2002::/16), which guests seldom reach, and those
// made of the network card's hardware address (EUI-64), which an invite
// link would tell: temporary and privacy addresses alone.
func PublicIPv6() []netip.Addr {
	ifs, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var out []netip.Addr
	for _, in := range ifs {
		if in.Flags&net.FlagUp == 0 || in.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := in.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipn, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip, ok := netip.AddrFromSlice(ipn.IP)
			if ok && reachableIPv6(ip) && !eui64(ip) {
				out = append(out, ip)
			}
		}
	}
	return out
}

// reachableIPv6 reports whether ip is an IPv6 address that the internet
// may reach.
func reachableIPv6(ip netip.Addr) bool {
	if !ip.Is6() || ip.Is4In6() || !ip.IsGlobalUnicast() || ip.IsPrivate() {
		return false
	}
	for _, p := range []string{"2001:db8::/32", "2001::/32", "2002::/16"} {
		if netip.MustParsePrefix(p).Contains(ip) {
			return false
		}
	}
	return true
}

// eui64 reports whether ip's interface identifier is made of a hardware
// address, as ff:fe in its middle tells (RFC 4291, appendix A).
func eui64(ip netip.Addr) bool {
	a := ip.As16()
	return a[11] == 0xff && a[12] == 0xfe
}
