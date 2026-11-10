package remote

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"reflect"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/dnssd"
)

// fast is the timing of the tests: short, but not so short that a busy
// runner takes a computer for gone.
var fast = Timing{Ping: 100 * time.Millisecond, Silence: 2 * time.Second, Code: 5 * time.Second,
	StateEvery: 10 * time.Millisecond, Rest: 200 * time.Millisecond}

// fakeHost is the computer controlled's side of the app: who may control
// it, the codes it shows, and a player that keeps the commands it gets.
type fakeHost struct {
	mu       sync.Mutex
	allowed  map[Fingerprint]bool
	off      bool
	keepErr  error
	state    State
	commands []Command
	watchers []*func()
	// shown gets the request of each pairing as its code shows, and ended
	// how each ended.
	shown chan PairRequest
	ended chan error
}

func newFakeHost() *fakeHost {
	return &fakeHost{allowed: map[Fingerprint]bool{}, shown: make(chan PairRequest, 10), ended: make(chan error, 10)}
}

func (h *fakeHost) Allowed(fp Fingerprint) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.allowed[fp]
}

func (h *fakeHost) Pairing(req PairRequest) (func(error) error, error) {
	h.mu.Lock()
	off := h.off
	h.mu.Unlock()
	if off {
		return nil, ErrOff
	}
	h.shown <- req
	return func(err error) error {
		h.mu.Lock()
		defer h.mu.Unlock()
		if err == nil && h.keepErr == nil {
			h.allowed[req.Peer] = true
		}
		h.ended <- err
		return h.keepErr
	}, nil
}

func (h *fakeHost) State() State {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.state
}

func (h *fakeHost) Do(c Command) {
	h.mu.Lock()
	h.commands = append(h.commands, c)
	h.mu.Unlock()
}

func (h *fakeHost) Watch(fn func()) func() {
	f := &fn
	h.mu.Lock()
	h.watchers = append(h.watchers, f)
	h.mu.Unlock()
	return func() {
		h.mu.Lock()
		h.watchers = slices.DeleteFunc(h.watchers, func(g *func()) bool { return g == f })
		h.mu.Unlock()
	}
}

func (h *fakeHost) setState(st State) {
	h.mu.Lock()
	h.state = st
	fns := slices.Clone(h.watchers)
	h.mu.Unlock()
	for _, fn := range fns {
		(*fn)()
	}
}

func (h *fakeHost) revoke(fp Fingerprint) {
	h.mu.Lock()
	delete(h.allowed, fp)
	h.mu.Unlock()
}

func (h *fakeHost) got() []Command {
	h.mu.Lock()
	defer h.mu.Unlock()
	return slices.Clone(h.commands)
}

// serve runs a server of h on a port of 127.0.0.1 until the test ends.
func serve(t *testing.T, h Host, timing Timing) (*Server, string) {
	t.Helper()
	id, err := NewIdentity()
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Identity: id, Name: "Desktop", Host: h, Timing: timing}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- s.Serve(ln) }()
	t.Cleanup(func() {
		s.Close()
		if err := <-done; err != nil {
			t.Errorf("serving: %v", err)
		}
	})
	return s, ln.Addr().String()
}

func newDialer(t *testing.T, name string, timing Timing) *Dialer {
	t.Helper()
	id, err := NewIdentity()
	if err != nil {
		t.Fatal(err)
	}
	return &Dialer{Identity: id, Name: name, Timing: timing}
}

// pair pairs d with the computer at addr, whose host h shows the code: the
// user types what typed makes of it.
func pair(t *testing.T, d *Dialer, addr string, h *fakeHost, typed func(code string) string) (Peer, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return d.Pair(ctx, addr, func(ctx context.Context) (string, error) {
		select {
		case req := <-h.shown:
			return typed(req.Code), nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	})
}

func same(code string) string { return code }

// waitFor waits for ok, failing the test after a few seconds with what was
// seen.
func waitFor(t *testing.T, what string, ok func() bool, seen func() any) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); !ok(); time.Sleep(5 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("waiting for %s; seen %+v", what, seen())
		}
	}
}

