package remote

import (
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"testing"
	"time"
)

// held is how many places the server holds now.
func held(s *Server) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.adm.unproven)
}

// TestAdmissionPlaces lets in connections of a few sources: a source holds
// maxUnprovenFrom places at most, and the server maxUnproven, each new one
// taking the place of the oldest of its source, or of the source that
// holds the most.
func TestAdmissionPlaces(t *testing.T) {
	var a admission
	var closed []string
	let := func(src string, i int) *unproven {
		name := fmt.Sprint(src, "#", i)
		u, out := a.letIn(src, func() { closed = append(closed, name) })
		for _, o := range out {
			o.close()
		}
		return u
	}
	for i := range 10 {
		let("10.0.0.9", i)
	}
	if len(a.unproven) != maxUnprovenFrom || !slices.Equal(closed, []string{"10.0.0.9#0", "10.0.0.9#1", "10.0.0.9#2", "10.0.0.9#3", "10.0.0.9#4", "10.0.0.9#5"}) {
		t.Fatalf("one source: %d places, closed %v", len(a.unproven), closed)
	}
	// Many sources fill the places; then each new one takes the place of
	// the oldest of the source that holds the most, the flood's.
	closed = nil
	for i := range maxUnproven - maxUnprovenFrom {
		let(fmt.Sprintf("10.0.1.%d", i), 0)
	}
	if len(a.unproven) != maxUnproven || len(closed) != 0 {
		t.Fatalf("filled: %d places, closed %v", len(a.unproven), closed)
	}
	let("10.0.2.1", 0)
	if len(a.unproven) != maxUnproven || !slices.Equal(closed, []string{"10.0.0.9#6"}) {
		t.Fatalf("full: %d places, closed %v", len(a.unproven), closed)
	}
	// Places given back are taken again without closing any.
	closed = nil
	u := a.unproven[5]
	a.proven(u)
	a.proven(u)
	let("10.0.2.2", 0)
	if len(a.unproven) != maxUnproven || len(closed) != 0 {
		t.Fatalf("after one gave its place back: %d places, closed %v", len(a.unproven), closed)
	}
}

// TestSourceOf tells sources apart: an IPv4 address, and an IPv6 network.
func TestSourceOf(t *testing.T) {
	for addr, want := range map[string]string{
		"192.168.1.20:5000":                  "192.168.1.20",
		"[::ffff:192.168.1.20]:5000":         "192.168.1.20",
		"[fd00:1:2:3:aaaa:bbbb:cccc:dddd]:1": "fd00:1:2:3::/64",
		"[fd00:1:2:3::9]:1":                  "fd00:1:2:3::/64",
		"[fe80::1%eth0]:1":                   "fe80::/64",
		"[::1]:1":                            "::1",
		"127.0.0.1:1":                        "127.0.0.1",
	} {
		if got := sourceOf(addr); got != want {
			t.Errorf("sourceOf(%q) = %q, not %q", addr, got, want)
		}
	}
}

// TestUnpairedConnectionsCapped opens many connections from one computer
// that isn't paired, some silent after TLS, some before it: the server
// holds maxUnprovenFrom of them at most, and ends the oldest (#736).
func TestUnpairedConnectionsCapped(t *testing.T) {
	s, addr := serve(t, newFakeHost(), patient)
	const n = 3 * maxUnprovenFrom
	var conns []net.Conn
	for range n {
		conns = append(conns, dialUnpaired(t, addr))
		if h := held(s); h > maxUnprovenFrom {
			t.Fatalf("%d places held", h)
		}
	}
	// Connections that never start TLS hold places too.
	for range n {
		c, err := net.Dial("tcp", addr)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { c.Close() })
		conns = append(conns, c)
	}
	for i, c := range conns[:len(conns)-maxUnprovenFrom] {
		if !endedWithin(c, 5*time.Second) {
			t.Fatalf("connection %d of %d went on", i, len(conns))
		}
	}
	if h := held(s); h > maxUnprovenFrom {
		t.Fatalf("%d places held", h)
	}
	for i, c := range conns[len(conns)-maxUnprovenFrom:] {
		if endedWithin(c, 50*time.Millisecond) {
			t.Errorf("connection %d of the newest ended", i)
		}
	}
}

// TestPairingUntouched runs a pairing while a computer not paired floods
// the server: no new connection takes the pairing's place, another
// computer that asks to pair is refused as busy, and the pairing goes.
func TestPairingUntouched(t *testing.T) {
	h := newFakeHost()
	s, addr := serve(t, h, patient)
	d := newDialer(t, "Laptop", patient)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	codes := make(chan string)
	paired := make(chan error, 1)
	go func() {
		_, err := d.Pair(ctx, addr, func(ctx context.Context) (string, error) {
			select {
			case code := <-codes:
				return code, nil
			case <-ctx.Done():
				return "", ctx.Err()
			}
		})
		paired <- err
	}()
	var req PairRequest
	select {
	case req = <-h.shown:
	case <-ctx.Done():
		t.Fatal("no code showed")
	}
	if held(s) != 0 {
		t.Fatalf("the pairing holds a place")
	}
	for range 3 * maxUnprovenFrom {
		dialUnpaired(t, addr)
	}
	if _, err := pair(t, newDialer(t, "Phone", patient), addr, h, same); !errors.Is(err, ErrBusy) {
		t.Fatalf("another pairing meanwhile: %v, not ErrBusy", err)
	}
	codes <- req.Code
	if err := <-paired; err != nil {
		t.Fatalf("the pairing: %v", err)
	}
	if err := <-h.ended; err != nil {
		t.Fatalf("the pairing ended with %v there", err)
	}
}

// TestSessionUntouched floods the server with connections of a computer
// not paired while a paired one's session runs: the session goes on, and
// a paired computer connects again past the flood.
func TestSessionUntouched(t *testing.T) {
	h, s, d, sess := controlled(t)
	for range 3 * maxUnprovenFrom {
		dialUnpaired(t, addrOf(s))
	}
	h.setState(State{Track: &Track{Title: "After the flood"}, Volume: 1})
	waitFor(t, "the state after the flood", func() bool {
		st, _, _ := sess.State()
		return st.Track != nil && st.Track.Title == "After the flood"
	}, func() any { return sess.Err() })
	if err := sess.Do(Command{Op: OpPause}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the command", func() bool { return len(h.got()) == 1 }, func() any { return h.got() })
	again := connect(t, d, addrOf(s), s.Identity.Fingerprint())
	waitFor(t, "the state of a session connected again", func() bool { _, _, ok := again.State(); return ok }, func() any { return again.Err() })
}

// addrOf is the address s listens on.
func addrOf(s *Server) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ln.Addr().String()
}
