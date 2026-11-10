package remote

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"slices"
	"sync"
	"time"
)

// Dialer is the computer that controls others': it pairs with them, and
// connects to those paired.
type Dialer struct {
	Identity *Identity
	// Name is the computer's, which the others show.
	Name   string
	Timing Timing
	// Dial connects to an address, as net.Dialer does when nil.
	Dial func(ctx context.Context, network, addr string) (net.Conn, error)
}

// Peer is a computer paired with: its key, and its name, as it told it.
type Peer struct {
	Key  Fingerprint
	Name string
}

// connect connects to addr over TLS, and returns the connection and the
// key of the computer there, which must be pinned, unless pinned is nil.
func (d *Dialer) connect(ctx context.Context, addr string, pinned *Fingerprint) (*tls.Conn, Fingerprint, error) {
	dial := d.Dial
	if dial == nil {
		dial = (&net.Dialer{}).DialContext
	}
	ctx, cancel := context.WithTimeout(ctx, d.Timing.silence())
	defer cancel()
	raw, err := dial(ctx, "tcp", addr)
	if err != nil {
		return nil, Fingerprint{}, err
	}
	var peer Fingerprint
	tc := tls.Client(raw, &tls.Config{
		Certificates: []tls.Certificate{d.Identity.cert},
		// No chain is checked: the key is, below.
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS13,
		NextProtos:         []string{alpn},
		ServerName:         "mixmusic",
		VerifyConnection: func(cs tls.ConnectionState) error {
			fp, err := peerKey(cs)
			if err != nil {
				return err
			}
			if pinned != nil && fp != *pinned {
				return ErrNotSame
			}
			peer = fp
			return nil
		},
	})
	if err := tc.HandshakeContext(ctx); err != nil {
		raw.Close()
		if errors.Is(err, ErrNotSame) {
			return nil, peer, ErrNotSame
		}
		return nil, peer, err
	}
	if tc.ConnectionState().NegotiatedProtocol != alpn {
		tc.Close()
		return nil, peer, errors.New("remote: not MixMusic's protocol")
	}
	return tc, peer, nil
}

// Pair pairs with the computer at addr: once it shows its code, code is
// asked for what the user typed, and may wait for them until ctx ends. It
// returns the computer's key and name, which the caller keeps.
func (d *Dialer) Pair(ctx context.Context, addr string, code func(ctx context.Context) (string, error)) (Peer, error) {
	t := d.Timing
	tc, host, err := d.connect(ctx, addr, nil)
	if err != nil {
		return Peer{}, err
	}
	defer tc.Close()
	stop := context.AfterFunc(ctx, func() { tc.Close() })
	defer stop()
	w := newWire(tc)
	fail := func(err error) (Peer, error) {
		if ctx.Err() != nil {
			return Peer{}, ctx.Err()
		}
		return Peer{}, err
	}
	if err := w.send(message{Type: "hello", Want: wantPair, Name: d.Name, Version: version}, t.silence()); err != nil {
		return fail(err)
	}
	if _, err := w.expect("code", t.silence()); err != nil {
		return fail(err)
	}
	typed, err := d.typed(ctx, tc, w, code)
	if err != nil {
		return fail(err)
	}
	value, ok := CodeValue(typed)
	if !ok {
		return fail(ErrWrongCode)
	}
	ts, err := transcriptOf(tc.ConnectionState(), d.Identity.fp, host)
	if err != nil {
		return fail(err)
	}
	if err := (pairing{w: w, t: ts, code: value, first: t.silence(), wait: t.silence()}).controller(); err != nil {
		return fail(err)
	}
	if err := w.send(message{Type: "confirm"}, t.silence()); err != nil {
		return fail(err)
	}
	paired, err := w.expect("paired", t.silence())
	if err != nil {
		return fail(err)
	}
	return Peer{Key: host, Name: cleanName(paired.Name)}, nil
}

// typed asks code for what the user typed, while it listens to the
// computer paired with, which sends nothing meanwhile but that its user
// canceled, or its code ran out: either ends the wait.
func (d *Dialer) typed(ctx context.Context, tc *tls.Conn, w *wire, code func(ctx context.Context) (string, error)) (string, error) {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	heard := make(chan error, 1)
	// The deadline the listening sets comes before the one that stops it.
	tc.SetReadDeadline(time.Now().Add(d.Timing.code()))
	go func() {
		m, err := w.read()
		if err == nil {
			err = errors.New("remote: a message where none was due")
			if m.Type == "failed" || m.Type == "refused" {
				err = errOf(m.Reason)
			}
		}
		heard <- err
		cancel(err)
	}()
	typed, err := code(ctx)
	// The listening stops, which a message that came meanwhile beat.
	tc.SetReadDeadline(time.Now())
	if herr := <-heard; !errors.Is(herr, ErrLost) {
		return "", herr
	}
	if err != nil {
		if cause := context.Cause(ctx); cause != nil && !errors.Is(cause, context.Canceled) {
			return "", cause
		}
		return "", err
	}
	return typed, nil
}

