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

// TestSourceOfKeepsIPv6ToItsSlash64 counts an IPv4 address alone, and an
// IPv6 address by its /64, which one computer changes within as it likes.
func TestSourceOfKeepsIPv6ToItsSlash64(t *testing.T) {
	same := [][2]string{
		{"[2001:db8:1:2::1]:4000", "[2001:db8:1:2:ffff:ffff:ffff:fffe]:5000"},
		{"[fe80::1%eth0]:4000", "[fe80::2%eth1]:4000"},
		{"192.168.1.20:4000", "[::ffff:192.168.1.20]:4001"},
	}
	for _, p := range same {
		if a, b := sourceOf(p[0]), sourceOf(p[1]); a != b {
			t.Errorf("%s is %s, %s is %s: one source", p[0], a, p[1], b)
		}
	}
	apart := [][2]string{
		{"[2001:db8:1:2::1]:4000", "[2001:db8:1:3::1]:4000"},
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
	if v := a.tryCode("10.9.9.9", now, wait); v != tryWaits {
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

// TestUnprovenConnectionsAreCapped holds connections that prove nothing:
// past maxUnprovenFrom of one source, the room closes the next at once;
// a guest that proves the code gives its place back.
func TestUnprovenConnectionsAreCapped(t *testing.T) {
	r := newTestRoom(t)
	var idle []net.Conn
	for range maxUnprovenFrom - 1 {
		c, err := net.Dial("tcp", r.addr)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { c.Close() })
		idle = append(idle, c)
	}
	waitFor(t, "the idle connections held", func() bool {
		r.server.mu.Lock()
		defer r.server.mu.Unlock()
		return r.server.adm.unproven == maxUnprovenFrom-1
	})
	// A guest takes the last place, proves the code, and gives it back:
	// another does too.
	r.join(newGuest(t, "One"))
	r.join(newGuest(t, "Two"))
	full, err := net.Dial("tcp", r.addr)
	if err != nil {
		t.Fatal(err)
	}
	defer full.Close()
	waitFor(t, "the last place held", func() bool {
		r.server.mu.Lock()
		defer r.server.mu.Unlock()
		return r.server.adm.unproven == maxUnprovenFrom
	})
	over, err := net.Dial("tcp", r.addr)
	if err != nil {
		t.Fatal(err)
	}
	defer over.Close()
	over.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := over.Read(make([]byte, 1)); err == nil || isTimeout(err) {
		t.Fatalf("a connection past the cap: %v, not closed at once", err)
	}
	// Those held stay open, until their silence.
	idle[0].SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	if _, err := idle[0].Read(make([]byte, 1)); !isTimeout(err) {
		t.Fatalf("a connection within the cap: %v", err)
	}
}

// TestRoomWideUnprovenCap closes a connection past the room's cap of the
// unproven, from whatever source.
func TestRoomWideUnprovenCap(t *testing.T) {
	r := newTestRoomWith(t, func(s *Server) { s.caps = &caps{unproven: 3, unprovenFrom: 100, tries: Tries, roomTries: RoomTries} })
	for range 3 {
		c, err := net.Dial("tcp", r.addr)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { c.Close() })
	}
	waitFor(t, "the room full of the unproven", func() bool {
		r.server.mu.Lock()
		defer r.server.mu.Unlock()
		return r.server.adm.unproven == 3
	})
	over, err := net.Dial("tcp", r.addr)
	if err != nil {
		t.Fatal(err)
	}
	defer over.Close()
	over.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := over.Read(make([]byte, 1)); err == nil || isTimeout(err) {
		t.Fatalf("a connection past the room's cap: %v, not closed at once", err)
	}
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
