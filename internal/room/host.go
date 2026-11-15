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

// Host is what the computer that hosts a room does for its server: it
// tells what its player plays.
type Host interface {
	// State is what the player plays now: where its track is heard, at the
	// time of the server's Clock.
	State() State
	// Queue is the player's queue around what plays.
	Queue() Queue
	// Watch has fn told, without waiting on it, as the state or the queue
	// changes, until cancel.
	Watch(fn func()) (cancel func())
}

// What the activity of a room tells of a guest.
const (
	Joined  = "joined"
	Left    = "left"
	Removed = "removed"
	Blocked = "blocked"
)

// Activity is something that happened in the room: a guest named Name
// joined, left, was removed or blocked.
type Activity struct {
	At   time.Time
	Name string
	What string
}

// maxActivity bounds the activity a room keeps.
const maxActivity = 100

// Member is a guest of the room: its key, its name, whether it is
// connected now, and when it joined. A guest that dropped off keeps its
// place a while, and comes back by its key.
type Member struct {
	Key    remote.Fingerprint
	Name   string
	Here   bool
	Joined time.Time
}

// Server is the host's: it lets in the guests that prove the room's code
// or its link's secret, tells them what plays, and starts each track for
// all of them at one moment, once they have it ready.
type Server struct {
	Identity *remote.Identity
	// Name is the host's name in rooms, which the guests show.
	Name string
	// Code is the room's code, and Secret its invite link's secret, as it
	// opens: SetCode changes them.
	Code   string
	Secret []byte
	Host   Host
	// Clock is the host's: the positions of its states are at its times.
	Clock  Clock
	Timing Timing
	// Changed, when set, is told, without waiting on it, as guests join,
	// leave, drop off and come back, or are removed.
	Changed func()

	mu      sync.Mutex
	ln      net.Listener
	started bool
	closed  bool
	conns   map[*conn]bool
	guests  map[remote.Fingerprint]*guest
	blocked map[remote.Fingerprint]string
	// adm counts the codes tried, and the connections that proved nothing
	// yet (admit.go); caps, for tests, are its bounds.
	adm      admission
	caps     *caps
	activity []Activity
	// seq counts the track changes, and gate is the one that waits for
	// the guests; starting is the state of its start, which the guests
	// are told until the host's track plays.
	seq      int64
	gate     *gate
	starting *State
	// queue is the queue the guests were told last, and sentQueue it as
	// sent.
	queue     *Queue
	sentQueue []byte
	wake      chan struct{}
	stopRun   chan struct{}
	unwatch   func()
}

// guest is a guest of the room, and its connection while it has one.
type guest struct {
	Member
	conn *conn
	away time.Time
}

// conn is a connection the server has: the key and the source of its
// other end (sourceOf), and the messages that wait to go to it.
type conn struct {
	raw  net.Conn
	f    *remote.Frames
	key  remote.Fingerprint
	src  string
	out  chan message
	once sync.Once
	gone chan struct{}
}

// outQueue is how many messages may wait to go to a guest: one slower
// than that is dropped, and comes back.
const outQueue = 64

// send queues m for c, or drops c when it is that far behind.
func (c *conn) send(m message) {
	select {
	case <-c.gone:
	case c.out <- m:
	default:
		c.close()
	}
}

func (c *conn) close() {
	c.once.Do(func() {
		close(c.gone)
		c.raw.Close()
	})
}

// write sends what is queued for c, until it closes.
func (c *conn) write(wait time.Duration) {
	for {
		select {
		case <-c.gone:
			return
		case m := <-c.out:
			if err := c.f.Send(m, wait); err != nil {
				c.close()
				return
			}
		}
	}
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
	s.adm.caps = s.caps.orDefault()
	s.start()
	s.mu.Unlock()
	for {
		raw, err := ln.Accept()
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
		// A connection that proves nothing yet waits for a place among
		// those the room holds, or is closed at once.
		src := sourceOf(raw.RemoteAddr().String())
		s.mu.Lock()
		ok := s.adm.letIn(src)
		s.mu.Unlock()
		if !ok {
			raw.Close()
			continue
		}
		go s.handle(raw, src)
	}
}

// start starts telling the guests what plays. s.mu is held.
func (s *Server) start() {
	if s.started {
		return
	}
	s.started = true
	s.wake = make(chan struct{}, 1)
	s.stopRun = make(chan struct{})
	s.unwatch = s.Host.Watch(func() {
		select {
		case s.wake <- struct{}{}:
		default:
		}
	})
	go s.run(s.wake, s.stopRun)
}

