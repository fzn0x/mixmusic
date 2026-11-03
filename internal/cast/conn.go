package cast

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// These times are the protocol's; tests shorten them.
var (
	// pingEvery is how often the connection is pinged, as senders do.
	pingEvery = 5 * time.Second
	// silentFor is how long a device says nothing, not even to the pings,
	// before its connection is taken for lost.
	silentFor = 16 * time.Second
	// dialTimeout bounds connecting to a device.
	dialTimeout = 10 * time.Second
)

// ErrClosed is why a request of a connection that closed fails.
var ErrClosed = errors.New("cast: the connection to the device closed")

// errSilent is why a connection whose device stopped answering closes.
var errSilent = errors.New("the device stopped answering")

// header is what every JSON payload of the protocol starts with.
type header struct {
	Type      string `json:"type"`
	RequestID int    `json:"requestId"`
	Reason    string `json:"reason"`
}

// reply is a message that answers a request.
type reply struct {
	msg    message
	header header
}

// conn is a connection to a cast device: TLS, with the protocol's frames,
// and the heartbeat that keeps it. Replies to requests are told apart by
// their requestId, and every message but the heartbeat's goes to onMessage
// too, as statuses tell of the device whether they answer a request or
// not.
type conn struct {
	c      net.Conn
	sender string
	// onMessage is told each message, on the connection's goroutine.
	onMessage func(message, header)

	writing sync.Mutex
	mu      sync.Mutex
	nextID  int
	waiting map[int]chan reply
	closed  chan struct{}
	err     error
	seen    atomic.Int64 // when the device last sent anything, in Unix ns
	once    sync.Once
}

// dial connects to the device at addr. Cast devices have certificates of
// their own, which no authority signs that a sender could check without
// the device authentication MixMusic doesn't need: the connection is
// encrypted, and the device isn't checked.
func dial(ctx context.Context, addr string, onMessage func(message, header)) (*conn, error) {
	ctx, cancel := context.WithTimeout(ctx, dialTimeout)
	defer cancel()
	d := tls.Dialer{Config: &tls.Config{InsecureSkipVerify: true}} //nolint:gosec // see above
	nc, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	var id [4]byte
	_, _ = rand.Read(id[:])
	c := &conn{c: nc, sender: "sender-" + hex.EncodeToString(id[:]), onMessage: onMessage,
		waiting: map[int]chan reply{}, closed: make(chan struct{})}
	c.seen.Store(time.Now().UnixNano())
	go c.read()
	go c.beat()
	return c, nil
}

// localAddr is the computer's address the connection leaves from: the one
// the device reaches it at.
func (c *conn) localAddr() net.Addr { return c.c.LocalAddr() }

// remoteIP is the device's address.
func (c *conn) remoteIP() net.IP {
	if a, ok := c.c.RemoteAddr().(*net.TCPAddr); ok {
		return a.IP
	}
	return nil
}

// close closes the connection for err, once.
func (c *conn) close(err error) {
	c.once.Do(func() {
		c.mu.Lock()
		c.err = err
		c.mu.Unlock()
		close(c.closed)
		c.c.Close()
	})
}

// shutdown closes the connection as the sender leaves: what it sent last
// still reaches the device, which closes its side, or is cut off after a
// moment.
func (c *conn) shutdown() {
	c.once.Do(func() {
		c.mu.Lock()
		c.err = ErrClosed
		c.mu.Unlock()
		close(c.closed)
		c.writing.Lock()
		if tc, ok := c.c.(*tls.Conn); ok {
			c.c.SetWriteDeadline(time.Now().Add(time.Second))
			_ = tc.CloseWrite()
		}
		c.writing.Unlock()
		time.AfterFunc(time.Second, func() { c.c.Close() })
	})
}

// reason returns why the connection closed, or nil while it's open.
func (c *conn) reason() error {
	select {
	case <-c.closed:
		c.mu.Lock()
		defer c.mu.Unlock()
		return c.err
	default:
		return nil
	}
}

