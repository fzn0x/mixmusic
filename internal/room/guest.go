package room

import (
	"context"
	"crypto/hmac"
	"crypto/tls"
	"errors"
	"net"
	"slices"
	"sync"
	"time"

	"github.com/fzn0x/mixmusic/internal/remote"
)

// Guest is the computer that joins rooms: it proves a room's code or its
// link's secret, then follows what the host plays.
type Guest struct {
	Identity *remote.Identity
	// Name is the guest's name in rooms, which the host and the other
	// guests show.
	Name string
	// Clock is the guest's: its offset from the host's is measured against
	// it.
	Clock  Clock
	Timing Timing
	// Dial connects to an address, as net.Dialer does when nil.
	Dial func(ctx context.Context, network, addr string) (net.Conn, error)
	// Elsewhere, when set, tells where else the room is reached, as its
	// invite link does: a connection to a pinned host tries those too
	// (dial.go).
	Elsewhere func() []string
}

// connect connects to the room at addr over TLS, and returns the
// connection and the host's key, which must be pinned, unless pinned is
// nil.
func (g *Guest) connect(ctx context.Context, addr string, pinned *remote.Fingerprint) (*tls.Conn, remote.Fingerprint, error) {
	if g.Elsewhere != nil {
		return g.connectFirst(ctx, g.addrsOf(addr, pinned), pinned)
	}
	dial := g.Dial
	if dial == nil {
		dial = (&net.Dialer{}).DialContext
	}
	ctx, cancel := context.WithTimeout(ctx, g.Timing.silence())
	defer cancel()
	raw, err := dial(ctx, "tcp", addr)
	if err != nil {
		return nil, remote.Fingerprint{}, err
	}
	var host remote.Fingerprint
	tc := tls.Client(raw, g.Identity.ClientTLS(alpn, pinned, &host))
	if err := tc.HandshakeContext(ctx); err != nil {
		raw.Close()
		if errors.Is(err, remote.ErrNotSame) {
			return nil, host, ErrNotSame
		}
		return nil, host, err
	}
	if tc.ConnectionState().NegotiatedProtocol != alpn {
		tc.Close()
		return nil, host, errors.New("room: not MixMusic's protocol")
	}
	return tc, host, nil
}

// Knock connects to the room at addr and proves the room's code, or its
// invite link's secret, when link is set: its host's key must be pinned,
// unless pinned is nil, as a code tells no key. The session it returns
// shows the room (Preview), and Join joins it.
func (g *Guest) Knock(ctx context.Context, addr string, pinned *remote.Fingerprint, password []byte, link bool) (*Session, error) {
	t := g.Timing
	tc, host, err := g.connect(ctx, addr, pinned)
	if err != nil {
		return nil, err
	}
	stop := context.AfterFunc(ctx, func() { tc.Close() })
	defer stop()
	fail := func(err error) (*Session, error) {
		tc.Close()
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, err
	}
	cs := tc.ConnectionState()
	aad, err := cs.ExportKeyingMaterial(exportLabel, nil, 32)
	if err != nil {
		return fail(err)
	}
	me := g.Identity.Fingerprint()
	sp, err := newSpake(true, passwordScalar(password), me[:], host[:])
	if err != nil {
		return fail(err)
	}
	f := remote.NewFrames(tc)
	// Until the host proved the code, its messages are of a handshake's
	// size: at an address typed in, anyone may answer.
	f.Limit(handshakeMessage)
	by := byCode
	if link {
		by = byLink
	}
	hello := message{Type: "hello", Want: wantJoin, By: by, Name: CleanName(g.Name), Version: version, Features: features, Spake: sp.msg}
	if err := f.Send(hello, t.silence()); err != nil {
		return fail(err)
	}
	theirs, err := expect(f, "spake", t.silence())
	if err != nil {
		return fail(err)
	}
	keys, err := sp.finish(theirs.Spake, aad)
	if err != nil {
		return fail(ErrWrongCode)
	}
	if err := f.Send(message{Type: "confirm", MAC: keys.ca}, t.silence()); err != nil {
		return fail(err)
	}
	welcome, err := expect(f, "welcome", t.silence())
	if err != nil {
		return fail(err)
	}
	if !hmac.Equal(welcome.MAC, keys.cb) {
		// The host let in a code it doesn't know: it isn't the room's.
		return fail(ErrWrongCode)
	}
	f.Limit(0)
	s := g.session(tc, f, host)
	if welcome.Preview != nil {
		s.preview = welcome.Preview.clean()
	}
	s.start()
	return s, nil
}