// Connect starts a session with the computer at addr, whose key must be
// key: one paired with.
func (d *Dialer) Connect(ctx context.Context, addr string, key Fingerprint) (*Session, error) {
	t := d.Timing
	tc, _, err := d.connect(ctx, addr, &key)
	if err != nil {
		return nil, err
	}
	w := newWire(tc)
	if err := w.send(message{Type: "hello", Want: wantControl, Name: d.Name, Version: version}, t.silence()); err != nil {
		tc.Close()
		return nil, err
	}
	welcome, err := w.expect("welcome", t.silence())
	if err != nil {
		tc.Close()
		return nil, err
	}
	s := &Session{w: w, conn: tc, name: cleanName(welcome.Name), timing: t, done: make(chan struct{})}
	go s.read()
	go s.ping()
	return s, nil
}

// Session is a session with a computer controlled: the state it sends,
// and the commands sent to it. It ends as that computer goes quiet, or
// refuses it, or Close.
type Session struct {
	w      *wire
	conn   net.Conn
	name   string
	timing Timing

	mu       sync.Mutex
	state    State
	at       time.Time
	has      bool
	watchers []*func()
	err      error
	done     chan struct{}
	once     sync.Once
}

// Name returns the name of the computer controlled, as it told it.
func (s *Session) Name() string { return s.name }

// State returns the state the computer controlled sent last, when it came,
// and whether one came yet.
func (s *Session) State() (State, time.Time, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state, s.at, s.has
}

// Watch has fn told, from any goroutine, as a state comes, and as the
// session ends, until cancel.
func (s *Session) Watch(fn func()) (cancel func()) {
	f := &fn
	s.mu.Lock()
	s.watchers = append(slices.Clone(s.watchers), f)
	s.mu.Unlock()
	return func() {
		s.mu.Lock()
		s.watchers = slices.DeleteFunc(slices.Clone(s.watchers), func(g *func()) bool { return g == f })
		s.mu.Unlock()
	}
}

func (s *Session) tell() {
	s.mu.Lock()
	fns := s.watchers
	s.mu.Unlock()
	for _, fn := range fns {
		(*fn)()
	}
}

// Do sends a command.
func (s *Session) Do(c Command) error {
	c, ok := c.clean()
	if !ok {
		return errors.New("remote: no such command")
	}
	select {
	case <-s.done:
		return s.Err()
	default:
	}
	return s.w.send(message{Type: "command", Command: &c}, s.timing.silence())
}

// Done is closed once the session ended.
func (s *Session) Done() <-chan struct{} { return s.done }

// Err returns why the session ended: ErrLost for a computer that went
// quiet, ErrNotPaired for one that removed this one, net.ErrClosed after
// Close.
func (s *Session) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

// Close ends the session.
func (s *Session) Close() { s.end(net.ErrClosed) }

func (s *Session) end(err error) {
	s.once.Do(func() {
		s.mu.Lock()
		s.err = err
		s.mu.Unlock()
		s.conn.Close()
		close(s.done)
		s.tell()
	})
}

func (s *Session) read() {
	for {
		m, err := s.w.receive(s.timing.silence())
		if err != nil {
			select {
			case <-s.done:
			default:
				if !errors.Is(err, ErrLost) {
					// The other end closed, or the network failed.
					err = ErrLost
				}
				s.end(err)
			}
			return
		}
		switch m.Type {
		case "state":
			if m.State == nil {
				continue
			}
			st := cleanState(*m.State)
			s.mu.Lock()
			s.state, s.at, s.has = st, time.Now(), true
			s.mu.Unlock()
			s.tell()
		case "refused":
			s.end(errOf(m.Reason))
			return
		}
	}
}

func (s *Session) ping() {
	tick := time.NewTicker(s.timing.ping())
	defer tick.Stop()
	for {
		select {
		case <-s.done:
			return
		case <-tick.C:
			if err := s.w.send(message{Type: "ping"}, s.timing.silence()); err != nil {
				s.end(ErrLost)
				return
			}
		}
	}
}
