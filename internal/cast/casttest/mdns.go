package casttest

import (
	"encoding/binary"
	"errors"
	"net"
	"strings"
	"sync"
	"testing"
)

// Entry is a cast device as the responder announces it.
type Entry struct {
	ID, Name, Model string
	// Host is the address of the device's A record: 127.0.0.1 when "",
	// none when "-", which leaves the device's address to the answer's.
	Host string
	Port int
}

// Responder answers mDNS queries for cast devices at a loopback address,
// as devices answer one-shot queriers: directly, to the query's address,
// with its ID and its questions. Its DNS is its own, apart from package
// cast's.
type Responder struct {
	conn *net.UDPConn

	mu      sync.Mutex
	entries []Entry
	// sparse has it answer only what each question asks, with nothing
	// beside it.
	sparse bool
	// queries counts the queries it got.
	queries int
}

// NewResponder starts a responder of entries until the test ends.
func NewResponder(t testing.TB, entries ...Entry) *Responder {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	r := &Responder{conn: conn, entries: entries}
	go r.serve()
	t.Cleanup(func() { conn.Close() })
	return r
}

// Addr is where the responder takes queries.
func (r *Responder) Addr() string { return r.conn.LocalAddr().String() }

// Sparse has the responder answer only what each question asks: the
// pointers to the devices alone, at first.
func (r *Responder) Sparse() {
	r.mu.Lock()
	r.sparse = true
	r.mu.Unlock()
}

// Queries returns how many queries the responder got.
func (r *Responder) Queries() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.queries
}

// The record types and the class of the answers.
const (
	typeA   = 1
	typePTR = 12
	typeTXT = 16
	typeSRV = 33
	inet    = 1
	// flush is the cache-flush bit, which devices set in the class of
	// their unique records.
	flush = 1 << 15
)

const service = "_googlecast._tcp.local."

func instance(e Entry) string { return "Chromecast-" + e.ID + "." + service }
func hostOf(e Entry) string   { return e.ID + ".local." }

type dnsQuestion struct {
	name  string
	qtype uint16
}

func (r *Responder) serve() {
	buf := make([]byte, 9000)
	for {
		n, from, err := r.conn.ReadFromUDP(buf)
		if err != nil {
			return
		}
		id, qs, err := parseQuery(buf[:n])
		if err != nil {
			continue
		}
		r.mu.Lock()
		r.queries++
		entries, sparse := append([]Entry(nil), r.entries...), r.sparse
		r.mu.Unlock()
		if msg := answer(id, qs, entries, sparse); msg != nil {
			r.conn.WriteToUDP(msg, from)
		}
	}
}

// parseQuery reads the ID and the questions of a query.
func parseQuery(msg []byte) (uint16, []dnsQuestion, error) {
	bad := errors.New("casttest: a bad query")
	if len(msg) < 12 || msg[2]&0x80 != 0 {
		return 0, nil, bad
	}
	n := int(binary.BigEndian.Uint16(msg[4:]))
	off := 12
	var qs []dnsQuestion
	for range n {
		var labels []string
		for {
			if off >= len(msg) {
				return 0, nil, bad
			}
			l := int(msg[off])
			off++
			if l == 0 {
				break
			}
			if l > 63 || off+l > len(msg) {
				return 0, nil, bad // queries come without pointers
			}
			labels = append(labels, string(msg[off:off+l]))
			off += l
		}
		if off+4 > len(msg) {
			return 0, nil, bad
		}
		qs = append(qs, dnsQuestion{strings.ToLower(strings.Join(labels, ".") + "."), binary.BigEndian.Uint16(msg[off:])})
		off += 4
	}
	return binary.BigEndian.Uint16(msg), qs, nil
}

// rr is a record of an answer, in the wire's form.
type rr struct {
	name  string
	rtype uint16
	data  []byte
}

// answer builds the answer to the questions qs of the query id.
func answer(id uint16, qs []dnsQuestion, entries []Entry, sparse bool) []byte {
	var answers, extra []rr
	for _, q := range qs {
		for _, e := range entries {
			srv := rr{instance(e), typeSRV, append([]byte{0, 0, 0, 0, byte(e.Port >> 8), byte(e.Port)}, name(hostOf(e))...)}
			var txt []byte
			for _, s := range []string{"id=" + e.ID, "cd=0123", "rm=", "ve=05", "md=" + e.Model, "ic=/setup/icon.png",
				"fn=" + e.Name, "ca=4101", "st=0", "rs="} {
				txt = append(append(txt, byte(len(s))), s...)
			}
			ip := net.ParseIP(e.Host).To4()
			if ip == nil {
				ip = net.IPv4(127, 0, 0, 1).To4()
			}
			a := rr{hostOf(e), typeA, []byte(ip)}
			switch {
			case q.qtype == typePTR && q.name == service:
				answers = append(answers, rr{service, typePTR, name(instance(e))})
				if !sparse {
					extra = append(extra, srv, rr{instance(e), typeTXT, txt})
					if e.Host != "-" {
						extra = append(extra, a)
					}
				}
			case q.qtype == typeSRV && q.name == strings.ToLower(instance(e)):
				answers = append(answers, srv)
			case q.qtype == typeTXT && q.name == strings.ToLower(instance(e)):
				answers = append(answers, rr{instance(e), typeTXT, txt})
			case q.qtype == typeA && q.name == strings.ToLower(hostOf(e)) && e.Host != "-":
				answers = append(answers, a)
			}
		}
	}
	if len(answers) == 0 {
		return nil
	}
	msg := binary.BigEndian.AppendUint16(nil, id)
	msg = append(msg, 0x84, 0) // a response, authoritative
	for _, n := range []int{len(qs), len(answers), 0, len(extra)} {
		msg = binary.BigEndian.AppendUint16(msg, uint16(n))
	}
	for _, q := range qs {
		msg = append(msg, name(q.name)...)
		msg = binary.BigEndian.AppendUint16(msg, q.qtype)
		msg = binary.BigEndian.AppendUint16(msg, inet)
	}
	for _, r := range append(answers, extra...) {
		class := uint16(inet)
		if r.rtype != typePTR {
			class |= flush
		}
		msg = append(msg, name(r.name)...)
		msg = binary.BigEndian.AppendUint16(msg, r.rtype)
		msg = binary.BigEndian.AppendUint16(msg, class)
		msg = binary.BigEndian.AppendUint32(msg, 120)
		msg = binary.BigEndian.AppendUint16(msg, uint16(len(r.data)))
		msg = append(msg, r.data...)
	}
	return msg
}

// name writes a name in DNS's labels, without compression.
func name(s string) []byte {
	var out []byte
	for _, l := range strings.Split(strings.TrimSuffix(s, "."), ".") {
		out = append(append(out, byte(len(l))), l...)
	}
	return append(out, 0)
}