func connect(t *testing.T, d *Dialer, addr string, key Fingerprint) *Session {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := d.Connect(ctx, addr, key)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

// TestPairAndControl pairs with the code shown, then sees what plays there
// as it changes, and controls it.
func TestPairAndControl(t *testing.T) {
	h := newFakeHost()
	s, addr := serve(t, h, fast)
	d := newDialer(t, "Laptop", fast)
	peer, err := pair(t, d, addr, h, same)
	if err != nil {
		t.Fatal(err)
	}
	if peer.Key != s.Identity.Fingerprint() || peer.Name != "Desktop" {
		t.Errorf("paired with %+v", peer)
	}
	if err := <-h.ended; err != nil {
		t.Errorf("the pairing ended with %v there", err)
	}
	if !h.Allowed(d.Identity.Fingerprint()) {
		t.Fatal("the computer controlled didn't keep the one paired")
	}

	h.setState(State{Track: &Track{Title: "Teardrop", Artists: []string{"Massive Attack"}, Artwork: "https://example.com/a.jpg"},
		Playing: true, PositionMs: 1000, DurationMs: 330000, Volume: 0.5})
	sess := connect(t, d, addr, peer.Key)
	if sess.Name() != "Desktop" {
		t.Errorf("session of %q", sess.Name())
	}
	state := func() any { st, _, _ := sess.State(); return st }
	waitFor(t, "the state", func() bool {
		st, _, ok := sess.State()
		return ok && st.Track != nil && st.Track.Title == "Teardrop" && st.Playing
	}, state)

	for _, c := range []Command{{Op: OpPause}, {Op: OpSeek, PositionMs: 61000}, {Op: OpVolume, Volume: 1.5}, {Op: OpNext}, {Op: OpPrevious}, {Op: OpPlay}} {
		if err := sess.Do(c); err != nil {
			t.Fatal(err)
		}
	}
	want := []Command{{Op: OpPause}, {Op: OpSeek, PositionMs: 61000}, {Op: OpVolume, Volume: 1}, {Op: OpNext}, {Op: OpPrevious}, {Op: OpPlay}}
	waitFor(t, "the commands", func() bool { return reflect.DeepEqual(h.got(), want) }, func() any { return h.got() })
	if err := sess.Do(Command{Op: "format the disk"}); err == nil {
		t.Error("a command the protocol hasn't was sent")
	}

	h.setState(State{Track: &Track{Title: "Angel"}, Volume: 0.5})
	waitFor(t, "the next state", func() bool {
		st, _, _ := sess.State()
		return st.Track != nil && st.Track.Title == "Angel" && !st.Playing
	}, state)
}

// TestWrongCode fails a pairing with a code other than the one shown,
// whichever bit differs, on both computers; rests a moment after; and
// shows a new code each time, so that the one shown before is no good.
func TestWrongCode(t *testing.T) {
	h := newFakeHost()
	_, addr := serve(t, h, fast)
	d := newDialer(t, "Laptop", fast)
	for _, bit := range []uint32{0, 9, 19} {
		var shown string
		_, err := pair(t, d, addr, h, func(code string) string {
			shown = code
			v, _ := CodeValue(code)
			v ^= 1 << bit
			if v >= 1_000_000 {
				v ^= 1<<bit | 1<<18
			}
			return fmt.Sprintf("%03d %03d", v/1000, v%1000)
		})
		if !errors.Is(err, ErrWrongCode) {
			t.Fatalf("bit %d: paired with a wrong code: %v", bit, err)
		}
		if err := <-h.ended; !errors.Is(err, ErrWrongCode) {
			t.Errorf("bit %d: the pairing ended with %v there", bit, err)
		}
		if h.Allowed(d.Identity.Fingerprint()) {
			t.Fatalf("bit %d: kept as paired", bit)
		}
		// The computer rests a moment, and shows no code meanwhile.
		if _, err := pair(t, d, addr, h, same); !errors.Is(err, ErrBusy) {
			t.Errorf("bit %d: paired again at once: %v", bit, err)
		}
		time.Sleep(fast.Rest)
		// The next code is another: the one shown before doesn't pair.
		_, err = pair(t, d, addr, h, func(code string) string {
			if code == shown {
				t.Skip("the same code came twice, once in a million times")
			}
			return shown
		})
		if !errors.Is(err, ErrWrongCode) {
			t.Fatalf("bit %d: paired with the code shown before: %v", bit, err)
		}
		<-h.ended
		time.Sleep(fast.Rest)
	}
	if _, err := pair(t, d, addr, h, func(string) string { return "12345" }); !errors.Is(err, ErrWrongCode) {
		t.Errorf("a code of 5 digits: %v", err)
	}
	<-h.ended
}

// TestRefused refuses a computer not paired, and a pairing while remote
// control is off; and a pairing whose user cancels it ends on both.
func TestRefused(t *testing.T) {
	h := newFakeHost()
	s, addr := serve(t, h, fast)
	d := newDialer(t, "Laptop", fast)
	ctx := context.Background()
	if _, err := d.Connect(ctx, addr, s.Identity.Fingerprint()); !errors.Is(err, ErrNotPaired) {
		t.Errorf("a computer not paired connected: %v", err)
	}
	h.mu.Lock()
	h.off = true
	h.mu.Unlock()
	if _, err := pair(t, d, addr, h, same); !errors.Is(err, ErrOff) {
		t.Errorf("paired while off: %v", err)
	}
	h.mu.Lock()
	h.off = false
	h.mu.Unlock()
	_, err := d.Pair(ctx, addr, func(ctx context.Context) (string, error) {
		req := <-h.shown
		req.Cancel()
		<-ctx.Done()
		return "", ctx.Err()
	})
	if !errors.Is(err, ErrCanceled) {
		t.Errorf("a pairing canceled there ended with %v", err)
	}
	if err := <-h.ended; !errors.Is(err, ErrCanceled) {
		t.Errorf("the pairing canceled ended with %v there", err)
	}
	// A computer that can't keep the pairing says so.
	h.mu.Lock()
	h.keepErr = errors.New("the disk is full")
	h.mu.Unlock()
	if _, err := pair(t, d, addr, h, same); err == nil {
		t.Error("paired with a computer that couldn't keep it")
	}
	<-h.ended
}

// TestRevoked ends the session of a computer removed at once, and refuses
// it after.
func TestRevoked(t *testing.T) {
	h := newFakeHost()
	s, addr := serve(t, h, fast)
	d := newDialer(t, "Laptop", fast)
	peer, err := pair(t, d, addr, h, same)
	if err != nil {
		t.Fatal(err)
	}
	<-h.ended
	sess := connect(t, d, addr, peer.Key)
	waitFor(t, "the state", func() bool { _, _, ok := sess.State(); return ok }, func() any { return sess.Err() })
	h.revoke(d.Identity.Fingerprint())
	s.Drop(d.Identity.Fingerprint())
	select {
	case <-sess.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("the session of a computer removed went on")
	}
	if !errors.Is(sess.Err(), ErrNotPaired) {
		t.Errorf("the session ended with %v", sess.Err())
	}
	if _, err := d.Connect(context.Background(), addr, peer.Key); !errors.Is(err, ErrNotPaired) {
		t.Errorf("a computer removed connected again: %v", err)
	}
	if err := sess.Do(Command{Op: OpPlay}); err == nil {
		t.Error("a command went in a session that ended")
	}
	if got := h.got(); len(got) != 0 {
		t.Errorf("commands ran: %+v", got)
	}
}

// TestNotTheSame refuses a computer that answers with another key where the
// one paired was.
func TestNotTheSame(t *testing.T) {
	h := newFakeHost()
	_, addr := serve(t, h, fast)
	d := newDialer(t, "Laptop", fast)
	peer, err := pair(t, d, addr, h, same)
	if err != nil {
		t.Fatal(err)
	}
	<-h.ended
	other := newFakeHost()
	other.allowed[d.Identity.Fingerprint()] = true
	_, impostor := serve(t, other, fast)
	if _, err := d.Connect(context.Background(), impostor, peer.Key); !errors.Is(err, ErrNotSame) {
		t.Errorf("connected to another computer as the one paired: %v", err)
	}
}

// TestBetween fails a pairing through someone between the two computers,
// who passes on all that each sends, the code typed right.
func TestBetween(t *testing.T) {
	h := newFakeHost()
	_, addr := serve(t, h, fast)
	d := newDialer(t, "Laptop", fast)
	mid, err := NewIdentity()
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		raw, err := ln.Accept()
		if err != nil {
			return
		}
		defer raw.Close()
		tc := tls.Server(raw, (&Server{Identity: mid}).tlsConfig())
		if tc.Handshake() != nil {
			return
		}
		toController := newWire(tc)
		hello, err := toController.expect("hello", fast.silence())
		if err != nil {
			return
		}
		md := &Dialer{Identity: mid, Timing: fast}
		hc, _, err := md.connect(context.Background(), addr, nil)
		if err != nil {
			return
		}
		defer hc.Close()
		toHost := newWire(hc)
		if toHost.send(hello, time.Second) != nil {
			return
		}
		pass := func(from, to *wire) {
			for {
				m, err := from.receive(fast.Code)
				if err != nil || to.send(m, time.Second) != nil {
					raw.Close()
					hc.Close()
					return
				}
			}
		}
		go pass(toHost, toController)
		pass(toController, toHost)
	}()
	_, err = pair(t, d, ln.Addr().String(), h, same)
	if !errors.Is(err, ErrWrongCode) {
		t.Errorf("paired through someone between: %v", err)
	}
	if err := <-h.ended; !errors.Is(err, ErrWrongCode) {
		t.Errorf("the pairing ended with %v there", err)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.allowed) != 0 {
		t.Errorf("kept %d computers as paired", len(h.allowed))
	}
}