// Close ends the room: each guest is told so, and the server listens no
// more.
func (s *Server) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	ln, conns, unwatch, stop := s.ln, s.conns, s.unwatch, s.stopRun
	s.conns = nil
	if s.gate != nil {
		s.gate.timer.Stop()
		s.gate = nil
	}
	s.mu.Unlock()
	if unwatch != nil {
		unwatch()
	}
	if stop != nil {
		close(stop)
	}
	var wg sync.WaitGroup
	for c := range conns {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = c.f.Send(message{Type: "ended", Reason: reasonEnded}, 300*time.Millisecond)
			c.close()
		}()
	}
	wg.Wait()
	if ln != nil {
		return ln.Close()
	}
	return nil
}

// handle runs a connection of the source src: TLS, then a hello, then a
// guest's proof and stay, or its coming back. Until it proved the code,
// the link or its key, it holds a place among the unproven, and its
// messages are of a handshake's size.
func (s *Server) handle(raw net.Conn, src string) {
	defer raw.Close()
	proven := sync.OnceFunc(func() {
		s.mu.Lock()
		s.adm.proven(src)
		s.mu.Unlock()
	})
	defer proven()
	t := s.Timing
	tc := tls.Server(raw, s.Identity.ServerTLS(alpn))
	ctx, cancel := context.WithTimeout(context.Background(), t.silence())
	err := tc.HandshakeContext(ctx)
	cancel()
	if err != nil {
		return
	}
	key, err := remote.PeerKey(tc.ConnectionState())
	if err != nil {
		return
	}
	c := &conn{raw: tc, f: remote.NewFrames(tc), key: key, src: src, out: make(chan message, outQueue), gone: make(chan struct{})}
	c.f.Limit(handshakeMessage)
	if !s.add(c) {
		return
	}
	defer s.removeConn(c)
	defer c.close()
	var hello message
	if err := c.f.Receive(&hello, t.silence()); err != nil || hello.Type != "hello" {
		return
	}
	refuse := func(reason string) { _ = c.f.Send(message{Type: "refused", Reason: reason}, t.silence()) }
	if hello.Version != version {
		refuse(reasonVersion)
		return
	}
	name := CleanName(hello.Name)
	switch hello.Want {
	case wantRejoin:
		if reason := s.comeBack(c, name); reason != "" {
			refuse(reason)
			return
		}
		proven()
		c.f.Limit(0)
	case wantJoin:
		if !s.join(c, hello, name, refuse, proven) {
			return
		}
	default:
		return
	}
	go c.write(t.silence())
	s.stay(c)
}

func (s *Server) add(c *conn) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false
	}
	if s.conns == nil {
		s.conns = map[*conn]bool{}
	}
	s.conns[c] = true
	return true
}

// removeConn forgets c, and keeps the place of its guest, which dropped
// off, a while.
func (s *Server) removeConn(c *conn) {
	s.mu.Lock()
	delete(s.conns, c)
	g := s.guests[c.key]
	dropped := g != nil && g.conn == c
	if dropped {
		g.conn, g.Here, g.away = nil, false, time.Now()
	}
	gate := s.gate
	s.mu.Unlock()
	if dropped {
		if gate != nil {
			s.ready(c.key, gate.seq)
		}
		s.changed(true)
	}
}

// refusal is why the host refuses the key now, or "": blocked, or a
// room full of other guests.
func (s *Server) refusal(key remote.Fingerprint) string {
	if _, ok := s.blocked[key]; ok {
		return reasonBlocked
	}
	if _, ok := s.guests[key]; !ok && len(s.guests) >= MaxGuests {
		return reasonFull
	}
	return ""
}

