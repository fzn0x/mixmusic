package remote

// The transport of remote control, which the rooms of Listen with friends
// (internal/room) speak over too: TLS 1.3 between two computers' keys, and
// JSON messages, each after its length. docs/architecture/remote.md tells
// why it is so.

import (
	"bufio"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// MaxMessage bounds a message, in bytes.
const MaxMessage = maxMessage

// Frames sends and receives JSON messages over a connection: each after its
// length, in 4 bytes, of at most MaxMessage. Sends may come from any
// goroutine; reads from one at a time.
type Frames struct {
	conn net.Conn
	r    *bufio.Reader
	mu   sync.Mutex
	// limit bounds the messages read, when not 0 (Limit).
	limit uint32
}

// NewFrames frames the messages of conn.
func NewFrames(conn net.Conn) *Frames { return &Frames{conn: conn, r: bufio.NewReader(conn)} }

// Limit bounds the messages Read takes to n bytes, as a server does before
// the other computer proved who it is: a longer one is refused from its
// length, before it is read. 0, as at first, or more than MaxMessage, is
// MaxMessage. Call it between reads.
func (f *Frames) Limit(n int) {
	f.limit = 0
	if n > 0 && n < maxMessage {
		f.limit = uint32(n)
	}
}

// Send sends v, giving up after wait.
func (f *Frames) Send(v any, wait time.Duration) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	b := binary.BigEndian.AppendUint32(make([]byte, 0, 4+len(data)), uint32(len(data)))
	b = append(b, data...)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.conn.SetWriteDeadline(time.Now().Add(wait))
	_, err = f.conn.Write(b)
	return err
}

// Receive waits up to wait for the next message, and reads it into v: a
// wait that runs out is ErrLost.
func (f *Frames) Receive(v any, wait time.Duration) error {
	f.conn.SetReadDeadline(time.Now().Add(wait))
	return f.Read(v)
}

// Read reads the next message into v, until the connection's read
// deadline.
func (f *Frames) Read(v any) error {
	var head [4]byte
	if _, err := io.ReadFull(f.r, head[:]); err != nil {
		return quiet(err)
	}
	n := binary.BigEndian.Uint32(head[:])
	if n > maxMessage || f.limit > 0 && n > f.limit {
		return fmt.Errorf("remote: a message of %d bytes", n)
	}
	data := make([]byte, n)
	if _, err := io.ReadFull(f.r, data); err != nil {
		return quiet(err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("remote: a message that isn't JSON: %w", err)
	}
	return nil
}

// Certificate is the self-signed certificate of the key, which TLS
// presents.
func (id *Identity) Certificate() tls.Certificate { return id.cert }

// ServerTLS is how a server of the protocol proto, as ALPN names it, speaks
// TLS with the key of id: TLS 1.3 alone, and a certificate of an Ed25519
// key asked of each computer that connects, whatever its key. Who it is,
// the server tells from its key (PeerKey).
func (id *Identity) ServerTLS(proto string) *tls.Config {
	return &tls.Config{
		Certificates: []tls.Certificate{id.cert},
		ClientAuth:   tls.RequireAnyClientCert,
		MinVersion:   tls.VersionTLS13,
		NextProtos:   []string{proto},
		VerifyConnection: func(cs tls.ConnectionState) error {
			if cs.NegotiatedProtocol != proto {
				return errors.New("remote: not MixMusic's protocol")
			}
			_, err := peerKey(cs)
			return err
		},
	}
}

// ClientTLS is how a computer of the key id connects to a server of the
// protocol proto: no chain of certificates is checked, but the server's
// key, which must be pinned, unless pinned is nil, or the handshake fails
// with ErrNotSame. peer, when not nil, is set to the server's key.
func (id *Identity) ClientTLS(proto string, pinned, peer *Fingerprint) *tls.Config {
	return &tls.Config{
		Certificates:       []tls.Certificate{id.cert},
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS13,
		NextProtos:         []string{proto},
		ServerName:         "mixmusic",
		VerifyConnection: func(cs tls.ConnectionState) error {
			fp, err := peerKey(cs)
			if err != nil {
				return err
			}
			if pinned != nil && fp != *pinned {
				return ErrNotSame
			}
			if peer != nil {
				*peer = fp
			}
			return nil
		},
	}
}

// PeerKey returns the fingerprint of the certificate a TLS connection's
// other end presented, which must be of an Ed25519 key.
func PeerKey(cs tls.ConnectionState) (Fingerprint, error) { return peerKey(cs) }

// Clean is t with its words as a state's are, its covers only from
// https, and no ID of a file: what another computer sends is shown and
// played, so it can't be trusted more than its user.
func (t QueueTrack) Clean() QueueTrack { return cleanQueueTrack(t) }