// relay is a TCP relay between a controller and the computer at to, on
// 127.0.0.1: it keeps what the first connection's controller sent, can
// send its next record twice, and can stop passing anything on, as a
// network that drops all.
type relay struct {
	ln      net.Listener
	to      string
	mu      sync.Mutex
	sent    []byte
	first   bool
	twice   bool
	dropped bool
	conns   []net.Conn
}

func newRelay(t *testing.T, to string) *relay {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	r := &relay{ln: ln, to: to}
	t.Cleanup(func() {
		ln.Close()
		r.mu.Lock()
		defer r.mu.Unlock()
		for _, c := range r.conns {
			c.Close()
		}
	})
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			s, err := net.Dial("tcp", to)
			if err != nil {
				c.Close()
				continue
			}
			r.mu.Lock()
			r.conns = append(r.conns, c, s)
			first := !r.first
			r.first = true
			r.mu.Unlock()
			go r.records(s, c, first)
			go r.pass(c, s)
		}
	}()
	return r
}

func (r *relay) addr() string { return r.ln.Addr().String() }

// records passes on what the controller sends, record by TLS record.
func (r *relay) records(dst, src net.Conn, keep bool) {
	defer dst.Close()
	for {
		var head [5]byte
		if _, err := io.ReadFull(src, head[:]); err != nil {
			return
		}
		body := make([]byte, binary.BigEndian.Uint16(head[3:]))
		if _, err := io.ReadFull(src, body); err != nil {
			return
		}
		rec := append(head[:], body...)
		r.mu.Lock()
		if keep {
			r.sent = append(r.sent, rec...)
		}
		twice := r.twice && head[0] == 23 // application data
		if twice {
			r.twice = false
		}
		dropped := r.dropped
		r.mu.Unlock()
		if dropped {
			continue
		}
		if _, err := dst.Write(rec); err != nil {
			return
		}
		if twice {
			if _, err := dst.Write(rec); err != nil {
				return
			}
		}
	}
}

