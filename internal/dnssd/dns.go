// Package dnssd finds and announces services on the local network by
// DNS-SD (RFC 6763) over mDNS (RFC 6762), in Go: it writes and reads the
// few DNS records that takes, PTR, SRV, TXT and A, itself.
package dnssd

import (
	"encoding/binary"
	"errors"
	"net"
	"strings"
)

// The DNS record types DNS-SD takes (RFC 1035, RFC 2782), ANY, which asks
// for every type, and the Internet class.
const (
	typeA    = 1
	typePTR  = 12
	typeTXT  = 16
	typeSRV  = 33
	typeANY  = 255
	classIN  = 1
	classTop = 0x8000 // a question's unicast-response bit, a record's cache-flush bit
)

// question is a question of a DNS query: a name, as
// "_mixmusic._tcp.local.", the type of record asked for, and whether the
// asker wants the answer sent to it directly (mDNS's QU bit).
type question struct {
	name    string
	qtype   uint16
	unicast bool
}

// record is a resource record: its name, its type, how long it holds, in
// seconds, and what its type holds. unique records set mDNS's cache-flush
// bit: no other responder answers for their name and type.
type record struct {
	name   string
	rtype  uint16
	ttl    uint32
	unique bool
	// target is a PTR's name, or an SRV's host; port is an SRV's.
	target string
	port   uint16
	txt    []string
	ip     net.IP
}

// message is a DNS message, as far as DNS-SD reads it.
type message struct {
	id        uint16
	response  bool
	questions []question
	// records are those of the answers, the authorities and the
	// additional records, of the types DNS-SD reads.
	records []record
}

var errShort = errors.New("dnssd: a DNS message cut short")

// appendQuery appends a DNS query of id with questions qs, of the Internet
// class, as mDNS takes it.
func appendQuery(b []byte, id uint16, qs []question) ([]byte, error) {
	b = binary.BigEndian.AppendUint16(b, id)
	b = append(b, 0, 0) // a standard query
	b = binary.BigEndian.AppendUint16(b, uint16(len(qs)))
	b = append(b, 0, 0, 0, 0, 0, 0)
	for _, q := range qs {
		var err error
		if b, err = appendQuestion(b, q); err != nil {
			return nil, err
		}
	}
	return b, nil
}

func appendQuestion(b []byte, q question) ([]byte, error) {
	b, err := appendName(b, q.name)
	if err != nil {
		return nil, err
	}
	class := uint16(classIN)
	if q.unicast {
		class |= classTop
	}
	b = binary.BigEndian.AppendUint16(b, q.qtype)
	return binary.BigEndian.AppendUint16(b, class), nil
}

// appendResponse appends an authoritative DNS response of id: the
// questions it answers, which only answers to a one-shot querier repeat,
// its answers, and the additional records that save asking for them.
func appendResponse(b []byte, id uint16, qs []question, answers, extra []record) ([]byte, error) {
	b = binary.BigEndian.AppendUint16(b, id)
	b = append(b, 0x84, 0) // a response, authoritative
	b = binary.BigEndian.AppendUint16(b, uint16(len(qs)))
	b = binary.BigEndian.AppendUint16(b, uint16(len(answers)))
	b = append(b, 0, 0)
	b = binary.BigEndian.AppendUint16(b, uint16(len(extra)))
	var err error
	for _, q := range qs {
		if b, err = appendQuestion(b, q); err != nil {
			return nil, err
		}
	}
	for _, r := range append(answers[:len(answers):len(answers)], extra...) {
		if b, err = appendRecord(b, r); err != nil {
			return nil, err
		}
	}
	return b, nil
}

// appendRecord appends r, whose data its type gives.
func appendRecord(b []byte, r record) ([]byte, error) {
	b, err := appendName(b, r.name)
	if err != nil {
		return nil, err
	}
	class := uint16(classIN)
	if r.unique {
		class |= classTop
	}
	b = binary.BigEndian.AppendUint16(b, r.rtype)
	b = binary.BigEndian.AppendUint16(b, class)
	b = binary.BigEndian.AppendUint32(b, r.ttl)
	at := len(b)
	b = append(b, 0, 0) // the data's length, once written
	switch r.rtype {
	case typePTR:
		b, err = appendName(b, r.target)
	case typeSRV:
		b = append(b, 0, 0, 0, 0) // priority and weight
		b = binary.BigEndian.AppendUint16(b, r.port)
		b, err = appendName(b, r.target)
	case typeTXT:
		if len(r.txt) == 0 {
			b = append(b, 0) // one empty string, as RFC 6763 6.1 has it
		}
		for _, s := range r.txt {
			if len(s) > 255 {
				return nil, errors.New("dnssd: a TXT string is too long")
			}
			b = append(b, byte(len(s)))
			b = append(b, s...)
		}
	case typeA:
		ip := r.ip.To4()
		if ip == nil {
			return nil, errors.New("dnssd: an A record without an IPv4 address")
		}
		b = append(b, ip...)
	default:
		return nil, errors.New("dnssd: a record of a type it doesn't write")
	}
	if err != nil {
		return nil, err
	}
	binary.BigEndian.PutUint16(b[at:], uint16(len(b)-at-2))
	return b, nil
}

