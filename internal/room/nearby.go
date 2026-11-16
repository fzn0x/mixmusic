package room

// What is near (#562): the addresses of this computer's own networks, as
// against the internet's. One idea of it serves the room's budgets of wrong
// codes (admit.go), the label of the knocks (letin.go), and the app's room
// of the network alone, which takes connections of this computer's
// networks alone while friends outside it may not join: a private,
// loopback or link-local address, one of the networks of its interfaces,
// as a network of public addresses is, and, while one of its interfaces is
// in it, the providers' shared range, 100.64.0.0/10, which VPNs such as
// Tailscale give their computers, each an address of its own.

import (
	"net"
	"net/netip"
	"sync"
	"time"
)

// sharedNet is the range providers share among their customers (RFC 6598),
// which VPNs give their computers too.
var sharedNet = netip.MustParsePrefix("100.64.0.0/10")

// NearbyOf reports whether ip is of this computer's own networks, as nets,
// its interfaces' (InterfaceNets), tell them: private, loopback or
// link-local, of one of nets, or of the providers' shared range while one
// of nets is in it, as a VPN's address is: the others on the VPN reach this
// computer there, and only they do.
func NearbyOf(ip netip.Addr, nets []netip.Prefix) bool {
	ip = ip.Unmap().WithZone("")
	if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return true
	}
	shared := sharedNet.Contains(ip)
	for _, p := range nets {
		if p.Contains(ip) || shared && sharedNet.Contains(p.Addr()) {
			return true
		}
	}
	return false
}

// InterfaceNets are the networks of this computer's interfaces, those of
// one address alone too, as a VPN's of /32: they tell nothing of who else
// is on them, but that this computer is on the VPN.
func InterfaceNets() []netip.Prefix {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	var out []netip.Prefix
	for _, a := range addrs {
		ipn, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		ip, ok := netip.AddrFromSlice(ipn.IP)
		ones, bits := ipn.Mask.Size()
		if !ok || bits == 0 {
			continue
		}
		ip = ip.Unmap()
		if ip.Is4() && bits == 128 {
			ones -= 96
		}
		out = append(out, netip.PrefixFrom(ip, ones).Masked())
	}
	return out
}

// Nearby tells the addresses of this computer's own networks from the
// internet's (NearbyOf), by the networks of its interfaces, which it reads
// again each half minute. Its zero value reads them.
type Nearby struct {
	// Nets, when set, tells this computer's networks in place of its
	// interfaces, as tests do.
	Nets func() []netip.Prefix

	mu   sync.Mutex
	nets []netip.Prefix
	at   time.Time
}

// interfaces is the Nearby of a server that has none of its own.
var interfaces Nearby

// Of reports whether addr, a connection's other end, "host:port" or a host,
// is of this computer's own networks.
func (n *Nearby) Of(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	return NearbyOf(ip, n.networks())
}

// networks are this computer's networks, as read in the last half minute.
func (n *Nearby) networks() []netip.Prefix {
	if n.Nets != nil {
		return n.Nets()
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.at.IsZero() || time.Since(n.at) > 30*time.Second {
		n.nets, n.at = InterfaceNets(), time.Now()
	}
	return n.nets
}
