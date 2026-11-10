package dnssd

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"net"
	"slices"
	"strconv"
	"strings"
	"time"
)

// mdnsGroup is the address of mDNS's IPv4 group and port.
var mdnsGroup = &net.UDPAddr{IP: net.IPv4(224, 0, 0, 251), Port: 5353}

// An Instance is an instance of a service, found on the network: its name,
// as "Desktop", the address of its host and its port, and the keys and
// values of its TXT record, whose keys are lowercased.
type Instance struct {
	Name string
	Host string
	Port int
	Text map[string]string
}

// Addr is the instance's host and port, for net.Dial.
func (i Instance) Addr() string { return net.JoinHostPort(i.Host, strconv.Itoa(i.Port)) }

// BrowseOptions configure Browse. Their zero value asks the network.
type BrowseOptions struct {
	// Wait is how long Browse waits for answers: a second when 0.
	Wait time.Duration
	// To, when set, is where the questions go, host:port, in place of
	// mDNS's group on each of the computer's networks: a test's responder.
	To string
	// From, when set, is the address Browse asks from, as
	// "127.0.0.1:0": a test's, which mustn't reach the network. Any
	// address, and a port of the system's choice, when empty.
	From string
}

// Browse finds the instances of service, as "_mixmusic._tcp.local.": it
// asks for them by mDNS, and gathers what they answer for opts.Wait, sorted
// by name.
//
// It asks as a one-shot querier does (RFC 6762, 6.7), from a port of its
// own rather than mDNS's, so that the responders answer it alone, and
// directly: it listens on no port anyone else sends to.
func Browse(ctx context.Context, service string, opts BrowseOptions) ([]Instance, error) {
	service = strings.ToLower(service)
	wait := opts.Wait
	if wait <= 0 {
		wait = time.Second
	}
	from := &net.UDPAddr{}
	if opts.From != "" {
		a, err := net.ResolveUDPAddr("udp4", opts.From)
		if err != nil {
			return nil, err
		}
		from = a
	}
	conn, err := net.ListenUDP("udp4", from)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()

	id := queryID()
	send := func(qs []question) error {
		q, err := appendQuery(nil, id, qs)
		if err != nil {
			return err
		}
		if opts.To != "" {
			to, err := net.ResolveUDPAddr("udp4", opts.To)
			if err != nil {
				return err
			}
			_, err = conn.WriteToUDP(q, to)
			return err
		}
		return sendToGroup(conn, q)
	}
	if err := send([]question{{name: service, qtype: typePTR}}); err != nil {
		return nil, err
	}
	found := newFindings(service)
	deadline := time.Now().Add(wait)
	// Halfway, the instances whose answers were in part are asked again
	// for the rest: a responder may answer the first question alone.
	again := time.Now().Add(wait / 2)
	asked := false
	buf := make([]byte, 9000)
	for {
		until := deadline
		if !asked {
			until = again
		}
		conn.SetReadDeadline(until)
		n, addr, err := conn.ReadFromUDP(buf)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if err != nil {
			var ne net.Error
			if !errors.As(err, &ne) || !ne.Timeout() {
				return nil, err
			}
			if asked || time.Now().After(deadline) {
				break
			}
			asked = true
			if qs := found.missing(); len(qs) > 0 {
				_ = send(qs)
			}
			continue
		}
		found.read(buf[:n], addr.IP)
	}
	return found.instances(), nil
}

// queryID returns an ID for a query, which a one-shot querier's answers
// repeat.
func queryID() uint16 {
	var b [2]byte
	_, _ = rand.Read(b[:])
	return binary.BigEndian.Uint16(b[:])
}

// sendToGroup sends q to mDNS's group on each network of the computer that
// takes multicast, or, where it finds none, as the system routes it.
func sendToGroup(conn *net.UDPConn, q []byte) error {
	sent := false
	var last error
	for _, n := range multicastNetworks() {
		if err := setMulticastInterface(conn, n.ip); err != nil {
			last = err
			continue
		}
		if _, err := conn.WriteToUDP(q, mdnsGroup); err != nil {
			last = err
			continue
		}
		sent = true
	}
	if sent {
		return nil
	}
	if _, err := conn.WriteToUDP(q, mdnsGroup); err != nil {
		if last != nil {
			return last
		}
		return err
	}
	return nil
}