// Rejoin connects to the room of the host of key at addr again, where the
// guest joined before: the host lets it back in by its key while it keeps
// its place.
func (g *Guest) Rejoin(ctx context.Context, addr string, host remote.Fingerprint) (*Session, error) {
	t := g.Timing
	tc, _, err := g.connect(ctx, addr, &host)
	if err != nil {
		return nil, err
	}
	f := remote.NewFrames(tc)
	if err := f.Send(message{Type: "hello", Want: wantRejoin, Name: CleanName(g.Name), Version: version, Features: features}, t.silence()); err != nil {
		tc.Close()
		return nil, err
	}
	joined, err := expect(f, "joined", t.silence(), "asking")
	if err != nil {
		tc.Close()
		return nil, err
	}
	s := g.session(tc, f, host)
	if joined.Type == "asking" {
		// The room moved here, and its host's user lets this computer in
		// first (letin.go).
		return s.letIn()
	}
	s.joined, s.preview.Host = true, CleanName(joined.Name)
	s.start()
	return s, nil
}

// expect receives the next message, which must be of type typ, or of one
// of or: a refused or ended message is its reason's error.
func expect(f *remote.Frames, typ string, wait time.Duration, or ...string) (message, error) {
	var m message
	if err := f.Receive(&m, wait); err != nil {
		return m, err
	}
	if slices.Contains(or, m.Type) {
		return m, nil
	}
	switch m.Type {
	case typ:
		return m, nil
	case "refused", "ended":
		return m, errOf(m.Reason)
	}
	return m, errors.New("room: a " + m.Type + " message where " + typ + " was due")
}

func (g *Guest) session(tc *tls.Conn, f *remote.Frames, host remote.Fingerprint) *Session {
	clock := g.Clock
	if clock == nil {
		clock = NewSystemClock()
	}
	return &Session{conn: tc, f: f, host: host, clock: clock, timing: g.Timing, done: make(chan struct{}), joinedCh: make(chan struct{})}
}

// EventKind is what changed in a session.
type EventKind int

const (
	// StateChanged: the host told where its track is.
	StateChanged EventKind = iota
	// QueueChanged: the host's queue changed.
	QueueChanged
	// MembersChanged: who listens changed.
	MembersChanged
	// Prepare: the host's next track, Track of Entry, which the guest
	// readies, then tells so (Ready), for the change Seq.
	Prepare
	// Ended: the session ended (Err).
	Ended
)

// Event is what changed in a session.
type Event struct {
	Kind  EventKind
	Seq   int64
	Entry string
	Track remote.QueueTrack
	// Reason, Seats, Key and Addr are of the host's rules and of a move
	// (rules.go, move.go).
	Reason string
	Seats  Seats
	Key    remote.Fingerprint
	Addr   string
}

// Session is a guest's stay in a room: what the host plays, and who
// listens. It ends as the host goes quiet, or ends it, or Close.
type Session struct {
	conn   net.Conn
	f      *remote.Frames
	host   remote.Fingerprint
	clock  Clock
	timing Timing

	mu       sync.Mutex
	preview  Preview
	joined   bool
	joinedCh chan struct{}
	state    State
	hasState bool
	queue    Queue
	members  []string
	offset   Offset
	// asks are what the host tells beside what it plays (rules.go).
	asks     sessionAsks
	watchers []*func(Event)
	err      error
	done     chan struct{}
	once     sync.Once
}

// Host returns the key of the room's host.
func (s *Session) Host() remote.Fingerprint { return s.host }

// Preview returns what the room shows: who hosts, what plays and how many
// listen, as the host told it before the guest joined.
func (s *Session) Preview() Preview {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.preview
}

// Join joins the room the guest knocked at, and returns once the host let
// it in.
func (s *Session) Join(ctx context.Context) error {
	s.mu.Lock()
	joined := s.joined
	s.mu.Unlock()
	if joined {
		return nil
	}
	if err := s.f.Send(message{Type: "join"}, s.timing.silence()); err != nil {
		s.end(ErrLost)
		return err
	}
	select {
	case <-s.joinedCh:
		return nil
	case <-s.done:
		return s.Err()
	case <-ctx.Done():
		return ctx.Err()
	}
}

// State returns where the host's track is, as it told last, and whether it
// told yet.
func (s *Session) State() (State, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state, s.hasState
}

// Queue returns the host's queue, as it told last.
func (s *Session) Queue() Queue {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.queue
}

// Members returns the names of who listens, the host's first.
func (s *Session) Members() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.members)
}

// Offset returns how far the host's clock is ahead of the guest's, and
// whether a ping came back yet to tell.
func (s *Session) Offset() (time.Duration, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	off, _, ok := s.offset.Get()
	return off, ok
}

