// Package fcast plays on FCast receivers: the open receiver apps of TVs,
// TV sticks and computers. Browse finds them on the network by mDNS, under
// _fcast._tcp, and Open starts a session on one, which plays the media at
// a URL it's given.
//
// It speaks protocol v4 to a receiver that announces it, over TLS 1.3,
// with the receiver's key checked against the fingerprint it announced,
// and never falls back to plain v3 then; to an older receiver it speaks
// v3, in JSON. Both are as docs.fcast.org tells them. Nothing here comes
// from an FCast library.
package fcast

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/fzn0x/mixmusic/internal/dnssd"
)

// Service is the name receivers announce themselves under by mDNS.
const Service = "_fcast._tcp.local."

// Receiver is an FCast receiver on the network, as it announces itself.
type Receiver struct {
	// ID is the receiver's own, as MixMusic tells it apart: its
	// fingerprint, when it announces one, else its name.
	ID   string
	Name string
	// Host and Port are where it speaks the protocol.
	Host string
	Port int
	// Version is the highest version of the protocol it speaks, as its
	// "v" TXT value tells: 0 when it tells none.
	Version int
	// Fingerprint is its "fp" TXT value: the SHA-256 of its key, in
	// base64, which a v4 connection checks.
	Fingerprint string
}

// Addr returns the receiver's address, host:port.
func (r Receiver) Addr() string { return net.JoinHostPort(r.Host, strconv.Itoa(r.Port)) }

// BrowseOptions configure Browse. Their zero value asks the network.
type BrowseOptions struct {
	// Wait is how long Browse waits for answers: a second when 0.
	Wait time.Duration
	// To and From are dnssd's: a test's responder, and the test's
	// address to ask from.
	To, From string
}

// Browse finds the FCast receivers on the computer's networks, sorted by
// name.
func Browse(ctx context.Context, opts BrowseOptions) ([]Receiver, error) {
	insts, err := dnssd.Browse(ctx, Service, dnssd.BrowseOptions{Wait: opts.Wait, To: opts.To, From: opts.From})
	if err != nil {
		return nil, err
	}
	var out []Receiver
	for _, in := range insts {
		r := Receiver{Name: in.Name, Host: in.Host, Port: in.Port, Fingerprint: in.Text["fp"]}
		r.Version, _ = strconv.Atoi(strings.TrimSpace(in.Text["v"]))
		r.ID = "fcast:" + r.Name
		if r.Fingerprint != "" {
			r.ID = "fcast:" + r.Fingerprint
		}
		if slices.ContainsFunc(out, func(o Receiver) bool { return o.ID == r.ID }) {
			continue
		}
		out = append(out, r)
	}
	slices.SortFunc(out, func(a, b Receiver) int { return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)) })
	return out, nil
}

// The opcodes of the packets, of both versions.
const (
	opPlay            = 1
	opPause           = 2
	opResume          = 3
	opStop            = 4
	opSeek            = 5
	opPlaybackUpdate  = 6
	opVolumeUpdate    = 7
	opSetVolume       = 8
	opPlaybackError   = 9
	opSetSpeed        = 10
	opVersion         = 11
	opPing            = 12
	opPong            = 13
	opInitial         = 14
	opPlayUpdate      = 15
	opSubscribeEvent  = 17
	opEvent           = 19
	opFlatbuf         = 20
	maxPacketV3       = 32000
	maxPacketV4       = 512 << 10
	firstVersionOfTLS = 4
)

// errPacket is why a connection ends whose packet is malformed.
var errPacket = errors.New("fcast: a malformed packet")

// packet is a packet of the protocol: its opcode and body.
type packet struct {
	op   byte
	body []byte
}

// writePacket writes a packet: its size, little-endian, which counts the
// opcode and the body, the opcode and the body.
func writePacket(w io.Writer, op byte, body []byte) error {
	b := make([]byte, 5, 5+len(body))
	binary.LittleEndian.PutUint32(b, uint32(1+len(body)))
	b[4] = op
	_, err := w.Write(append(b, body...))
	return err
}

// readPacket reads a packet of at most limit bytes, opcode and body.
func readPacket(r io.Reader, limit int) (packet, error) {
	var h [4]byte
	if _, err := io.ReadFull(r, h[:]); err != nil {
		return packet{}, err
	}
	n := binary.LittleEndian.Uint32(h[:])
	if n == 0 || n > uint32(limit) {
		return packet{}, fmt.Errorf("%w: of %d bytes", errPacket, n)
	}
	b := make([]byte, n)
	if _, err := io.ReadFull(r, b); err != nil {
		return packet{}, err
	}
	return packet{op: b[0], body: b[1:]}, nil
}
