package remote

// Admission: what the server takes of the computers it doesn't know as
// paired (#736). Until its TLS handshake ends, no one knows a connection's
// key; after it, until Host.Allowed tells that its key is paired, the
// computer may only pair, or be refused. So the server holds a few such
// connections at once, of all and of each source, as rooms do, and a new
// one takes the place of the oldest: a computer that opens many takes the
// places of its own, not those of a paired computer on its way in. A
// connection gives its place back once its key is paired, or as it becomes
// the one pairing that runs, whose place no one takes; the others that ask
// to pair meanwhile are refused as busy, as before.
//
// Rooms' admission (internal/room's admit.go) isn't shared, only copied:
// room imports this package, so sharing it would move it here, with the
// codes a room counts and its sources of the internet, the /56 of an IPv6
// address, which remote control, on one network, has no use for.

import (
	"net"
	"net/netip"
)

// maxUnproven bounds the connections not known as paired, those whose TLS
// handshake runs and those whose key isn't paired, and maxUnprovenFrom
// those of one source.
const (
	maxUnproven     = 32
	maxUnprovenFrom = 4
)

// sourceOf is the source of the address addr ("host:port"): its IPv4
// address, or the /64 of its IPv6 address, the network's, within which a
// computer picks addresses as it likes; or a loopback address whole.
func sourceOf(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return host
	}
	ip = ip.Unmap().WithZone("")
	if ip.Is4() || ip.IsLoopback() {
		return ip.String()
	}
	p, err := ip.Prefix(64)
	if err != nil {
		return ip.String()
	}
	return p.String()
}

// admission holds the places of the connections not known as paired,
// oldest first. Server.mu guards it.
type admission struct {
	unproven []*unproven
}

// unproven is the place of a connection of src not known as paired, which
// close closes.
type unproven struct {
	src   string
	close func()
	gone  bool
}

// letIn takes a connection of src, which close closes, and returns those it
// takes the place of: the oldest of src, when the server holds as many of
// src as it takes, and, when it holds as many as it takes in all, the
// oldest of the source that holds the most, so that sources that each
// flood the server take each other's places. The caller closes them.
func (a *admission) letIn(src string, close func()) (*unproven, []*unproven) {
	var out []*unproven
	n := 0
	for _, u := range a.unproven {
		if u.src == src {
			n++
		}
	}
	if n >= maxUnprovenFrom {
		for _, u := range a.unproven {
			if u.src == src {
				out = append(out, u)
				a.proven(u)
				break
			}
		}
	}
	if len(a.unproven) >= maxUnproven {
		u := a.crowding()
		out = append(out, u)
		a.proven(u)
	}
	u := &unproven{src: src, close: close}
	a.unproven = append(a.unproven, u)
	return u, out
}

// crowding is the oldest connection of the source that holds the most
// places, the oldest of those that hold as many.
func (a *admission) crowding() *unproven {
	held := map[string]int{}
	most := 0
	for _, u := range a.unproven {
		held[u.src]++
		most = max(most, held[u.src])
	}
	for _, u := range a.unproven {
		if held[u.src] == most {
			return u
		}
	}
	return a.unproven[0]
}

// letGo gives back the place of u, as proven does.
func (s *Server) letGo(u *unproven) {
	s.mu.Lock()
	s.adm.proven(u)
	s.mu.Unlock()
}

// proven gives back the place of a connection whose key is paired, or
// which pairs now, or ended, or gave its place to another.
func (a *admission) proven(u *unproven) {
	if u == nil || u.gone {
		return
	}
	u.gone = true
	for i, o := range a.unproven {
		if o == u {
			a.unproven = append(a.unproven[:i:i], a.unproven[i+1:]...)
			return
		}
	}
}