// pass passes on what the computer controlled sends.
func (r *relay) pass(dst, src net.Conn) {
	defer dst.Close()
	buf := make([]byte, 32<<10)
	for {
		n, err := src.Read(buf)
		if err != nil {
			return
		}
		r.mu.Lock()
		dropped := r.dropped
		r.mu.Unlock()
		if dropped {
			continue
		}
		if _, err := dst.Write(buf[:n]); err != nil {
			return
		}
	}
}

// TestReplayed refuses what was sent before, sent again: a record of a
// session within it, and a whole session's on a new connection. The
// command they carry runs once.
func TestReplayed(t *testing.T) {
	h := newFakeHost()
	s, addr := serve(t, h, fast)
	slow := fast
	slow.Ping = time.Minute // nothing but the command goes
	d := newDialer(t, "Laptop", slow)
	peer, err := pair(t, d, addr, h, same)
	if err != nil {
		t.Fatal(err)
	}
	<-h.ended
	r := newRelay(t, addr)
	sess := connect(t, d, r.addr(), peer.Key)
	waitFor(t, "the state", func() bool { _, _, ok := sess.State(); return ok }, func() any { return sess.Err() })
	r.mu.Lock()
	r.twice = true
	r.mu.Unlock()
	if err := sess.Do(Command{Op: OpPause}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-sess.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("the session went on after a record came twice")
	}
	waitFor(t, "the session's end there", func() bool { return s.Sessions() == 0 }, func() any { return s.Sessions() })
	if got, want := h.got(), []Command{{Op: OpPause}}; !reflect.DeepEqual(got, want) {
		t.Errorf("commands %+v, want %+v", got, want)
	}

	// The whole of what the controller sent, again.
	r.mu.Lock()
	sent := slices.Clone(r.sent)
	r.mu.Unlock()
	c, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, err := c.Write(sent); err != nil {
		t.Fatal(err)
	}
	c.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := io.Copy(io.Discard, c); err != nil {
		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() {
			t.Error("the connection sent again stayed open")
		}
	}
	waitFor(t, "the connection's end there", func() bool { return s.Sessions() == 0 }, func() any { return s.Sessions() })
	if got := h.got(); len(got) != 1 {
		t.Errorf("commands %+v after the session was sent again", got)
	}
}

