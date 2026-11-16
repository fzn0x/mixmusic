package portmap

// NAT-PMP (RFC 6886): two questions over UDP to the router's port 5351,
// its internet address, then the forward of a TCP port, each answered by
// a result code.

import (
	"context"
	"encoding/binary"
	"net/netip"
	"strconv"
	"time"
)

// NAT-PMP's opcodes.
const (
	pmpAddress = 0
	pmpTCP     = 2
)

// pmpReason tells a result code of NAT-PMP (RFC 6886, 3.5).
func pmpReason(code uint16) string {
	switch code {
	case 1:
		return "unsupported version"
	case 2:
		return "not authorized: the router may have forwards from computers off"
	case 3:
		return "network failure: the router may have no internet address yet"
	case 4:
		return "out of resources"
	case 5:
		return "unsupported opcode"
	}
	return "result code " + strconv.Itoa(int(code))
}

// mapNATPMP asks server, by NAT-PMP, for its internet address, then to
// forward its TCP port external, or another, to the port internal of this
// computer, for lifetime, or no more when lifetime is 0.
func (c *Client) mapNATPMP(ctx context.Context, server netip.AddrPort, internal, external uint16, lifetime time.Duration) (*Mapping, error) {
	var addr netip.Addr
	if lifetime > 0 {
		b, _, err := c.ask(ctx, netip.Addr{}, server, func(netip.Addr) []byte { return []byte{0, pmpAddress} }, func(b []byte) bool {
			return len(b) >= 4 && b[0] == 0 && b[1] == 128+pmpAddress
		})
		if err != nil {
			return nil, err
		}
		if code := binary.BigEndian.Uint16(b[2:]); code != 0 {
			return nil, &RefusedError{Method: NATPMP, Code: int(code), Reason: pmpReason(code)}
		}
		if len(b) < 12 {
			return nil, &RefusedError{Method: NATPMP, Reason: "an answer too short"}
		}
		addr = netip.AddrFrom4([4]byte(b[8:12]))
	}
	secs := uint32(lifetime / time.Second)
	req := []byte{0, pmpTCP, 0, 0}
	req = binary.BigEndian.AppendUint16(req, internal)
	req = binary.BigEndian.AppendUint16(req, external)
	req = binary.BigEndian.AppendUint32(req, secs)
	b, local, err := c.ask(ctx, netip.Addr{}, server, func(netip.Addr) []byte { return req }, func(b []byte) bool {
		return len(b) >= 4 && b[0] == 0 && b[1] == 128+pmpTCP
	})
	if err != nil {
		return nil, err
	}
	if code := binary.BigEndian.Uint16(b[2:]); code != 0 {
		return nil, &RefusedError{Method: NATPMP, Code: int(code), Reason: pmpReason(code)}
	}
	if len(b) < 16 || binary.BigEndian.Uint16(b[8:]) != internal {
		return nil, &RefusedError{Method: NATPMP, Reason: "an answer of another port"}
	}
	got := binary.BigEndian.Uint16(b[10:])
	life := time.Duration(binary.BigEndian.Uint32(b[12:])) * time.Second
	if lifetime > 0 && (got == 0 || life == 0) {
		return nil, &RefusedError{Method: NATPMP, Reason: "no port given"}
	}
	return &Mapping{Method: NATPMP, External: netip.AddrPortFrom(addr, got), Internal: internal, Lifetime: life, server: server, local: local}, nil
}
