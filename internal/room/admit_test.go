package room

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/remote"
)

// TestWrongCodesAtOnceTryFive knocks 40 times at once from one address,
// each with another wrong code: the room checks 5 of them at most, and the
// address then waits, the right code too.
func TestWrongCodesAtOnceTryFive(t *testing.T) {
	// The room holds all 40 connections, so that the tries are what
	// stops them.
	r := newTestRoomWith(t, func(s *Server) { s.caps = &caps{unproven: 100, unprovenFrom: 100, tries: Tries, roomTries: RoomTries} })
	g := newGuest(t, "Guesser")
	var wg sync.WaitGroup
	var mu sync.Mutex
	results := map[string]int{}
	for i := range 40 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := g.Knock(context.Background(), r.addr, nil, fmt.Appendf(nil, "ZZZ%03d", i), false)
			key := "other"
			switch {
			case errors.Is(err, ErrWrongCode):
				key = "wrong"
			case errors.Is(err, ErrWait):
				key = "wait"
			case errors.Is(err, ErrBusy):
				key = "busy"
			}
			mu.Lock()
			results[key]++
			mu.Unlock()
		}()
	}
	wg.Wait()
	t.Logf("40 knocks at once: %v", results)
	if results["wrong"] > Tries || results["wrong"] == 0 || results["other"] > 0 {
		t.Fatalf("40 knocks at once: %v; at most %d codes may be checked", results, Tries)
	}
	if _, err := g.Knock(context.Background(), r.addr, nil, []byte("ABC123"), false); !errors.Is(err, ErrWait) {
		t.Fatalf("the right code after the wrong ones: %v", err)
	}
}

// TestSourceOfKeepsIPv6ToItsSlash56 counts an IPv4 address alone, and an
// IPv6 address by its /56, as a home's router has from its provider: its
// 256 networks of /64 are one source.
func TestSourceOfKeepsIPv6ToItsSlash56(t *testing.T) {
	same := [][2]string{
		{"[2001:db8:1:2::1]:4000", "[2001:db8:1:2:ffff:ffff:ffff:fffe]:5000"},
		{"[2001:db8:1:200::1]:4000", "[2001:db8:1:2ff::1]:4000"},
		{"[fe80::1%eth0]:4000", "[fe80::2%eth1]:4000"},
		{"192.168.1.20:4000", "[::ffff:192.168.1.20]:4001"},
	}
	for _, p := range same {
		if a, b := sourceOf(p[0]), sourceOf(p[1]); a != b {
			t.Errorf("%s is %s, %s is %s: one source", p[0], a, p[1], b)
		}
	}
	apart := [][2]string{
		{"[2001:db8:1:200::1]:4000", "[2001:db8:1:300::1]:4000"},
		{"192.168.1.20:4000", "192.168.1.21:4000"},
	}
	for _, p := range apart {
		if a, b := sourceOf(p[0]), sourceOf(p[1]); a == b {
			t.Errorf("%s and %s are both %s", p[0], p[1], a)
		}
	}
}

// newAdmission is an admission of the default bounds.
func newAdmission() *admission { return &admission{caps: (*caps)(nil).orDefault()} }

// TestRoomWideTriesHoldAcrossSources has wrong codes come from as many
// sources as there are codes: once the room took RoomTries of them, it
// checks no code until the wait is over.
func TestRoomWideTriesHoldAcrossSources(t *testing.T) {
	a := newAdmission()
	now := time.Unix(1_000_000, 0)
	const wait = time.Minute
	checked := 0
	for i := range 200 {
		src := fmt.Sprintf("10.0.%d.%d", i/250, i%250)
		if a.tryCode(src, now, wait) != tryRuns {
			continue
		}
		checked++
		a.endTry(src, true, now, wait)
		now = now.Add(100 * time.Millisecond)
	}
	if checked != RoomTries {
		t.Fatalf("the room checked %d codes in %v, from 200 sources", checked, 20*time.Second)
	}
	if v := a.tryCode("10.9.9.9", now, wait); v != tryLocked {
		t.Fatalf("a new source, as the room waits: %v", v)
	}
	// The wait over, codes are checked again.
	now = now.Add(wait)
	if v := a.tryCode("10.9.9.9", now, wait); v != tryRuns {
		t.Fatalf("after the wait: %v", v)
	}
}

