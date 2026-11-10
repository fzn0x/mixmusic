package dnssd

import (
	"errors"
	"net"
	"strings"
)

// The times records hold, in seconds, as RFC 6762, 10, has them: those
// that name a host, or are a host's, for two minutes, others for 75; and
// answers to a one-shot querier for ten seconds at most.
const (
	hostTTL   = 120
	otherTTL  = 75 * 60
	legacyTTL = 10
)

// servicesName is the name that lists the services of a network
// (RFC 6763, 9).
const servicesName = "_services._dns-sd._udp.local."

// A Responder answers the questions about one instance of a service: its
// PTR, SRV, TXT and its host's A record. It answers a one-shot querier
// directly, and those that ask from mDNS's port in mDNS's group, or
// directly where they ask so.
type Responder struct {
	// Service is the service's type, as "_mixmusic._tcp.local.", and
	// Instance the instance's name, as "Desktop".
	Service, Instance string
	// Host is the name of the instance's host, as "desktop.local.": one
	// made of Instance when empty.
	Host string
	// Port is the port the service listens on, and Text its TXT record's
	// strings, as "id=…".
	Port int
	Text []string
	// AddrFor returns the computer's IPv4 address that the asker at from
	// reaches, or nil for none: the address of the network from is on, or
	// else the first network's, when nil. Tests give their own.
	AddrFor func(from net.IP) net.IP
}

// Serve answers the questions conn gets until it closes, and returns nil
// then.
func (r *Responder) Serve(conn net.PacketConn) error {
	buf := make([]byte, 9000)
	for {
		n, from, err := conn.ReadFrom(buf)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				continue
			}
			// Windows tells, in the next read, of an answer the asker's
			// port refused (WSAECONNRESET).
			if isReset(err) {
				continue
			}
			return err
		}
		addr, ok := from.(*net.UDPAddr)
		if !ok {
			continue
		}
		reply, to := r.answer(buf[:n], addr)
		if reply != nil {
			_, _ = conn.WriteTo(reply, to)
		}
	}
}

// answer returns the answer to the query msg from, and where it goes, or
// nil when msg asks nothing the responder answers.
func (r *Responder) answer(msg []byte, from *net.UDPAddr) ([]byte, net.Addr) {
	m, err := parse(msg)
	if err != nil || m.response || len(m.questions) == 0 {
		return nil, nil
	}
	// A one-shot querier asks from a port of its own, and is answered
	// there, with its ID and questions, and short times (RFC 6762, 6.7).
	legacy := from.Port != mdnsGroup.Port
	direct := legacy
	var answers, extra []record
	var asked []question
	have := func(rs []record, r record) bool {
		for _, o := range rs {
			if o.name == r.name && o.rtype == r.rtype {
				return true
			}
		}
		return false
	}
	add := func(to *[]record, recs ...record) {
		for _, rec := range recs {
			if !have(answers, rec) && !have(extra, rec) {
				*to = append(*to, rec)
			}
		}
	}
	addr := r.addrFor(from.IP)
	service, instance, host := strings.ToLower(r.Service), strings.ToLower(r.instanceName()), strings.ToLower(r.hostName())
	for _, q := range m.questions {
		all := q.qtype == typeANY
		var got []record
		switch q.name {
		case service:
			if q.qtype == typePTR || all {
				got = append(got, r.ptr())
				add(&extra, r.srv(), r.txt())
				if addr != nil {
					add(&extra, r.a(addr))
				}
			}
		case instance:
			if q.qtype == typeSRV || all {
				got = append(got, r.srv())
				if addr != nil {
					add(&extra, r.a(addr))
				}
			}
			if q.qtype == typeTXT || all {
				got = append(got, r.txt())
			}
		case host:
			if (q.qtype == typeA || all) && addr != nil {
				got = append(got, r.a(addr))
			}
		case servicesName:
			if q.qtype == typePTR || all {
				got = append(got, record{name: servicesName, rtype: typePTR, ttl: otherTTL, target: r.Service})
			}
		}
		if len(got) == 0 {
			continue
		}
		add(&answers, got...)
		asked = append(asked, question{name: q.name, qtype: q.qtype})
		if q.unicast {
			direct = true
		}
	}
	if len(answers) == 0 {
		return nil, nil
	}
	id := uint16(0)
	if legacy {
		id = m.id
		for _, recs := range [][]record{answers, extra} {
			for i := range recs {
				recs[i].ttl = min(recs[i].ttl, legacyTTL)
				recs[i].unique = false
			}
		}
	} else {
		asked = nil
	}
	reply, err := appendResponse(nil, id, asked, answers, extra)
	if err != nil {
		return nil, nil
	}
	if direct {
		return reply, from
	}
	return reply, mdnsGroup
}

// instanceName is the full name of the instance, as
// "Desktop._mixmusic._tcp.local.".
func (r *Responder) instanceName() string { return escapeLabel(r.Instance) + "." + r.Service }

// hostName is the name of the instance's host.
func (r *Responder) hostName() string {
	if r.Host != "" {
		return r.Host
	}
	return HostLabel(r.Instance) + ".local."
}

func (r *Responder) ptr() record {
	return record{name: r.Service, rtype: typePTR, ttl: otherTTL, target: r.instanceName()}
}

func (r *Responder) srv() record {
	return record{name: r.instanceName(), rtype: typeSRV, ttl: hostTTL, unique: true, target: r.hostName(), port: uint16(r.Port)}
}

func (r *Responder) txt() record {
	return record{name: r.instanceName(), rtype: typeTXT, ttl: otherTTL, unique: true, txt: r.Text}
}

func (r *Responder) a(ip net.IP) record {
	return record{name: r.hostName(), rtype: typeA, ttl: hostTTL, unique: true, ip: ip}
}

// addrFor is the computer's address the asker at from reaches.
func (r *Responder) addrFor(from net.IP) net.IP {
	if r.AddrFor != nil {
		return r.AddrFor(from)
	}
	nets := multicastNetworks()
	for _, n := range nets {
		if n.net.Contains(from) {
			return n.ip
		}
	}
	if len(nets) > 0 {
		return nets[0].ip
	}
	return nil
}

// HostLabel makes a host's label of a name, as "Atka's MacBook Pro"
// gives "Atkas-MacBook-Pro": letters, digits and hyphens.
func HostLabel(name string) string {
	var sb strings.Builder
	dash := false
	for _, c := range name {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
			if dash && sb.Len() > 0 {
				sb.WriteByte('-')
			}
			dash = false
			sb.WriteRune(c)
		case c == '\'' || c == '’':
		default:
			dash = true
		}
		if sb.Len() >= 60 {
			break
		}
	}
	if sb.Len() == 0 {
		return "mixmusic"
	}
	return sb.String()
}

// ListenMulticast listens on mDNS's port in mDNS's group, on each of the
// computer's networks that takes multicast, beside the system's own
// responder, which shares the port. It listens on the network: tests
// serve on a port of 127.0.0.1 instead.
func ListenMulticast() (*net.UDPConn, error) {
	conn, err := net.ListenMulticastUDP("udp4", nil, mdnsGroup)
	if err != nil {
		return nil, err
	}
	// The group is joined on the network the system chose; it is joined
	// on the others too, where it wasn't.
	for _, n := range multicastNetworks() {
		_ = joinGroup(conn, n.ip)
	}
	return conn, nil
}
