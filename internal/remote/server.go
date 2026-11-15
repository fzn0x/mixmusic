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

// Timing is how long the protocol waits. Its zero fields are the
// defaults: tests shorten them.
type Timing struct {
	// Ping is how often each side of a session sends, when it has nothing
	// else to: 2 s.
	Ping time.Duration
	// Silence is how long a side waits for a message before it takes the
	// other for gone: 6 s.
	Silence time.Duration
	// Code is how long a code holds, while its user types it: 2 minutes.
	Code time.Duration
	// Keep is how long the computer controlled may take to keep a pairing,
	// once both proved the code, as on a slow or busy disk: 30 s. The other
	// waits that long for paired, and a silence more.
	Keep time.Duration
	// StateEvery is how often at most the computer controlled sends its
	// state as it changes: 200 ms.
	StateEvery time.Duration
	// Rest is how long the computer controlled refuses to pair again after
	// a code that didn't match: 3 s.
	Rest time.Duration

	// linger, for tests, holds a pairing's handler before it returns, once
	// the other computer was told how the pairing went, as a busy computer
	// may: none at all.
	linger time.Duration
}

func (t Timing) ping() time.Duration       { return or(t.Ping, 2*time.Second) }
func (t Timing) silence() time.Duration    { return or(t.Silence, 6*time.Second) }
func (t Timing) code() time.Duration       { return or(t.Code, 2*time.Minute) }
func (t Timing) keep() time.Duration       { return or(t.Keep, 30*time.Second) }
func (t Timing) stateEvery() time.Duration { return or(t.StateEvery, 200*time.Millisecond) }
func (t Timing) rest() time.Duration       { return or(t.Rest, 3*time.Second) }

func or(d, def time.Duration) time.Duration {
	if d > 0 {
		return d
	}
	return def
}

// Host is what the computer controlled does for its server: it tells who
// may control it, shows the codes, and runs the player.
type Host interface {
	// Allowed reports whether the computer of the key fp may control this
	// one.
	Allowed(fp Fingerprint) bool
	// Pairing shows req's code while its pairing runs, or returns an
	// error, as ErrOff, which refuses it. The server ends it with the
	// PairEnd's calls.
	Pairing(req PairRequest) (PairEnd, error)
	// State is what the player does now.
	State() State
	// Do runs a command.
	Do(c Command)
	// TakeOver plays what the computer of the key fp hands over, in place
	// of what plays, and tells how it took it.
	TakeOver(fp Fingerprint, h HandOver) Taken
	// Watch has fn told, without waiting on it, when the state changes,
	// until cancel.
	Watch(fn func()) (cancel func())
}

// PairRequest is a pairing a computer asks for: its name, as it tells it,
// its key, and the code to show.
type PairRequest struct {
	Name string
	Peer Fingerprint
	Code string
	// Cancel ends the pairing, as the user of the computer controlled
	// refused it.
	Cancel func()
}

// PairEnd is how the host ends a pairing whose code it shows: it keeps
// the other computer once both proved the code, then hears how the
// pairing ended, so that both computers end it the same way.
type PairEnd struct {
	// Keep keeps the other computer as paired, once both computers proved
	// the code, and returns an error if it can't, which fails the pairing.
	// The other may control this one from then on: it is told so after.
	Keep func() error
	// Done is told once, last, how the pairing ended: nil once the other
	// computer was told it is paired. An error after Keep kept it, as the
	// other stopped answering before it was told, or ErrNotKept, as Keep
	// took longer than the other waits, has Done drop what Keep kept, since
	// the other keeps nothing either.
	Done func(err error)
}

// Server is the computer controlled's: it pairs with the computers that
// ask, while its Host lets it, and runs the sessions of those paired.
type Server struct {
	Identity *Identity
	// Name is the computer's, which the others show.
	Name   string
	Host   Host
	Timing Timing

	mu    sync.Mutex
	ln    net.Listener
	conns map[*serverConn]bool
	// pairing is set while a pairing runs: one at a time. rested is when
	// the last code that didn't match was tried.
	pairing bool
	rested  time.Time
	closed  bool
}

// serverConn is a connection the server has, and the key of the computer
// at its other end.
type serverConn struct {
	conn net.Conn
	w    *wire
	peer Fingerprint
}

