package portmap

// PCP (RFC 6887): a MAP request over UDP to the router's port 5351, which
// tells this computer's address, and is answered with the external
// address and port the router gives, of IPv4 or of IPv6.

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"net/netip"
	"strconv"
	"time"
)

// PCP's version, its MAP opcode, and the protocol of TCP.
const (
	pcpVersion = 2
	pcpMap     = 1
	protoTCP   = 6
)

// pcpReason tells a result code of PCP (RFC 6887, 7.4).
func pcpReason(code byte) string {
	switch code {
	case 1:
		return "unsupported version"
	case 2:
		return "not authorized: the router may have forwards from computers off"
	case 3:
		return "malformed request"
	case 4:
		return "unsupported opcode"
	case 7:
		return "network failure: the router may have no internet address yet"
	case 8:
		return "no resources"
	case 9:
		return "unsupported protocol"
	case 10:
		return "too many forwards of this computer"
	case 11:
		return "cannot provide an external address or port"
	case 12:
		return "address mismatch: another router stands between"
	case 13:
		return "excessive remote peers"
	}
	return "result code " + strconv.Itoa(int(code))
}

// mapPCP asks server, by PCP, to forward its TCP port external, or another,
// to the port internal of local, an address of this computer, or the one
// the system picks when it isn't valid, for lifetime, or no more when
// lifetime is 0. nonce names the mapping to renew or delete; a new one is
// made when nil. A router that speaks NAT-PMP alone is errNotPCP.
func (c *Client) mapPCP(ctx context.Context, server netip.AddrPort, local netip.Addr, internal, external uint16, lifetime time.Duration, nonce *[12]byte) (*Mapping, error) {
	var n [12]byte
	if nonce != nil {
		n = *nonce
	} else if _, err := rand.Read(n[:]); err != nil {
		return nil, err
	}
	secs := uint32(lifetime / time.Second)
	request := func(me netip.Addr) []byte {
		b := []byte{pcpVersion, pcpMap, 0, 0}
		b = binary.BigEndian.AppendUint32(b, secs)
		me16 := me.As16()
		b = append(b, me16[:]...)
		b = append(b, n[:]...)
		b = append(b, protoTCP, 0, 0, 0)
		b = binary.BigEndian.AppendUint16(b, internal)
		b = binary.BigEndian.AppendUint16(b, external)
		// No address suggested: the all-zeros address of its family.
		var none [16]byte
		if me.Is4() {
			none[10], none[11] = 0xff, 0xff
		}
		return append(b, none[:]...)
	}
	b, me, err := c.ask(ctx, local, server, request, func(b []byte) bool {
		// A NAT-PMP router answers with its own version, 0; PCP's answer
		// to this MAP echoes its nonce.
		if len(b) >= 2 && b[0] == 0 {
			return true
		}
		if len(b) < 4 || b[0] != pcpVersion || b[1] != 0x80|pcpMap {
			return false
		}
		return b[3] != 0 || len(b) >= 60 && [12]byte(b[24:36]) == n
	})
	if err != nil {
		return nil, err
	}
	if b[0] == 0 || b[3] == 1 {
		return nil, errNotPCP
	}
	if code := b[3]; code != 0 {
		return nil, &RefusedError{Method: PCP, Code: int(code), Reason: pcpReason(code)}
	}
	if b[36] != protoTCP || binary.BigEndian.Uint16(b[40:]) != internal {
		return nil, &RefusedError{Method: PCP, Reason: "an answer of another port"}
	}
	life := time.Duration(binary.BigEndian.Uint32(b[4:])) * time.Second
	got := binary.BigEndian.Uint16(b[42:])
	addr := netip.AddrFrom16([16]byte(b[44:60])).Unmap()
	if lifetime > 0 && (got == 0 || life == 0 || !addr.IsValid()) {
		return nil, &RefusedError{Method: PCP, Reason: "no port given"}
	}
	return &Mapping{Method: PCP, External: netip.AddrPortFrom(addr, got), Internal: internal, Lifetime: life, server: server, local: me, nonce: n}, nil
}