// TestVanished ends a session on both computers once the other goes quiet,
// as one asleep or gone from the network.
func TestVanished(t *testing.T) {
	h := newFakeHost()
	timing := fast
	timing.Silence = 500 * time.Millisecond
	s, addr := serve(t, h, timing)
	d := newDialer(t, "Laptop", timing)
	peer, err := pair(t, d, addr, h, same)
	if err != nil {
		t.Fatal(err)
	}
	<-h.ended
	r := newRelay(t, addr)
	sess := connect(t, d, r.addr(), peer.Key)
	waitFor(t, "the state", func() bool { _, _, ok := sess.State(); return ok }, func() any { return sess.Err() })
	waitFor(t, "the session there", func() bool { return s.Sessions() == 1 }, func() any { return s.Sessions() })
	r.mu.Lock()
	r.dropped = true
	r.mu.Unlock()
	select {
	case <-sess.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("the session went on with a computer gone quiet")
	}
	if !errors.Is(sess.Err(), ErrLost) {
		t.Errorf("the session ended with %v", sess.Err())
	}
	waitFor(t, "the session's end there", func() bool { return s.Sessions() == 0 }, func() any { return s.Sessions() })
}

// TestClosed ends the sessions as the server closes, as remote control
// turns off, which they're told.
func TestClosed(t *testing.T) {
	h := newFakeHost()
	s, addr := serve(t, h, fast)
	d := newDialer(t, "Laptop", fast)
	peer, err := pair(t, d, addr, h, same)
	if err != nil {
		t.Fatal(err)
	}
	<-h.ended
	sess := connect(t, d, addr, peer.Key)
	waitFor(t, "the state", func() bool { _, _, ok := sess.State(); return ok }, func() any { return sess.Err() })
	s.Close()
	select {
	case <-sess.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("the session went on after the server closed")
	}
	if !errors.Is(sess.Err(), ErrOff) {
		t.Errorf("the session ended with %v", sess.Err())
	}
	if _, err := d.Connect(context.Background(), addr, peer.Key); err == nil {
		t.Error("connected to a server closed")
	}
}