// connect opens the virtual connection to dest, which every destination
// needs before it takes messages.
func (c *conn) connect(dest string) error {
	return c.send(nsConnection, dest, map[string]any{"type": "CONNECT", "userAgent": "MixMusic"})
}

// send sends payload to dest in namespace ns, as JSON, without waiting for
// an answer.
func (c *conn) send(ns, dest string, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if err := c.reason(); err != nil {
		return ErrClosed
	}
	c.writing.Lock()
	defer c.writing.Unlock()
	c.c.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err := writeMessage(c.c, message{Source: c.sender, Destination: dest, Namespace: ns, Payload: b}); err != nil {
		c.close(err)
		return err
	}
	return nil
}

// RequestError is a request the device turned down: Type is how, as
// LOAD_FAILED or INVALID_REQUEST, and Reason why, when it tells.
type RequestError struct {
	Type, Reason string
}

func (e *RequestError) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("cast: %s (%s)", e.Type, e.Reason)
	}
	return "cast: " + e.Type
}

// failures are the types of the answers that turn a request down.
var failures = map[string]bool{
	"LOAD_FAILED": true, "LOAD_CANCELLED": true, "INVALID_REQUEST": true, "INVALID_PLAYER_STATE": true,
	"LAUNCH_ERROR": true, "ERROR": true,
}

// newID returns a requestId no request had.
func (c *conn) newID() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nextID++
	return c.nextID
}

// request sends payload, with a requestId of its own, and returns the
// answer, as a RequestError when it turns the request down.
func (c *conn) request(ctx context.Context, ns, dest string, payload map[string]any) (reply, error) {
	return c.requestAs(ctx, c.newID(), ns, dest, payload)
}

// requestAs is request with the requestId id, from newID.
func (c *conn) requestAs(ctx context.Context, id int, ns, dest string, payload map[string]any) (reply, error) {
	ch := make(chan reply, 1)
	c.mu.Lock()
	c.waiting[id] = ch
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.waiting, id)
		c.mu.Unlock()
	}()
	payload["requestId"] = id
	if err := c.send(ns, dest, payload); err != nil {
		return reply{}, err
	}
	select {
	case r := <-ch:
		if failures[r.header.Type] {
			return r, &RequestError{Type: r.header.Type, Reason: r.header.Reason}
		}
		return r, nil
	case <-c.closed:
		return reply{}, ErrClosed
	case <-ctx.Done():
		return reply{}, ctx.Err()
	}
}

// read reads the device's messages until the connection closes: it
// answers pings, hands requests their answers, and tells onMessage the
// rest.
func (c *conn) read() {
	for {
		m, err := readMessage(c.c)
		if err != nil {
			if errors.Is(err, io.EOF) {
				err = errors.New("the device closed the connection")
			}
			c.close(err)
			return
		}
		c.seen.Store(time.Now().UnixNano())
		if m.Binary {
			continue // device authentication, which MixMusic doesn't ask for
		}
		var h header
		if json.Unmarshal(m.Payload, &h) != nil {
			continue
		}
		if m.Namespace == nsHeartbeat {
			if h.Type == "PING" {
				_ = c.send(nsHeartbeat, m.Source, map[string]string{"type": "PONG"})
			}
			continue
		}
		if h.RequestID != 0 {
			c.mu.Lock()
			ch := c.waiting[h.RequestID]
			c.mu.Unlock()
			if ch != nil {
				select {
				case ch <- reply{msg: m, header: h}:
				default:
				}
			}
		}
		if c.onMessage != nil {
			c.onMessage(m, h)
		}
	}
}

// beat pings the device every pingEvery, and closes the connection once
// it said nothing for silentFor.
func (c *conn) beat() {
	t := time.NewTicker(pingEvery)
	defer t.Stop()
	for {
		select {
		case <-c.closed:
			return
		case <-t.C:
			if time.Since(time.Unix(0, c.seen.Load())) > silentFor {
				c.close(errSilent)
				return
			}
			_ = c.send(nsHeartbeat, receiverID, map[string]string{"type": "PING"})
		}
	}
}
