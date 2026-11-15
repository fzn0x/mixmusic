package dnssd

import (
	"errors"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeLAN is a network whose sockets hear what any of them sends, their
// own sends too, as mDNS's group does with multicast loopback. Until it is
// allowed, as on macOS before the user lets an app use the local network,
// a send fails, and a socket opened meanwhile stays deaf after: what the
// computer that wasn't found did (#725).
type fakeLAN struct {
	mu      sync.Mutex
	allowed bool
	socks   []*fakeSock
	probes  int
}

var errNoRoute = errors.New("sendto: no route to host")

func (l *fakeLAN) open() (net.PacketConn, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	s := &fakeSock{lan: l, addr: &net.UDPAddr{IP: net.IPv4(10, 0, 0, byte(len(l.socks)+1)), Port: 5353},
		deaf: !l.allowed, in: make(chan fakePacket, 64), closed: make(chan struct{}), fail: make(chan error, 1)}
	l.socks = append(l.socks, s)
	return s, nil
}

func (l *fakeLAN) allow() {
	l.mu.Lock()
	l.allowed = true
	l.mu.Unlock()
}

// opened is how many sockets were opened, and how many of them are open.
func (l *fakeLAN) opened() (all, open int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, s := range l.socks {
		if !s.isClosed() {
			open++
		}
	}
	return len(l.socks), open
}

// sock is the i-th socket opened.
func (l *fakeLAN) sock(i int) *fakeSock {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.socks[i]
}

func (l *fakeLAN) probesSent() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.probes
}

type fakePacket struct {
	b    []byte
	from net.Addr
}

type fakeSock struct {
	lan    *fakeLAN
	addr   *net.UDPAddr
	deaf   bool
	in     chan fakePacket
	closed chan struct{}
	once   sync.Once
	// fail is an error the next read returns.
	fail chan error
}

func (s *fakeSock) isClosed() bool {
	select {
	case <-s.closed:
		return true
	default:
		return false
	}
}

func (s *fakeSock) WriteTo(b []byte, _ net.Addr) (int, error) {
	l := s.lan
	l.mu.Lock()
	defer l.mu.Unlock()
	if s.isClosed() {
		return 0, net.ErrClosed
	}
	if !l.allowed || s.deaf {
		return 0, errNoRoute
	}
	if m, err := parse(b); err == nil && !m.response && len(m.questions) == 1 && strings.HasPrefix(m.questions[0].name, "probe.") {
		l.probes++
	}
	for _, o := range l.socks {
		if o.deaf || o.isClosed() {
			continue
		}
		select {
		case o.in <- fakePacket{append([]byte(nil), b...), s.addr}:
		default:
		}
	}
	return len(b), nil
}

func (s *fakeSock) ReadFrom(b []byte) (int, net.Addr, error) {
	select {
	case p := <-s.in:
		return copy(b, p.b), p.from, nil
	case err := <-s.fail:
		return 0, nil, err
	case <-s.closed:
		return 0, nil, net.ErrClosed
	}
}

func (s *fakeSock) Close() error {
	s.once.Do(func() { close(s.closed) })
	return nil
}

func (s *fakeSock) LocalAddr() net.Addr              { return s.addr }
func (s *fakeSock) SetDeadline(time.Time) error      { return nil }
func (s *fakeSock) SetReadDeadline(time.Time) error  { return nil }
func (s *fakeSock) SetWriteDeadline(time.Time) error { return nil }

// quickKeeper keeps r answering on lan's sockets, checked quickly.
func quickKeeper(lan *fakeLAN, r *Responder) *keeper {
	return &keeper{r: r, open: lan.open, group: mdnsGroup, first: 10 * time.Millisecond, wait: 200 * time.Millisecond,
		every: func(int) time.Duration { return 20 * time.Millisecond }, after: time.After}
}

// keepOn starts k until the test ends.
func keepOn(t *testing.T, k *keeper) {
	t.Helper()
	stop, err := k.start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(stop)
}