func TestIdentity(t *testing.T) {
	id, err := NewIdentity()
	if err != nil {
		t.Fatal(err)
	}
	again, err := IdentityOf(id.Seed())
	if err != nil {
		t.Fatal(err)
	}
	if again.Fingerprint() != id.Fingerprint() {
		t.Error("the same seed made another key")
	}
	fp := id.Fingerprint()
	if got, err := ParseFingerprint(fp.String()); err != nil || got != fp {
		t.Errorf("the fingerprint came back as %v, %v", got, err)
	}
	if n := len(fp.ID()); n != 16 {
		t.Errorf("an ID of %d letters: %q", n, fp.ID())
	}
	if _, err := ParseFingerprint("abc"); err == nil {
		t.Error("read a fingerprint of 3 letters")
	}
	if _, err := IdentityOf([]byte("short")); err == nil {
		t.Error("made a key of a short seed")
	}
}

func TestCodes(t *testing.T) {
	for i := range 100 {
		code, err := NewCode()
		if err != nil {
			t.Fatal(err)
		}
		if v, ok := CodeValue(code); !ok || v >= 1_000_000 || len(code) != 6 {
			t.Fatalf("code %d: %q", i, code)
		}
	}
	for in, want := range map[string]uint32{"482 913": 482913, "000-001": 1, "999999": 999999} {
		if v, ok := CodeValue(in); !ok || v != want {
			t.Errorf("CodeValue(%q) = %d, %v", in, v, ok)
		}
	}
	for _, in := range []string{"", "12345", "1234567", "12a456", "１２３４５６"} {
		if _, ok := CodeValue(in); ok {
			t.Errorf("read %q", in)
		}
	}
}

// TestClean keeps what a computer sends in range: the words it shows, a
// cover only from https, and no more than MaxNext tracks after.
func TestClean(t *testing.T) {
	next := make([]Track, MaxNext+5)
	st := cleanState(State{Track: &Track{Title: "A\x00b\n", Artists: []string{"", "X"}, Artwork: "file:///etc/passwd"},
		Volume: 7, PositionMs: -4, Next: next})
	if st.Track.Title != "Ab" || !slices.Equal(st.Track.Artists, []string{"X"}) || st.Track.Artwork != "" {
		t.Errorf("track %+v", st.Track)
	}
	if st.Volume != 1 || st.PositionMs != 0 || len(st.Next) != MaxNext {
		t.Errorf("state %+v", st)
	}
	if got := cleanName("  \x07Desk\u0000top  "); got != "Desktop" {
		t.Errorf("name %q", got)
	}
	if got := cleanName(""); got != "Another computer" {
		t.Errorf("no name: %q", got)
	}
}

// TestFoundOf finds the computers of this protocol's version, by the ID
// and name of their TXT records.
func TestFoundOf(t *testing.T) {
	got := foundOf([]dnssd.Instance{
		{Name: "MixMusic a", Host: "192.0.2.1", Port: 4000, Text: map[string]string{"id": "aaaa", "name": "Desktop", "v": "1"}},
		{Name: "MixMusic b", Host: "192.0.2.2", Port: 4000, Text: map[string]string{"id": "bbbb", "name": "Old", "v": "0"}},
		{Name: "MixMusic c", Host: "192.0.2.3", Port: 4000, Text: map[string]string{"name": "No ID", "v": "1"}},
	})
	want := []Found{{ID: "aaaa", Name: "Desktop", Addr: "192.0.2.1:4000"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("found %+v, want %+v", got, want)
	}
}
