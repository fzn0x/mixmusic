package cast

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"net"
	"slices"
	"strings"
	"time"
)

// service is the name cast devices announce themselves under by mDNS.
const service = "_googlecast._tcp.local."

// mdnsGroup is the address mDNS queries go to.
var mdnsGroup = &net.UDPAddr{IP: net.IPv4(224, 0, 0, 251), Port: 5353}

// BrowseOptions configure Browse. Its zero value asks the network.
type BrowseOptions struct {
	// Wait is how long Browse waits for answers: a second when 0.
	Wait time.Duration
	// To, when set, is where the queries go, host:port, in place of mDNS's
	// group on each of the computer's networks: a test's responder.
	To string
}

// Browse finds the cast devices on the computer's networks: it asks for
// them by mDNS, and gathers what they answer for opts.Wait. Groups of
// speakers are left out. The devices come sorted by name.
//
// It asks as a one-shot querier does (RFC 6762, 6.7), from a port of its
// own rather than mDNS's, so that the devices answer it alone, and
// directly: it listens on no port anyone else sends to.
func Browse(ctx context.Context, opts BrowseOptions) ([]Device, error) {
	wait := opts.Wait
	if wait <= 0 {
		wait = time.Second
	}
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{})
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()

	id := queryID()
	send := func(qs []question) error {
		q, err := dnsQuery(id, qs)
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
	if err := send([]question{{service, typePTR}}); err != nil {
		return nil, err
	}
	found := newFindings()
	deadline := time.Now().Add(wait)
	// Halfway, the instances whose answers were in part are asked again
	// for the rest: a device may answer the first question alone.
	again := time.Now().Add(wait / 2)
	asked := false
	buf := make([]byte, 9000)
	for {
		until := deadline
		if !asked {
			until = again
		}
		conn.SetReadDeadline(until)
		n, from, err := conn.ReadFromUDP(buf)
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
		found.read(buf[:n], from.IP)
	}
	return found.devices(), nil
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
	ifaces, _ := net.Interfaces()
	sent := false
	var last error
	for _, ifi := range ifaces {
		if ifi.Flags&net.FlagUp == 0 || ifi.Flags&net.FlagMulticast == 0 || ifi.Flags&net.FlagLoopback != 0 {
			continue
		}
		ip := ipv4Of(ifi)
		if ip == nil {
			continue
		}
		if err := setMulticastInterface(conn, ip); err != nil {
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

// ipv4Of returns the first IPv4 address of a network interface, or nil.
func ipv4Of(ifi net.Interface) net.IP {
	addrs, err := ifi.Addrs()
	if err != nil {
		return nil
	}
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok {
			if ip := n.IP.To4(); ip != nil && !ip.IsLinkLocalUnicast() {
				return ip
			}
		}
	}
	return nil
}

// findings gather what the answers of a browse tell, by the names of the
// records: the instances of the service, where each is and what it says of
// itself, and the addresses of the hosts.
type findings struct {
	instances []string
	srv       map[string]record
	txt       map[string][]string
	hosts     map[string]net.IP
	// from is the address each instance's answer came from, which is the
	// device's own when no record tells its host's address.
	from map[string]net.IP
}

func newFindings() *findings {
	return &findings{srv: map[string]record{}, txt: map[string][]string{}, hosts: map[string]net.IP{},
		from: map[string]net.IP{}}
}

// read takes in an answer from the address from: a device may put its
// records among the answers or beside them. What it can't read it leaves
// out.
func (f *findings) read(msg []byte, from net.IP) {
	_, response, records, err := parseDNS(msg)
	if err != nil || !response {
		return
	}
	for _, r := range records {
		name := r.name
		switch r.rtype {
		case typePTR:
			if name != service {
				continue
			}
			name = strings.ToLower(r.target)
			if !slices.Contains(f.instances, name) {
				f.instances = append(f.instances, name)
			}
		case typeSRV:
			r.target = strings.ToLower(r.target)
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
	for _, inst := range f.instances {
		srv, ok := f.srv[inst]
		if !ok {
			qs = append(qs, question{inst, typeSRV})
		}
		if _, ok := f.txt[inst]; !ok {
			qs = append(qs, question{inst, typeTXT})
		}
		if ok && f.hosts[srv.target] == nil {
			qs = append(qs, question{srv.target, typeA})
		}
	}
	return qs
}

// devices returns the devices found whole: each where its service is, by
// its ID, its name and its model, but groups.
func (f *findings) devices() []Device {
	var out []Device
	for _, inst := range f.instances {
		srv, ok := f.srv[inst]
		if !ok {
			continue
		}
		host := f.hosts[srv.target]
		if host == nil {
			host = f.from[inst]
		}
		if host == nil {
			continue
		}
		txt := txtValues(f.txt[inst])
		d := Device{ID: txt["id"], Name: txt["fn"], Model: txt["md"], Host: host.String(), Port: int(srv.port)}
		if d.Model == groupModel {
			continue
		}
		if d.ID == "" {
			d.ID = inst
		}
		if d.Name == "" {
			d.Name = firstLabel(inst)
		}
		if slices.ContainsFunc(out, func(o Device) bool { return o.ID == d.ID }) {
			continue
		}
		out = append(out, d)
	}
	slices.SortFunc(out, func(a, b Device) int { return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)) })
	return out
}

// txtValues reads the key=value strings of a TXT record; keys are told
// apart without their case.
func txtValues(txt []string) map[string]string {
	out := map[string]string{}
	for _, s := range txt {
		k, v, _ := strings.Cut(s, "=")
		k = strings.ToLower(k)
		if _, ok := out[k]; !ok {
			out[k] = v
		}
	}
	return out
}

// firstLabel returns the first label of a name as readName writes it,
// unescaped: the name of a device that tells none.
func firstLabel(name string) string {
	var sb strings.Builder
	for i := 0; i < len(name); i++ {
		switch c := name[i]; {
		case c == '\\' && i+1 < len(name):
			i++
			sb.WriteByte(name[i])
		case c == '.':
			return sb.String()
		default:
			sb.WriteByte(c)
		}
	}
	return sb.String()
}
