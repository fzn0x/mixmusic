package room

// What is near (#562): the addresses of this computer's own networks, as
// against the internet's. One idea of it serves the room's budgets of wrong
// codes (admit.go), the label of the knocks (letin.go), and the app's room
// of the network alone, which takes connections of this computer's
// networks alone while friends outside it may not join: a private,
// loopback or link-local address, or one of the networks of its
// interfaces, as a network of public addresses is.

import (
	"net"
	"net/netip"
	"sync"
	"time"
)

// NearbyOf reports whether ip is of this computer's own networks, as nets,
// its interfaces' (InterfaceNets), tell them: private, loopback or
// link-local, or of one of nets.
func NearbyOf(ip netip.Addr, nets []netip.Prefix) bool {
	ip = ip.Unmap().WithZone("")
	if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return true
	}
	for _, p := range nets {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

// InterfaceNets are the networks of this computer's interfaces, but those
// of one address alone, as a VPN's of /32, which tell nothing of who else
// is on them.
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
		if !ok || ones == bits || bits == 0 {
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