// answered reports whether a querier on lan, asking for the service, gets
// an answer within a while.
func answered(t *testing.T, lan *fakeLAN) bool {
	t.Helper()
	conn, _ := lan.open()
	defer conn.Close()
	q, err := appendQuery(nil, 0, []question{{name: testService, qtype: typePTR}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.WriteTo(q, mdnsGroup); err != nil {
		return false
	}
	got := make(chan bool, 1)
	go func() {
		buf := make([]byte, 9000)
		for {
			n, _, err := conn.ReadFrom(buf)
			if err != nil {
				got <- false
				return
			}
			if m, err := parse(buf[:n]); err == nil && m.response {
				got <- true
				return
			}
		}
	}()
	select {
	case ok := <-got:
		return ok
	case <-time.After(300 * time.Millisecond):
		conn.Close()
		return false
	}
}

// eventually polls ok for five seconds.
func eventually(ok func() bool) bool {
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if ok() {
			return true
		}
	}
	return false
}

func testResponder() *Responder {
	return &Responder{Service: testService, Instance: "Desktop", Port: 41234, Text: []string{"id=abcd"}, AddrFor: loopback}
}

// TestKeepPutsBackADeafSocket: a socket opened before the network let the
// app in stays deaf after; the keeper finds it out, and answers on a fresh
// socket, once the fresh one hears the network. Before, the computer
// wasn't found until it restarted (#725).
func TestKeepPutsBackADeafSocket(t *testing.T) {
	lan := &fakeLAN{}
	keepOn(t, quickKeeper(lan, testResponder()))
	// While the network refuses, fresh sockets do no better: the deaf
	// one stays.
	if !eventually(func() bool { all, _ := lan.opened(); return all >= 3 }) {
		t.Fatal("the keeper didn't try a fresh socket while the network refused")
	}
	if lan.sock(0).isClosed() {
		t.Error("the keeper dropped its socket for one that did no better")
	}
	lan.allow()
	if !eventually(func() bool { return answered(t, lan) }) {
		t.Fatal("once the network let the app in, the responder still didn't answer")
	}
	if !lan.sock(0).isClosed() {
		t.Error("the deaf socket stayed open")
	}
}

// TestKeepKeepsAWorkingSocket: a socket that hears its probes stays.
func TestKeepKeepsAWorkingSocket(t *testing.T) {
	lan := &fakeLAN{allowed: true}
	keepOn(t, quickKeeper(lan, testResponder()))
	if !eventually(func() bool { return lan.probesSent() >= 3 }) {
		t.Fatal("the keeper doesn't check its socket")
	}
	if all, _ := lan.opened(); all != 1 {
		t.Errorf("%d sockets opened for a socket that works, want 1", all)
	}
	if !answered(t, lan) {
		t.Error("the responder doesn't answer")
	}
}

// TestKeepRestartsAStoppedResponder: a Serve that stopped on an error is
// put back on a fresh socket at once.
func TestKeepRestartsAStoppedResponder(t *testing.T) {
	lan := &fakeLAN{allowed: true}
	k := quickKeeper(lan, testResponder())
	// No check comes by the clock: the Serve that stopped calls one.
	k.first, k.every = time.Hour, func(int) time.Duration { return time.Hour }
	keepOn(t, k)
	lan.sock(0).fail <- errors.New("read: something broke")
	if !eventually(func() bool { return answered(t, lan) && lan.sock(0).isClosed() }) {
		t.Fatal("the responder that stopped wasn't put back")
	}
}

// TestKeepStops: stop closes the socket answered on, and every fresh one.
func TestKeepStops(t *testing.T) {
	lan := &fakeLAN{allowed: true}
	stop, err := quickKeeper(lan, testResponder()).start()
	if err != nil {
		t.Fatal(err)
	}
	if !eventually(func() bool { return lan.probesSent() >= 1 }) {
		t.Fatal("the keeper doesn't check its socket")
	}
	stop()
	stop()
	if _, open := lan.opened(); open != 0 {
		t.Errorf("%d sockets open after stop", open)
	}
}

// TestCheckEvery: checks come within seconds while they fail, and half a
// minute apart at most; a minute apart while the socket works.
func TestCheckEvery(t *testing.T) {
	for failed, want := range map[int]time.Duration{0: time.Minute, 1: 2 * time.Second, 2: 8 * time.Second, 3: 18 * time.Second, 4: 30 * time.Second, 50: 30 * time.Second} {
		if got := checkEvery(failed); got != want {
			t.Errorf("after %d failed checks, the next in %v, want %v", failed, got, want)
		}
	}
}