// TestRightCodesAtOnceDontLockTheRoom runs as many tries at once as the
// room takes: one more is told the room is busy, not to wait, and once they
// were the code, codes are checked as before.
func TestRightCodesAtOnceDontLockTheRoom(t *testing.T) {
	a := newAdmission()
	now := time.Unix(1_000_000, 0)
	const wait = time.Minute
	for i := range RoomTries {
		if v := a.tryCode(fmt.Sprintf("10.0.0.%d", i), now, wait); v != tryRuns {
			t.Fatalf("try %d: %v", i, v)
		}
	}
	if v := a.tryCode("10.0.1.1", now, wait); v != tryBusy {
		t.Fatalf("one more at once: %v", v)
	}
	for i := range RoomTries {
		a.endTry(fmt.Sprintf("10.0.0.%d", i), false, now, wait)
	}
	if v := a.tryCode("10.0.1.1", now, wait); v != tryRuns {
		t.Fatalf("after the right codes: %v", v)
	}
}

// TestSourcesStayBounded tries codes from a new source each time, as from
// addresses that rotate: the room counts maxSources at most, and lets the
// old go once they tried a while ago.
func TestSourcesStayBounded(t *testing.T) {
	a := newAdmission()
	a.caps.roomTries = 1 << 30
	now := time.Unix(1_000_000, 0)
	const wait = time.Minute
	for i := range 3 * maxSources {
		src := fmt.Sprintf("[2001:db8:%x::1]:4000", i)
		if a.tryCode(sourceOf(src), now, wait) == tryRuns {
			a.endTry(sourceOf(src), true, now, wait)
		}
		if len(a.sources) > maxSources {
			t.Fatalf("%d sources counted", len(a.sources))
		}
		now = now.Add(time.Millisecond)
	}
	// A while later, the sweep lets them all go.
	a.sweep(now.Add(wait), wait)
	if len(a.sources) != 0 {
		t.Fatalf("%d sources kept after the wait", len(a.sources))
	}
}

// unprovenNow is how many places of the unproven the room holds.
func (r *testRoom) unprovenNow() int {
	r.server.mu.Lock()
	defer r.server.mu.Unlock()
	return len(r.server.adm.unproven)
}

// closedSoon reports whether c's other end closed it within a second.
func closedSoon(c net.Conn) bool {
	c.SetReadDeadline(time.Now().Add(time.Second))
	_, err := c.Read(make([]byte, 1))
	return err != nil && !isTimeout(err)
}

// openIdle opens n connections to the room, which send nothing.
func (r *testRoom) openIdle(n int) []net.Conn {
	r.t.Helper()
	var out []net.Conn
	for range n {
		c, err := net.Dial("tcp", r.addr)
		if err != nil {
			r.t.Fatal(err)
		}
		r.t.Cleanup(func() { c.Close() })
		out = append(out, c)
		// One at a time, so that the room takes them in the order opened.
		limits := r.server.caps.orDefault()
		want := min(len(out), limits.unprovenFrom, limits.unproven)
		waitFor(r.t, "the connection held", func() bool { return r.unprovenNow() >= want })
	}
	return out
}

// TestNewConnectionTakesTheOldestPlace opens one connection more than a
// source may hold before proving anything: the oldest is closed, the new
// one held, so that connections held open keep no one out.
func TestNewConnectionTakesTheOldestPlace(t *testing.T) {
	r := newTestRoom(t)
	idle := r.openIdle(maxUnprovenFrom)
	newest := r.openIdle(1)[0]
	if !closedSoon(idle[0]) {
		t.Fatal("the oldest connection of the source wasn't closed")
	}
	newest.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	if _, err := newest.Read(make([]byte, 1)); !isTimeout(err) {
		t.Fatalf("the newest connection: %v", err)
	}
	if n := r.unprovenNow(); n != maxUnprovenFrom {
		t.Fatalf("%d places held", n)
	}
}