// RoundTrip returns the round trip of the ping whose answer Offset takes,
// within half of which the estimate is.
func (s *Session) RoundTrip() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, rtt, _ := s.offset.Get()
	return rtt
}

// Now is the guest's clock's time.
func (s *Session) Now() time.Duration { return s.clock.Now() }

// Ready tells the host that the guest has the track of the change seq
// ready, or can't play it.
func (s *Session) Ready(seq int64) error {
	return s.f.Send(message{Type: "ready", Seq: seq}, s.timing.silence())
}

// Watch has fn told, from the session's goroutine, as something changes,
// until cancel. fn must return quickly.
func (s *Session) Watch(fn func(Event)) (cancel func()) {
	f := &fn
	s.mu.Lock()
	s.watchers = append(slices.Clone(s.watchers), f)
	s.mu.Unlock()
	return func() {
		s.mu.Lock()
		s.watchers = slices.DeleteFunc(slices.Clone(s.watchers), func(g *func(Event)) bool { return g == f })
		s.mu.Unlock()
	}
}

func (s *Session) tell(e Event) {
	s.mu.Lock()
	fns := s.watchers
	s.mu.Unlock()
	for _, fn := range fns {
		(*fn)(e)
	}
}

// Done is closed once the session ended.
func (s *Session) Done() <-chan struct{} { return s.done }

// Err returns why the session ended: ErrEnded once the host ended the
// room, ErrRemoved or ErrBlocked once it removed this guest, ErrLost for a
// host that went quiet, net.ErrClosed after Close or Leave.
func (s *Session) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

// Leave leaves the room: the host no longer keeps the guest's place.
func (s *Session) Leave() {
	_ = s.f.Send(message{Type: "leave"}, time.Second)
	s.end(net.ErrClosed)
}

// Close ends the session; the host keeps the guest's place a while.
func (s *Session) Close() { s.end(net.ErrClosed) }

func (s *Session) end(err error) {
	s.once.Do(func() {
		s.mu.Lock()
		s.err = err
		s.mu.Unlock()
		s.conn.Close()
		close(s.done)
		s.tell(Event{Kind: Ended})
	})
}

// start reads what the host sends, and pings it.
func (s *Session) start() {
	go s.read()
	go s.ping()
}

func (s *Session) read() {
	for {
		var m message
		err := s.f.Receive(&m, s.timing.silence())
		if err != nil {
			select {
			case <-s.done:
			default:
				s.end(ErrLost)
			}
			return
		}
		switch m.Type {
		case "joined":
			s.mu.Lock()
			if !s.joined {
				s.joined = true
				close(s.joinedCh)
			}
			s.mu.Unlock()
		case "state":
			if m.State == nil {
				continue
			}
			st := m.State.clean()
			s.mu.Lock()
			s.state, s.hasState = st, true
			s.mu.Unlock()
			s.tell(Event{Kind: StateChanged})
		case "queue":
			if m.Queue == nil {
				continue
			}
			q := m.Queue.clean()
			s.mu.Lock()
			s.queue = q
			s.mu.Unlock()
			s.tell(Event{Kind: QueueChanged})
		case "members":
			names := cleanMembers(m.Members)
			s.mu.Lock()
			s.members = names
			s.mu.Unlock()
			s.tell(Event{Kind: MembersChanged})
		case "prepare":
			if m.Track == nil {
				continue
			}
			s.tell(Event{Kind: Prepare, Seq: m.Seq, Entry: cleanEntry(m.Entry), Track: m.Track.Clean()})
		case "pong":
			now := s.clock.Now()
			s.mu.Lock()
			s.offset.Add(m.Sent, m.HostAt, now)
			s.mu.Unlock()
		case "ended", "refused":
			s.end(errOf(m.Reason))
			return
		default:
			s.fromHost(m)
		}
	}
}

// firstPings is how many pings a session sends at once, 100 ms apart, so
// that it soon knows the host's clock.
const firstPings = 5

func (s *Session) ping() {
	send := func() bool {
		return s.f.Send(message{Type: "ping", Sent: s.clock.Now()}, s.timing.silence()) == nil
	}
	for i := range firstPings {
		if !send() {
			s.end(ErrLost)
			return
		}
		if i < firstPings-1 {
			select {
			case <-s.done:
				return
			case <-time.After(100 * time.Millisecond):
			}
		}
	}
	tick := time.NewTicker(s.timing.ping())
	defer tick.Stop()
	for {
		select {
		case <-s.done:
			return
		case <-tick.C:
			if !send() {
				s.end(ErrLost)
				return
			}
		}
	}
}
