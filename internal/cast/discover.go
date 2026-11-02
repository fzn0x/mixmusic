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

	"golang.org/x/net/dns/dnsmessage"
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
	send := func(questions []dnsmessage.Question) error {
		q, err := query(id, questions)
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
	ptr := dnsmessage.Question{Name: mustName(service), Type: dnsmessage.TypePTR, Class: dnsmessage.ClassINET}
	if err := send([]dnsmessage.Question{ptr}); err != nil {
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

// query returns an mDNS query of questions.
func query(id uint16, questions []dnsmessage.Question) ([]byte, error) {
	b := dnsmessage.NewBuilder(nil, dnsmessage.Header{ID: id})
	b.EnableCompression()
	if err := b.StartQuestions(); err != nil {
		return nil, err
	}
	for _, q := range questions {
		if err := b.Question(q); err != nil {
			return nil, err
		}
	}
	return b.Finish()
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

func mustName(s string) dnsmessage.Name {
	n, err := dnsmessage.NewName(s)
	if err != nil {
		panic(err)
	}
	return n
}

// findings gather what the answers of a browse tell, by the names of the
// records: the instances of the service, where each is and what it says of
// itself, and the addresses of the hosts.
type findings struct {
	instances []string
	srv       map[string]dnsmessage.SRVResource
	txt       map[string][]string
	hosts     map[string]net.IP
	// from is the address each instance's answer came from, which is the
	// device's own when no record tells its host's address.
	from map[string]net.IP
}

func newFindings() *findings {
	return &findings{srv: map[string]dnsmessage.SRVResource{}, txt: map[string][]string{}, hosts: map[string]net.IP{},
		from: map[string]net.IP{}}
}

// read takes in an answer from the address from. What it can't read it
// leaves out.
func (f *findings) read(msg []byte, from net.IP) {
	var p dnsmessage.Parser
	h, err := p.Start(msg)
	if err != nil || !h.Response {
		return
	}
	if err := p.SkipAllQuestions(); err != nil {
		return
	}
	var named []string
	take := func(rh dnsmessage.ResourceHeader) error {
		name := strings.ToLower(rh.Name.String())
		switch rh.Type {
		case dnsmessage.TypePTR:
			r, err := p.PTRResource()
			if err != nil {
				return err
			}
			if name == service {
				inst := strings.ToLower(r.PTR.String())
				if !slices.Contains(f.instances, inst) {
					f.instances = append(f.instances, inst)
				}
				named = append(named, inst)
			}
		case dnsmessage.TypeSRV:
			r, err := p.SRVResource()
			if err != nil {
				return err
			}
			f.srv[name] = r
			named = append(named, name)
		case dnsmessage.TypeTXT:
			r, err := p.TXTResource()
			if err != nil {
				return err
			}
			f.txt[name] = r.TXT
			named = append(named, name)
		case dnsmessage.TypeA:
			r, err := p.AResource()
			if err != nil {
				return err
			}
			f.hosts[name] = net.IP(r.A[:])
		default:
			return p.SkipAnswer()
		}
		return nil
	}
	// The answers, then what comes with them. A device may put its records
	// in either.
	for {
		rh, err := p.AnswerHeader()
		if err != nil {
			break
		}
		if take(rh) != nil {
			return
		}
	}
	if p.SkipAllAuthorities() == nil {
		for {
			rh, err := p.AdditionalHeader()
			if err != nil {
				break
			}
			if rh.Type == dnsmessage.TypePTR || rh.Type == dnsmessage.TypeSRV || rh.Type == dnsmessage.TypeTXT || rh.Type == dnsmessage.TypeA {
				if take(rh) != nil {
					break
				}
			} else if p.SkipAdditional() != nil {
				break
			}
		}
	}
	for _, n := range named {
		if f.from[n] == nil {
			f.from[n] = from
		}
	}
}

// missing returns the questions that would tell the rest of the instances
// found in part: where they are, and what they say of themselves.
func (f *findings) missing() []dnsmessage.Question {
	var qs []dnsmessage.Question
	ask := func(name string, t dnsmessage.Type) {
		if n, err := dnsmessage.NewName(name); err == nil {
			qs = append(qs, dnsmessage.Question{Name: n, Type: t, Class: dnsmessage.ClassINET})
		}
	}
	for _, inst := range f.instances {
		srv, ok := f.srv[inst]
		if !ok {
			ask(inst, dnsmessage.TypeSRV)
		}
		if _, ok := f.txt[inst]; !ok {
			ask(inst, dnsmessage.TypeTXT)
		}
		if ok && f.hosts[strings.ToLower(srv.Target.String())] == nil {
			ask(srv.Target.String(), dnsmessage.TypeA)
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
		host := f.hosts[strings.ToLower(srv.Target.String())]
		if host == nil {
			host = f.from[inst]
		}
		if host == nil {
			continue
		}
		txt := txtValues(f.txt[inst])
		d := Device{ID: txt["id"], Name: txt["fn"], Model: txt["md"], Host: host.String(), Port: int(srv.Port)}
		if d.Model == groupModel {
			continue
		}
		if d.ID == "" {
			d.ID = inst
		}
		if d.Name == "" {
			d.Name, _, _ = strings.Cut(inst, ".")
		}
		if i := slices.IndexFunc(out, func(o Device) bool { return o.ID == d.ID }); i >= 0 {
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