// Serve takes the connections ln accepts until Close, and returns nil
// then.
func (s *Server) Serve(ln net.Listener) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		ln.Close()
		return nil
	}
	s.ln = ln
	s.mu.Unlock()
	for {
		c, err := ln.Accept()
		if err != nil {
			s.mu.Lock()
			closed := s.closed
			s.mu.Unlock()
			if closed {
				return nil
			}
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				continue
			}
			return err
		}
		go s.handle(c)
	}
}

// Close stops listening, and closes every connection, telling the other
// computers that remote control is off.
func (s *Server) Close() error {
	s.mu.Lock()
	s.closed = true
	ln := s.ln
	conns := s.conns
	s.conns = nil
	s.mu.Unlock()
	for c := range conns {
		_ = c.w.send(message{Type: "refused", Reason: reasonOff}, 300*time.Millisecond)
		c.conn.Close()
	}
	if ln != nil {
		return ln.Close()
	}
	return nil
}

// Drop ends the sessions of the computer of the key fp, which is no longer
// paired: it is told so.
func (s *Server) Drop(fp Fingerprint) {
	s.mu.Lock()
	var drop []*serverConn
	for c := range s.conns {
		if c.peer == fp {
			drop = append(drop, c)
		}
	}
	s.mu.Unlock()
	for _, c := range drop {
		_ = c.w.send(message{Type: "refused", Reason: reasonNotPaired}, time.Second)
		c.conn.Close()
	}
}

// Sessions returns how many sessions and pairings run.
func (s *Server) Sessions() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.conns)
}

func (s *Server) tlsConfig() *tls.Config {
	return &tls.Config{
		Certificates: []tls.Certificate{s.Identity.cert},
		// Any key may connect: whether it may control, Host tells, or it
		// asks to pair.
		ClientAuth: tls.RequireAnyClientCert,
		MinVersion: tls.VersionTLS13,
		NextProtos: []string{alpn},
		VerifyConnection: func(cs tls.ConnectionState) error {
			if cs.NegotiatedProtocol != alpn {
				return errors.New("remote: not MixMusic's protocol")
			}
			_, err := peerKey(cs)
			return err
		},
	}
}

// handle runs a connection: TLS, then a hello, then a pairing or a
// session.
func (s *Server) handle(raw net.Conn) {
	defer raw.Close()
	t := s.Timing
	tc := tls.Server(raw, s.tlsConfig())
	ctx, cancel := context.WithTimeout(context.Background(), t.silence())
	err := tc.HandshakeContext(ctx)
	cancel()
	if err != nil {
		return
	}
	peer, err := peerKey(tc.ConnectionState())
	if err != nil {
		return
	}
	c := &serverConn{conn: tc, w: newWire(tc), peer: peer}
	if !s.add(c) {
		return
	}
	defer s.remove(c)
	hello, err := c.w.expect("hello", t.silence())
	if err != nil {
		return
	}
	if hello.Version != version {
		_ = c.w.send(message{Type: "refused", Reason: reasonVersion}, t.silence())
		return
	}
	switch hello.Want {
	case wantPair:
		s.pair(c, hello)
	case wantControl:
		if !s.Host.Allowed(peer) {
			_ = c.w.send(message{Type: "refused", Reason: reasonNotPaired}, t.silence())
			return
		}
		s.session(c)
	}
}

func (s *Server) add(c *serverConn) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false
	}
	if s.conns == nil {
		s.conns = map[*serverConn]bool{}
	}
	s.conns[c] = true
	return true
}

func (s *Server) remove(c *serverConn) {
	s.mu.Lock()
	delete(s.conns, c)
	s.mu.Unlock()
}

