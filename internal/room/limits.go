package room

// What a room takes of anyone who connects, on top of admission
// (admit.go), as anyone on the internet may knock once friends outside the
// network join (#562): at most maxConnsFrom connections of one source at
// once, proven or not, and maxConns in all, past which a connection is
// closed as it comes; and maxRate messages a second from one connection,
// past which its messages wait to be read, so that its sends wait in turn.
// A message is of remote.MaxMessage bytes at most, and of a handshake's
// size until its sender proved the code, the link or its key.

import (
	"net"
	"sync"
	"time"

	"github.com/fzn0x/mixmusic/internal/remote"
)

const (
	// maxConnsFrom bounds the connections of a source at once, those
	// that proved something too: a household behind one address of IPv4
	// has room for its computers.
	maxConnsFrom = 32
	// maxConns bounds the connections of the room at once.
	maxConns = 512
	// maxRate bounds the messages read of a connection each second: a
	// guest sends a few, a ping each 2 s.
	maxRate = 60
)

func (c caps) messageRate() int {
	if c.rate > 0 {
		return c.rate
	}
	return maxRate
}

// limited is ln, which holds no more connections of a source at once than
// maxConnsFrom, nor more than maxConns in all: one more is closed as it
// comes. Each is more than the places of the unproven, of a source and in
// all, so that a new connection can take the place of the oldest
// (admission.letIn) rather than be closed.
func limited(ln net.Listener, c caps) net.Listener {
	return &limitListener{Listener: ln, most: max(maxConnsFrom, c.unprovenFrom+1), all: max(maxConns, c.unproven+1), from: map[string]int{}}
}

type limitListener struct {
	net.Listener
	most, all int

	mu   sync.Mutex
	from map[string]int
	n    int
}

func (l *limitListener) Accept() (net.Conn, error) {
	for {
		c, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}
		src := sourceOf(c.RemoteAddr().String())
		l.mu.Lock()
		full := l.from[src] >= l.most || l.n >= l.all
		if !full {
			l.from[src]++
			l.n++
		}
		l.mu.Unlock()
		if full {
			c.Close()
			continue
		}
		return &heldConn{Conn: c, l: l, src: src}, nil
	}
}

func (l *limitListener) release(src string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.n--
	if l.from[src]--; l.from[src] <= 0 {
		delete(l.from, src)
	}
}

// heldConn is a connection limitListener holds until it closes.
type heldConn struct {
	net.Conn
	l    *limitListener
	src  string
	once sync.Once
}

func (c *heldConn) Close() error {
	c.once.Do(func() { c.l.release(c.src) })
	return c.Conn.Close()
}

// frames are the messages of a connection the room has, of which it reads
// rate a second at most, after a burst of as many: one sent faster waits
// to be read. Reads come from one goroutine at a time, as Frames' do.
type frames struct {
	*remote.Frames
	rate   float64
	tokens float64
	last   time.Time
}

func newFrames(c net.Conn, rate int) *frames {
	return &frames{Frames: remote.NewFrames(c), rate: float64(rate), tokens: float64(rate)}
}

// Receive waits until the next message may be read, then up to wait for
// it.
func (f *frames) Receive(v any, wait time.Duration) error {
	now := time.Now()
	if !f.last.IsZero() {
		f.tokens = min(f.rate, f.tokens+now.Sub(f.last).Seconds()*f.rate)
	}
	f.last = now
	if f.tokens < 1 {
		d := time.Duration((1 - f.tokens) / f.rate * float64(time.Second))
		time.Sleep(d)
		f.last = f.last.Add(d)
		f.tokens = 1
	}
	f.tokens--
	return f.Frames.Receive(v, wait)
}