// TestRoomWidePlacesTakeTheOldest opens one connection more than the room
// holds before proving anything, from whatever source: the oldest is
// closed.
func TestRoomWidePlacesTakeTheOldest(t *testing.T) {
	r := newTestRoomWith(t, func(s *Server) { s.caps = &caps{unproven: 3, unprovenFrom: 100, tries: Tries, roomTries: RoomTries} })
	idle := r.openIdle(3)
	r.openIdle(1)
	if !closedSoon(idle[0]) {
		t.Fatal("the oldest connection wasn't closed")
	}
	if n := r.unprovenNow(); n != 3 {
		t.Fatalf("%d places held", n)
	}
}

// TestFullRoomStillLetsInLinksAndGuests fills every place of the unproven
// with connections held open: a link holder still joins, and a guest that
// dropped off still comes back by its key.
func TestFullRoomStillLetsInLinksAndGuests(t *testing.T) {
	// All come from 127.0.0.1: the source holds as many as the room.
	r := newTestRoomWith(t, func(s *Server) {
		s.caps = &caps{unproven: maxUnproven, unprovenFrom: maxUnproven, tries: Tries, roomTries: RoomTries}
	})
	back := newGuest(t, "Back")
	r.join(back).Close()
	waitFor(t, "the guest away", func() bool { m := r.server.Members(); return len(m) == 1 && !m[0].Here })
	r.openIdle(maxUnproven)
	link := newGuest(t, "Link")
	s, err := link.Knock(context.Background(), r.addr, &r.key, r.server.Secret, true)
	if err != nil {
		t.Fatalf("a link holder, with every place taken: %v", err)
	}
	if err := s.Join(context.Background()); err != nil {
		t.Fatal(err)
	}
	s.Close()
	again, err := back.Rejoin(context.Background(), r.addr, r.key)
	if err != nil {
		t.Fatalf("a guest that comes back, with every place taken: %v", err)
	}
	again.Close()
}

// TestProofHasOneDeadline gives a connection Prove in all, from its
// accepting, to prove anything: one that took TLS and sends nothing more
// is closed then, not a Silence after each step.
func TestProofHasOneDeadline(t *testing.T) {
	timing := testTiming
	timing.Prove = 300 * time.Millisecond
	r := newTestRoomWith(t, func(s *Server) { s.Timing = timing })
	id, _ := remote.NewIdentity()
	raw, err := net.Dial("tcp", r.addr)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	start := time.Now()
	tc := tls.Client(raw, id.ClientTLS(alpn, nil, nil))
	if err := tc.Handshake(); err != nil {
		t.Fatal(err)
	}
	tc.SetReadDeadline(time.Now().Add(testTiming.Silence))
	if _, err := tc.Read(make([]byte, 1)); err == nil || isTimeout(err) {
		t.Fatalf("a connection that proved nothing: %v", err)
	}
	if took := time.Since(start); took > 2*time.Second {
		t.Fatalf("closed after %v: more than Prove", took)
	}
	waitFor(t, "its place let go", func() bool { return r.unprovenNow() == 0 })
}

// TestRoomWindowRollsWithATryUnderWay keeps a try of a code under way past
// the room's minute: the minute rolls all the same, and its wrong codes
// count no more.
func TestRoomWindowRollsWithATryUnderWay(t *testing.T) {
	a := newAdmission()
	now := time.Unix(1_000_000, 0)
	const wait = time.Minute
	if a.tryCode("10.0.0.1", now, wait) != tryRuns {
		t.Fatal("the first try")
	}
	for i := range RoomTries - 1 {
		src := fmt.Sprintf("10.0.1.%d", i)
		if a.tryCode(src, now, wait) != tryRuns {
			t.Fatalf("try %d", i)
		}
		a.endTry(src, true, now, wait)
	}
	if v := a.tryCode("10.0.2.1", now, wait); v != tryBusy {
		t.Fatalf("one more, with 29 wrong and one under way: %v", v)
	}
	// A minute on, the first try still under way, the minute rolled.
	now = now.Add(wait + time.Second)
	if v := a.tryCode("10.0.2.1", now, wait); v != tryRuns {
		t.Fatalf("a minute on: %v", v)
	}
	// The sources are of the network: its count is the room's.
	if a.local.running != 2 || a.local.wrong != 0 {
		t.Fatalf("the room counts %d under way and %d wrong", a.local.running, a.local.wrong)
	}
}