// join runs a guest's proof of the code or the link's secret, then waits
// for it to join, as its user chooses to once they saw the room: proven is
// told once it proved it. It reports whether the guest joined.
func (s *Server) join(c *conn, hello message, name string, refuse func(string), proven func()) bool {
	t := s.Timing
	byCode := hello.By == byCode
	s.mu.Lock()
	password := []byte(s.Code)
	if hello.By == byLink {
		password = s.Secret
	}
	reason := s.refusal(c.key)
	if reason == "" && (!byCode && hello.By != byLink || len(password) == 0) {
		reason = reasonCode
	}
	// A try of a code counts before it runs, so that tries at once count.
	tried := false
	if reason == "" && byCode {
		switch s.adm.tryCode(c.src, time.Now(), t.wait()) {
		case tryRuns:
			tried = true
		case tryWaits:
			reason = reasonWait
		case tryBusy:
			reason = reasonBusy
		}
	}
	s.mu.Unlock()
	// endTry ends the try, once: one that ends before its code was
	// checked, as the guest went, tried none.
	ended := false
	endTry := func(wrong bool) {
		if !tried || ended {
			return
		}
		ended = true
		s.mu.Lock()
		s.adm.endTry(c.src, wrong, time.Now(), t.wait())
		s.mu.Unlock()
	}
	defer endTry(false)
	if reason != "" {
		refuse(reason)
		return false
	}
	cs := c.raw.(*tls.Conn).ConnectionState()
	aad, err := cs.ExportKeyingMaterial(exportLabel, nil, 32)
	if err != nil {
		return false
	}
	host := s.Identity.Fingerprint()
	sp, err := newSpake(false, passwordScalar(password), c.key[:], host[:])
	if err != nil {
		return false
	}
	keys, err := sp.finish(hello.Spake, aad)
	if err != nil {
		endTry(true)
		refuse(reasonCode)
		return false
	}
	if err := c.f.Send(message{Type: "spake", Spake: sp.msg}, t.silence()); err != nil {
		return false
	}
	var confirm message
	if err := c.f.Receive(&confirm, t.silence()); err != nil || confirm.Type != "confirm" {
		return false
	}
	if !hmac.Equal(confirm.MAC, keys.ca) {
		// One code tried, and wrong.
		endTry(true)
		refuse(reasonCode)
		return false
	}
	endTry(false)
	// Proven: its place among the unproven goes to another, and its
	// messages are a guest's.
	proven()
	c.f.Limit(0)
	preview := s.preview()
	if err := c.f.Send(message{Type: "welcome", MAC: keys.cb, Preview: &preview, Features: features}, t.silence()); err != nil {
		return false
	}
	// The guest's user sees the room, and joins it, or not: the guest
	// pings meanwhile.
	for {
		var m message
		if err := c.f.Receive(&m, t.silence()); err != nil {
			return false
		}
		switch m.Type {
		case "ping":
			if c.f.Send(message{Type: "pong", Sent: m.Sent, HostAt: s.Clock.Now()}, t.silence()) != nil {
				return false
			}
			continue
		case "join":
		default:
			continue
		}
		st := s.Host.State()
		s.mu.Lock()
		if reason := s.refusal(c.key); reason != "" || s.closed {
			s.mu.Unlock()
			refuse(reason)
			return false
		}
		s.admit(c, name, st)
		s.mu.Unlock()
		s.changed(true)
		return true
	}
}

// exportLabel is the label of the bytes a room's TLS connection exports
// (RFC 8446, 7.5), which bind its SPAKE2 to it.
const exportLabel = "EXPORTER-MixMusic-room"

// comeBack lets a guest whose place the room keeps back in, by its key,
// or returns why not.
func (s *Server) comeBack(c *conn, name string) string {
	st := s.Host.State()
	s.mu.Lock()
	if reason := s.refusal(c.key); reason != "" {
		s.mu.Unlock()
		return reason
	}
	g := s.guests[c.key]
	if g == nil {
		s.mu.Unlock()
		return reasonGone
	}
	if g.conn != nil {
		// The connection it had is gone, though the host doesn't know it
		// yet.
		g.conn.close()
	}
	s.admit(c, name, st)
	s.mu.Unlock()
	s.changed(true)
	return ""
}

// admit makes c the connection of its guest, a new one or one that came
// back, and tells it the room, where the host plays st. s.mu is held.
func (s *Server) admit(c *conn, name string, st State) {
	if s.guests == nil {
		s.guests = map[remote.Fingerprint]*guest{}
	}
	g := s.guests[c.key]
	if g == nil {
		g = &guest{Member: Member{Key: c.key, Joined: time.Now()}}
		s.guests[c.key] = g
		s.note(name, Joined)
	}
	g.Name, g.Here, g.conn, g.away = name, true, c, time.Time{}
	c.send(message{Type: "joined", Name: s.Name, Features: features})
	c.send(message{Type: "members", Members: s.memberNames()})
	if s.queue != nil {
		c.send(s.queueMessage())
	}
	st = s.stateOf(st)
	c.send(message{Type: "state", State: &st})
}

// note keeps an activity. s.mu is held.
func (s *Server) note(name, what string) {
	s.activity = append(s.activity, Activity{At: time.Now(), Name: name, What: what})
	if len(s.activity) > maxActivity {
		s.activity = slices.Delete(s.activity, 0, len(s.activity)-maxActivity)
	}
}