// appendName appends name in DNS's labels. A dot within a label is
// escaped, as "\.", and so is a backslash, as readName writes them.
func appendName(b []byte, name string) ([]byte, error) {
	name = strings.TrimSuffix(name, ".")
	var label []byte
	flush := func() error {
		if len(label) == 0 || len(label) > 63 {
			return errors.New("dnssd: a name's label is empty or too long")
		}
		b = append(b, byte(len(label)))
		b = append(b, label...)
		label = label[:0]
		return nil
	}
	for i := 0; i < len(name); i++ {
		switch c := name[i]; {
		case c == '\\' && i+1 < len(name):
			i++
			label = append(label, name[i])
		case c == '.':
			if err := flush(); err != nil {
				return nil, err
			}
		default:
			label = append(label, c)
		}
	}
	if len(name) > 0 {
		if err := flush(); err != nil {
			return nil, err
		}
	}
	return append(b, 0), nil
}

// parse reads a DNS message: its ID, whether it is a response, its
// questions, and the records of its answers, authorities and additional
// records of the types DNS-SD reads; it leaves out those of other types.
// Names come lowercased, but a PTR's target and a TXT's strings, which keep
// the case they were given.
func parse(msg []byte) (m message, err error) {
	if len(msg) < 12 {
		return m, errShort
	}
	m.id = binary.BigEndian.Uint16(msg)
	m.response = msg[2]&0x80 != 0
	qd := int(binary.BigEndian.Uint16(msg[4:]))
	rr := int(binary.BigEndian.Uint16(msg[6:])) + int(binary.BigEndian.Uint16(msg[8:])) + int(binary.BigEndian.Uint16(msg[10:]))
	off := 12
	for range qd {
		var q question
		if q.name, off, err = readName(msg, off); err != nil {
			return m, err
		}
		if off+4 > len(msg) {
			return m, errShort
		}
		q.name = strings.ToLower(q.name)
		q.qtype = binary.BigEndian.Uint16(msg[off:])
		q.unicast = binary.BigEndian.Uint16(msg[off+2:])&classTop != 0
		off += 4
		m.questions = append(m.questions, q)
	}
	for range rr {
		var r record
		if r.name, off, err = readName(msg, off); err != nil {
			return m, err
		}
		if off+10 > len(msg) {
			return m, errShort
		}
		r.rtype = binary.BigEndian.Uint16(msg[off:])
		r.unique = binary.BigEndian.Uint16(msg[off+2:])&classTop != 0
		r.ttl = binary.BigEndian.Uint32(msg[off+4:])
		size := int(binary.BigEndian.Uint16(msg[off+8:]))
		start := off + 10
		end := start + size
		if end > len(msg) {
			return m, errShort
		}
		off = end
		r.name = strings.ToLower(r.name)
		data := msg[start:end]
		switch r.rtype {
		case typePTR:
			if r.target, _, err = readName(msg, start); err != nil {
				return m, err
			}
		case typeSRV:
			if size < 7 {
				return m, errShort
			}
			r.port = binary.BigEndian.Uint16(data[4:])
			if r.target, _, err = readName(msg, start+6); err != nil {
				return m, err
			}
			r.target = strings.ToLower(r.target)
		case typeTXT:
			for len(data) > 0 {
				n := int(data[0])
				if 1+n > len(data) {
					return m, errShort
				}
				if n > 0 {
					r.txt = append(r.txt, string(data[1:1+n]))
				}
				data = data[1+n:]
			}
		case typeA:
			if size != 4 {
				continue
			}
			r.ip = net.IPv4(data[0], data[1], data[2], data[3])
		default:
			continue
		}
		m.records = append(m.records, r)
	}
	return m, nil
}

// readName reads the name at off of msg, following its compression
// pointers, and returns it with its labels' dots and backslashes escaped,
// ending in a dot, and the offset past it.
func readName(msg []byte, off int) (string, int, error) {
	var sb strings.Builder
	end := -1 // past the name where it is, once a pointer jumped
	for hops := 0; ; hops++ {
		if off >= len(msg) || hops > 64 {
			return "", 0, errShort
		}
		n := int(msg[off])
		switch {
		case n == 0:
			if end < 0 {
				end = off + 1
			}
			if sb.Len() == 0 {
				sb.WriteByte('.')
			}
			return sb.String(), end, nil
		case n&0xC0 == 0xC0:
			if off+1 >= len(msg) {
				return "", 0, errShort
			}
			if end < 0 {
				end = off + 2
			}
			off = int(binary.BigEndian.Uint16(msg[off:]) & 0x3FFF)
		case n&0xC0 != 0:
			return "", 0, errShort
		default:
			if off+1+n > len(msg) {
				return "", 0, errShort
			}
			for _, c := range msg[off+1 : off+1+n] {
				if c == '.' || c == '\\' {
					sb.WriteByte('\\')
				}
				sb.WriteByte(c)
			}
			sb.WriteByte('.')
			off += 1 + n
		}
	}
}

// escapeLabel escapes the dots and backslashes of a label, as an instance's
// name, for a name made of it.
func escapeLabel(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `.`, `\.`)
	return r.Replace(s)
}

// firstLabel returns the first label of a name as readName writes it,
// unescaped.
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
