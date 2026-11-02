package cast

import (
	"encoding/binary"
	"errors"
	"net"
	"strings"
)

// The DNS record types browsing reads (RFC 1035, RFC 2782), and the
// Internet class.
const (
	typeA     = 1
	typePTR   = 12
	typeTXT   = 16
	typeSRV   = 33
	classINET = 1
)

// question is a question of a DNS query: a name, as "_googlecast._tcp.local.",
// and the type of record asked for.
type question struct {
	name  string
	qtype uint16
}

// record is a resource record of a DNS answer, as far as browsing reads it:
// its name, lowercased, and what its type holds.
type record struct {
	name  string
	rtype uint16
	// target is a PTR's name, or an SRV's host; port is an SRV's.
	target string
	port   uint16
	txt    []string
	ip     net.IP
}

var errDNS = errors.New("cast: a DNS message cut short")

// dnsQuery returns a DNS query of id with questions qs, of the Internet
// class, as mDNS takes it.
func dnsQuery(id uint16, qs []question) ([]byte, error) {
	b := make([]byte, 12, 512)
	binary.BigEndian.PutUint16(b[0:], id)
	binary.BigEndian.PutUint16(b[4:], uint16(len(qs)))
	for _, q := range qs {
		var err error
		if b, err = appendName(b, q.name); err != nil {
			return nil, err
		}
		b = binary.BigEndian.AppendUint16(b, q.qtype)
		b = binary.BigEndian.AppendUint16(b, classINET)
	}
	return b, nil
}

// appendName appends name in DNS's labels. A dot within a label is
// escaped, as "\.", and so is a backslash, as names read have them.
func appendName(b []byte, name string) ([]byte, error) {
	name = strings.TrimSuffix(name, ".")
	var label []byte
	flush := func() error {
		if len(label) == 0 || len(label) > 63 {
			return errors.New("cast: a name's label is empty or too long")
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

// parseDNS reads a DNS message: its ID, whether it is a response, and the
// records of its answers, authorities and additional records that browsing
// reads; it leaves out those of other types.
func parseDNS(msg []byte) (id uint16, response bool, records []record, err error) {
	if len(msg) < 12 {
		return 0, false, nil, errDNS
	}
	id = binary.BigEndian.Uint16(msg)
	response = msg[2]&0x80 != 0
	qd := int(binary.BigEndian.Uint16(msg[4:]))
	rr := int(binary.BigEndian.Uint16(msg[6:])) + int(binary.BigEndian.Uint16(msg[8:])) + int(binary.BigEndian.Uint16(msg[10:]))
	off := 12
	for range qd {
		if _, off, err = readName(msg, off); err != nil {
			return
		}
		if off += 4; off > len(msg) {
			return 0, false, nil, errDNS
		}
	}
	for range rr {
		var r record
		if r.name, off, err = readName(msg, off); err != nil {
			return
		}
		if off+10 > len(msg) {
			return 0, false, nil, errDNS
		}
		r.rtype = binary.BigEndian.Uint16(msg[off:])
		size := int(binary.BigEndian.Uint16(msg[off+8:]))
		start := off + 10
		end := start + size
		if end > len(msg) {
			return 0, false, nil, errDNS
		}
		off = end
		r.name = strings.ToLower(r.name)
		data := msg[start:end]
		switch r.rtype {
		case typePTR:
			if r.target, _, err = readName(msg, start); err != nil {
				return
			}
		case typeSRV:
			if size < 7 {
				return 0, false, nil, errDNS
			}
			r.port = binary.BigEndian.Uint16(data[4:])
			if r.target, _, err = readName(msg, start+6); err != nil {
				return
			}
		case typeTXT:
			for len(data) > 0 {
				n := int(data[0])
				if 1+n > len(data) {
					return 0, false, nil, errDNS
				}
				r.txt = append(r.txt, string(data[1:1+n]))
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
		records = append(records, r)
	}
	return id, response, records, nil
}

// readName reads the name at off of msg, following its compression
// pointers, and returns it with its labels' dots and backslashes escaped,
// ending in a dot, and the offset past it.
func readName(msg []byte, off int) (string, int, error) {
	var sb strings.Builder
	end := -1 // past the name where it is, once a pointer jumped
	for hops := 0; ; hops++ {
		if off >= len(msg) || hops > 64 {
			return "", 0, errDNS
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
				return "", 0, errDNS
			}
			if end < 0 {
				end = off + 2
			}
			off = int(binary.BigEndian.Uint16(msg[off:]) & 0x3FFF)
		case n&0xC0 != 0:
			return "", 0, errDNS
		default:
			if off+1+n > len(msg) {
				return "", 0, errDNS
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