// TestRoomLockTellsTheHostOnce tries wrong codes until the room takes none:
// the host hears it once, the activity tells it, the next knock is told
// the room takes no code, without its own wrong ones, and a link still
// opens the room.
func TestRoomLockTellsTheHostOnce(t *testing.T) {
	told := make(chan struct{}, 10)
	r := newTestRoomWith(t, func(s *Server) {
		s.caps = &caps{unproven: maxUnproven, unprovenFrom: maxUnproven, tries: 100, roomTries: 3}
		s.Guessing = func() { told <- struct{}{} }
	})
	g := newGuest(t, "Guesser")
	for i := range 3 {
		if _, err := g.Knock(context.Background(), r.addr, nil, fmt.Appendf(nil, "ZZZ%03d", i), false); !errors.Is(err, ErrWrongCode) {
			t.Fatalf("wrong code %d: %v", i, err)
		}
	}
	other := newGuest(t, "Friend")
	for range 2 {
		if _, err := other.Knock(context.Background(), r.addr, nil, []byte("ABC123"), false); !errors.Is(err, ErrLocked) {
			t.Fatalf("the right code, as the room takes none: %v", err)
		}
	}
	waitFor(t, "the host told", func() bool { return len(told) >= 1 })
	if n := len(told); n != 1 {
		t.Fatalf("the host was told %d times", n)
	}
	guessed := 0
	for _, a := range r.server.Activity() {
		if a.What == Guessed {
			guessed++
		}
	}
	if guessed != 1 {
		t.Fatalf("the activity tells it %d times: %+v", guessed, r.server.Activity())
	}
	s, err := other.Knock(context.Background(), r.addr, &r.key, r.server.Secret, true)
	if err != nil {
		t.Fatalf("a link, as the room takes no code: %v", err)
	}
	s.Close()
}

// TestHandshakeTakesNoLongMessage sends, before proving anything, the
// length of a message far longer than a hello, and nothing of it: the room
// refuses it from its length, and closes the connection at once.
func TestHandshakeTakesNoLongMessage(t *testing.T) {
	r := newTestRoom(t)
	id, _ := remote.NewIdentity()
	raw, err := net.Dial("tcp", r.addr)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	tc := tls.Client(raw, id.ClientTLS(alpn, nil, nil))
	if err := tc.Handshake(); err != nil {
		t.Fatal(err)
	}
	var head [4]byte
	binary.BigEndian.PutUint32(head[:], 512<<10)
	if _, err := tc.Write(head[:]); err != nil {
		t.Fatal(err)
	}
	tc.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := tc.Read(make([]byte, 1)); err == nil || isTimeout(err) {
		t.Fatalf("a long message before the proof: %v, not refused at once", err)
	}
}

func isTimeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

// TestFullRoomMakesRoomFromTheMostCrowdedSource fills the places of the
// unproven from sources that each hold a few, beside a guest's one on its
// way in: new connections of those sources take each other's places, and
// never the guest's.
func TestFullRoomMakesRoomFromTheMostCrowdedSource(t *testing.T) {
	a := newAdmission()
	guest, _ := a.letIn("198.51.100.7", func() {})
	for i := range 200 {
		_, gone := a.letIn(fmt.Sprintf("203.0.113.%d", i%9), func() {})
		for _, u := range gone {
			if u == guest {
				t.Fatalf("connection %d took the guest's place", i)
			}
		}
		if len(a.unproven) > maxUnproven {
			t.Fatalf("%d unproven", len(a.unproven))
		}
	}
	if guest.gone {
		t.Fatal("the guest lost its place")
	}
}