// pair runs a pairing with the computer at the other end of c, which said
// hello: one at a time, and none for a moment after a code that didn't
// match.
func (s *Server) pair(c *serverConn, hello message) {
	t := s.Timing
	name := cleanName(hello.Name)
	refuse := func(reason string) { _ = c.w.send(message{Type: "refused", Reason: reason}, t.silence()) }
	s.mu.Lock()
	if s.pairing || time.Since(s.rested) < t.rest() {
		s.mu.Unlock()
		refuse(reasonBusy)
		return
	}
	s.pairing = true
	s.mu.Unlock()
	// The pairing ends here before the other computer hears how it went:
	// that computer may start the next one at once, which must not find
	// this one still running, and be refused as busy.
	release := sync.OnceFunc(func() {
		s.mu.Lock()
		s.pairing = false
		s.mu.Unlock()
	})
	defer release()
	if t.linger > 0 {
		// Before the release deferred, so that the tests find a pairing
		// that only ends as its handler returns.
		defer time.Sleep(t.linger)
	}
	code, err := NewCode()
	if err != nil {
		return
	}
	value, _ := CodeValue(code)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	end, err := s.Host.Pairing(PairRequest{Name: name, Peer: c.peer, Code: code, Cancel: cancel})
	if err != nil {
		reason := reasonOff
		if errors.Is(err, ErrBusy) {
			reason = reasonBusy
		}
		release()
		refuse(reason)
		return
	}
	// finish tells the host how the pairing ended, once, and ends it here.
	var once sync.Once
	finish := func(err error) {
		once.Do(func() {
			if errors.Is(err, ErrWrongCode) {
				s.mu.Lock()
				s.rested = time.Now()
				s.mu.Unlock()
			}
			end.Done(err)
			release()
		})
	}
	// The user refusing it ends it, then closes the connection, which ends
	// the rounds.
	stop := context.AfterFunc(ctx, func() {
		finish(ErrCanceled)
		_ = c.w.send(message{Type: "failed", Reason: reasonCanceled}, time.Second)
		c.conn.Close()
	})
	ts, err := transcriptOf(c.conn.(*tls.Conn).ConnectionState(), c.peer, s.Identity.fp)
	if err == nil {
		err = c.w.send(message{Type: "code"}, t.silence())
	}
	if err == nil {
		err = pairing{w: c.w, t: ts, code: value, first: t.code(), wait: t.silence()}.host()
	}
	if err == nil {
		// The other computer checked the last round too.
		_, err = c.w.expect("confirm", t.silence())
	}
	if !stop() {
		err = ErrCanceled
	}
	if err != nil {
		finish(err)
		return
	}
	// Both proved the code: this computer keeps the other, and only then
	// tells it, which keeps this one as it hears. When that doesn't go,
	// neither keeps the other: kept later than the other waits, it isn't.
	began := time.Now()
	err = end.Keep()
	if err == nil && time.Since(began) > t.keepFor(hello.Features) {
		err = ErrNotKept
	}
	if err != nil {
		finish(err)
		_ = c.w.send(message{Type: "failed", Reason: reasonKeep}, t.silence())
		return
	}
	release()
	finish(c.w.send(message{Type: "paired", Name: s.Name}, t.silence()))
}

// keepFor is how long the computer controlled may take to keep a pairing
// with the computer whose hello told the features told: Keep, for one that
// waits for paired so long (FeatureKeep). An older one waits a silence for
// it, from when it sent confirm: half of it, the other half for confirm
// and paired on the way.
func (t Timing) keepFor(told []string) time.Duration {
	if slices.Contains(told, FeatureKeep) {
		return t.keep()
	}
	return t.silence() / 2
}

// session runs a session with the computer paired at the other end of c:
// it sends the state as it changes, and every Ping, and runs the commands
// it gets, until either side goes quiet for Silence.
func (s *Server) session(c *serverConn) {
	t := s.Timing
	if err := c.w.send(message{Type: "welcome", Name: s.Name, Features: features}, t.silence()); err != nil {
		return
	}
	changed := make(chan struct{}, 1)
	cancel := s.Host.Watch(func() {
		select {
		case changed <- struct{}{}:
		default:
		}
	})
	defer cancel()
	ended := make(chan struct{})
	go func() {
		defer close(ended)
		defer c.conn.Close()
		for {
			m, err := c.w.receive(t.silence())
			if err != nil {
				return
			}
			command := m.Type == "command" && m.Command != nil
			handOver := m.Type == "handover" && m.HandOver != nil
			if !command && !handOver {
				continue
			}
			// A computer removed a moment ago may still have a command
			// or a hand-over on the way.
			if !s.Host.Allowed(c.peer) {
				return
			}
			if handOver {
				if s.takeOver(c, *m.HandOver) != nil {
					return
				}
				continue
			}
			if cmd, ok := m.Command.clean(); ok {
				s.Host.Do(cmd)
			}
		}
	}()
	tick := time.NewTicker(t.ping())
	defer tick.Stop()
	for {
		st := cleanState(s.Host.State())
		if err := c.w.send(message{Type: "state", State: &st}, t.silence()); err != nil {
			c.conn.Close()
			<-ended
			return
		}
		sent := time.Now()
		select {
		case <-ended:
			return
		case <-tick.C:
		case <-changed:
			// What changes at once, as a seek and the play that follows,
			// goes in one state.
			select {
			case <-ended:
				return
			case <-time.After(time.Until(sent.Add(t.stateEvery()))):
			}
		}
	}
}
