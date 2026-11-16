package room

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/remote"
)

// The tests of floods give each connection an address of their choice:
// the dialer's first line names it, which the room's listener reads.

// srcConn is a connection of the address src, which reads past the line
// that named it.
type srcConn struct {
	net.Conn
	r   *bufio.Reader
	src net.Addr
}

func (c *srcConn) Read(b []byte) (int, error) { return c.r.Read(b) }
func (c *srcConn) RemoteAddr() net.Addr       { return c.src }

// srcListener takes the connections whose first line names their address.
type srcListener struct{ net.Listener }

func (l srcListener) Accept() (net.Conn, error) {
	for {
		c, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}
		c.SetReadDeadline(time.Now().Add(5 * time.Second))
		r := bufio.NewReader(c)
		line, err := r.ReadString('\n')
		c.SetReadDeadline(time.Time{})
		if err != nil {
			c.Close()
			continue
		}
		return &srcConn{Conn: c, r: r, src: &net.TCPAddr{IP: net.ParseIP(strings.TrimSpace(line)), Port: 4000}}, nil
	}
}

// dialFrom connects to addr as the address src.
func dialFrom(ctx context.Context, addr, src string) (net.Conn, error) {
	c, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	if _, err := c.Write([]byte(src + "\n")); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

// slowConn holds each write and each read a while, as a network far
// away.
type slowConn struct {
	net.Conn
	delay time.Duration
}

func (c slowConn) Write(b []byte) (int, error) {
	time.Sleep(c.delay)
	return c.Conn.Write(b)
}

func (c slowConn) Read(b []byte) (int, error) {
	n, err := c.Conn.Read(b)
	time.Sleep(c.delay)
	return n, err
}

// newFloodRoom is a test room of the default bounds, whose connections
// tell their addresses.
func newFloodRoom(t *testing.T) *testRoom {
	t.Helper()
	id, err := remote.NewIdentity()
	if err != nil {
		t.Fatal(err)
	}
	clock := NewSystemClock()
	host := &fakeHost{clock: clock}
	host.play("e1", "First song", 0)
	secret, _ := NewSecret()
	srv := &Server{Identity: id, Name: "Desk", Code: "ABC123", Secret: secret, Host: host, Clock: clock, Timing: testTiming}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go srv.Serve(srcListener{ln})
	t.Cleanup(func() { srv.Close() })
	return &testRoom{t: t, server: srv, host: host, addr: ln.Addr().String(), key: id.Fingerprint()}
}

// flood has sources sources open connections to addr that send nothing,
// each as fast as the room takes them, until stop.
func flood(addr string, sources int, stop chan struct{}) *sync.WaitGroup {
	var wg sync.WaitGroup
	for i := range sources {
		for range maxUnprovenFrom {
			wg.Add(1)
			go func() {
				defer wg.Done()
				src := fmt.Sprintf("203.0.113.%d", i+1)
				buf := make([]byte, 1)
				for {
					select {
					case <-stop:
						return
					default:
					}
					c, err := dialFrom(context.Background(), addr, src)
					if err != nil {
						continue
					}
					// Held until the room closes it, or the flood stops.
					done := make(chan struct{})
					go func() {
						select {
						case <-stop:
							c.Close()
						case <-done:
						}
					}()
					c.Read(buf)
					close(done)
					c.Close()
				}
			}()
		}
	}
	return &wg
}

// TestGuestFarAwayJoinsThroughAFlood floods a room from 16 sources, each
// holding as many connections that send nothing as it may, as fast as the
// room takes them, while a guest 30 ms away knocks with the link, 3 times:
// it joins each time.
func TestGuestFarAwayJoinsThroughAFlood(t *testing.T) {
	r := newFloodRoom(t)
	stop := make(chan struct{})
	wg := flood(r.addr, 16, stop)
	defer func() {
		close(stop)
		wg.Wait()
	}()
	// The flood fills the room's places first.
	waitFor(t, "the places full", func() bool {
		r.server.mu.Lock()
		defer r.server.mu.Unlock()
		return len(r.server.adm.unproven) >= maxUnproven
	})
	for i := range 3 {
		g := newGuest(t, "Far")
		g.Dial = func(ctx context.Context, _, addr string) (net.Conn, error) {
			c, err := dialFrom(ctx, addr, "198.51.100.7")
			if err != nil {
				return nil, err
			}
			return slowConn{c, 15 * time.Millisecond}, nil
		}
		s, err := g.Knock(context.Background(), r.addr, &r.key, r.server.Secret, true)
		if err != nil {
			t.Fatalf("knock %d through the flood: %v", i, err)
		}
		if err := s.Join(context.Background()); err != nil {
			t.Fatalf("join %d through the flood: %v", i, err)
		}
		s.Leave()
	}
}