// stay runs a guest's stay in the room: it answers its pings, takes in
// that it has a track ready, and lets it leave, until either side goes
// quiet.
func (s *Server) stay(c *conn) {
	t := s.Timing
	for {
		var m message
		if err := c.f.Receive(&m, t.silence()); err != nil {
			return
		}
		switch m.Type {
		case "ping":
			c.send(message{Type: "pong", Sent: m.Sent, HostAt: s.Clock.Now()})
		case "ready":
			s.ready(c.key, m.Seq)
		case "leave":
			s.mu.Lock()
			g := s.guests[c.key]
			if g != nil && g.conn == c {
				delete(s.guests, c.key)
				s.note(g.Name, Left)
			}
			s.mu.Unlock()
			s.changed(true)
			return
		}
	}
}

// changed tells the guests and the app that the guests changed.
func (s *Server) changed(members bool) {
	if members {
		s.mu.Lock()
		names := s.memberNames()
		for _, g := range s.guests {
			if g.conn != nil {
				g.conn.send(message{Type: "members", Members: names})
			}
		}
		s.mu.Unlock()
	}
	if fn := s.Changed; fn != nil {
		fn()
	}
}

// memberNames are the names of the guests, in the order they joined, the
// host's first. s.mu is held.
func (s *Server) memberNames() []string {
	names := []string{CleanName(s.Name)}
	for _, m := range s.members() {
		names = append(names, m.Name)
	}
	return names
}

// members are the guests in the order they joined. s.mu is held.
func (s *Server) members() []Member {
	out := make([]Member, 0, len(s.guests))
	for _, g := range s.guests {
		out = append(out, g.Member)
	}
	slices.SortFunc(out, func(a, b Member) int { return a.Joined.Compare(b.Joined) })
	return out
}

// Members returns the guests of the room, in the order they joined.
func (s *Server) Members() []Member {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.members()
}

// Listeners returns how many guests listen now.
func (s *Server) Listeners() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listeners()
}

func (s *Server) listeners() int {
	n := 0
	for _, g := range s.guests {
		if g.conn != nil {
			n++
		}
	}
	return n
}

// Activity returns what happened in the room, the latest last.
func (s *Server) Activity() []Activity {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.activity)
}

// Blocked returns the computers the host blocked, by key, with their names.
func (s *Server) Blocked() []Member {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Member, 0, len(s.blocked))
	for k, name := range s.blocked {
		out = append(out, Member{Key: k, Name: name})
	}
	slices.SortFunc(out, func(a, b Member) int { return compareKeys(a.Key, b.Key) })
	return out
}

func compareKeys(a, b remote.Fingerprint) int {
	for i := range a {
		if a[i] != b[i] {
			return int(a[i]) - int(b[i])
		}
	}
	return 0
}

// SetCode gives the room a new code and a new secret for its invite link:
// the old ones let no one in from then on, a computer the host removed or
// blocked under another key too. The guests stay.
func (s *Server) SetCode(code string, secret []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Code, s.Secret = code, secret
}

// Remove removes the guest of key from the room: it is told so, and joins
// again only by the code or the link.
func (s *Server) Remove(key remote.Fingerprint) { s.drop(key, reasonRemoved, Removed) }

// Block removes the guest of key, and keeps it out, by its key, until
// Unblock.
func (s *Server) Block(key remote.Fingerprint) {
	s.mu.Lock()
	if s.blocked == nil {
		s.blocked = map[remote.Fingerprint]string{}
	}
	name := "Someone"
	if g := s.guests[key]; g != nil {
		name = g.Name
	}
	s.blocked[key] = name
	s.mu.Unlock()
	s.drop(key, reasonBlocked, Blocked)
}

// Unblock lets the computer of key join again, by the code or the link.
func (s *Server) Unblock(key remote.Fingerprint) {
	s.mu.Lock()
	delete(s.blocked, key)
	s.mu.Unlock()
	s.changed(false)
}

// drop ends the stay of the guest of key, telling it why.
func (s *Server) drop(key remote.Fingerprint, reason, what string) {
	s.mu.Lock()
	g := s.guests[key]
	if g == nil {
		s.mu.Unlock()
		s.changed(false)
		return
	}
	delete(s.guests, key)
	s.note(g.Name, what)
	c := g.conn
	gate := s.gate
	s.mu.Unlock()
	if c != nil {
		go func() {
			_ = c.f.Send(message{Type: "ended", Reason: reason}, time.Second)
			c.close()
		}()
	}
	if gate != nil {
		s.ready(key, gate.seq)
	}
	s.changed(true)
}

// preview is what the room shows a guest before it joins.
func (s *Server) preview() Preview {
	st := s.Host.State()
	p := Preview{Host: s.Name, Listeners: s.Listeners()}
	if st.Track != nil {
		p.Title = st.Track.Title
		for i, a := range st.Track.Artists {
			if i > 0 {
				p.Artists += ", "
			}
			p.Artists += a.Name
		}
	}
	return p.clean()
}