// network is a network of the computer that takes multicast: its
// interface, and the computer's IPv4 address on it, with its mask.
type network struct {
	ifi net.Interface
	ip  net.IP
	net *net.IPNet
}

// multicastNetworks returns the computer's networks that are up and take
// multicast, but loopback, each with its first IPv4 address.
func multicastNetworks() []network {
	ifaces, _ := net.Interfaces()
	var out []network
	for _, ifi := range ifaces {
		if ifi.Flags&net.FlagUp == 0 || ifi.Flags&net.FlagMulticast == 0 || ifi.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := ifi.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			if n, ok := a.(*net.IPNet); ok {
				if ip := n.IP.To4(); ip != nil && !ip.IsLinkLocalUnicast() {
					out = append(out, network{ifi: ifi, ip: ip, net: n})
					break
				}
			}
		}
	}
	return out
}

// findings gather what the answers of a browse tell, by the names of the
// records: the instances of the service, where each is and what it says of
// itself, and the addresses of the hosts.
type findings struct {
	service string
	// names are the instances' names, lowercased, in the order found, and
	// shown the same with the case they were given.
	names []string
	shown map[string]string
	srv   map[string]record
	txt   map[string][]string
	hosts map[string]net.IP
	// from is the address each instance's answer came from, which is the
	// host's own when no record tells its address.
	from map[string]net.IP
}

func newFindings(service string) *findings {
	return &findings{service: service, shown: map[string]string{}, srv: map[string]record{},
		txt: map[string][]string{}, hosts: map[string]net.IP{}, from: map[string]net.IP{}}
}

// read takes in an answer from the address from: a responder may put its
// records among the answers or beside them. What it can't read it leaves
// out.
func (f *findings) read(msg []byte, from net.IP) {
	m, err := parse(msg)
	if err != nil || !m.response {
		return
	}
	for _, r := range m.records {
		name := r.name
		switch r.rtype {
		case typePTR:
			if name != f.service {
				continue
			}
			name = strings.ToLower(r.target)
			if !slices.Contains(f.names, name) {
				f.names = append(f.names, name)
				f.shown[name] = firstLabel(r.target)
			}
		case typeSRV:
			f.srv[name] = r
		case typeTXT:
			f.txt[name] = r.txt
		case typeA:
			f.hosts[name] = r.ip
			continue
		}
		if f.from[name] == nil {
			f.from[name] = from
		}
	}
}

// missing returns the questions that would tell the rest of the instances
// found in part: where they are, and what they say of themselves.
func (f *findings) missing() []question {
	var qs []question
	for _, name := range f.names {
		srv, ok := f.srv[name]
		if !ok {
			qs = append(qs, question{name: name, qtype: typeSRV})
		}
		if _, ok := f.txt[name]; !ok {
			qs = append(qs, question{name: name, qtype: typeTXT})
		}
		if ok && f.hosts[srv.target] == nil {
			qs = append(qs, question{name: srv.target, qtype: typeA})
		}
	}
	return qs
}

// instances returns the instances found whole, each where its service is,
// sorted by name.
func (f *findings) instances() []Instance {
	var out []Instance
	for _, name := range f.names {
		srv, ok := f.srv[name]
		if !ok {
			continue
		}
		host := f.hosts[srv.target]
		if host == nil {
			host = f.from[name]
		}
		if host == nil {
			continue
		}
		out = append(out, Instance{Name: f.shown[name], Host: host.String(), Port: int(srv.port), Text: txtValues(f.txt[name])})
	}
	slices.SortFunc(out, func(a, b Instance) int { return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)) })
	return out
}

// txtValues reads the key=value strings of a TXT record; keys are told
// apart without their case, and the first of a key counts (RFC 6763, 6.4).
func txtValues(txt []string) map[string]string {
	out := map[string]string{}
	for _, s := range txt {
		k, v, _ := strings.Cut(s, "=")
		k = strings.ToLower(k)
		if _, ok := out[k]; !ok && k != "" {
			out[k] = v
		}
	}
	return out
}
