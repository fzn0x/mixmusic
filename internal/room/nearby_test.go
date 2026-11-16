package room

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"testing"
)

// netsOf is a Nearby of the networks nets, in place of this computer's
// interfaces.
func netsOf(nets ...string) *Nearby {
	var ps []netip.Prefix
	for _, n := range nets {
		ps = append(ps, netip.MustParsePrefix(n))
	}
	return &Nearby{Nets: func() []netip.Prefix { return ps }}
}

// TestNearbyOfItsKind tells, with none of this computer's networks known,
// the addresses of networks of its own kind, private, loopback and
// link-local, from those of the internet, the providers' shared range
// among them, a knock's address as its own: of IPv6 too, with its zone.
func TestNearbyOfItsKind(t *testing.T) {
	for addr, want := range map[string]bool{
		"192.168.1.20": true, "10.1.2.3": true, "172.16.5.4": true, "127.0.0.1": true, "169.254.3.4": true,
		"[fd00:1:2:3::9]:5": true, "[fe80::9%eth0]:5": true, "[::1]:5": true, "[::ffff:192.168.1.20]:5": true,
		"203.0.113.7:5": false, "100.101.102.103": false, "[2001:db8:1:2::9]:5": false, "not an address": false,
	} {
		if got := netsOf().Of(addr); got != want {
			t.Errorf("Of(%q) = %v", addr, got)
		}
	}
}

// TestNearbyOfItsNetworks counts the networks of this computer's
// interfaces as its own, as a network of public addresses.
func TestNearbyOfItsNetworks(t *testing.T) {
	lan := netsOf("198.51.100.0/24", "2001:db8:5::/64")
	for addr, want := range map[string]bool{"198.51.100.77": true, "[2001:db8:5::99]:5": true, "198.51.101.1": false,
		"[2001:db8:6::1]:5": false, "100.101.102.103": false} {
		if got := lan.Of(addr); got != want {
			t.Errorf("on a LAN of public addresses, Of(%q) = %v", addr, got)
		}
	}
}

// nearRoom is a test room whose server takes the connections of addresses
// of near as of this computer's own networks, and the others as the
// internet's, whatever they are.
func nearRoom(t *testing.T, near func(addr string) bool, set func(*Server)) *testRoom {
	t.Helper()
	return newTestRoomWith(t, func(s *Server) {
		s.nearOf = near
		if set != nil {
			set(s)
		}
	})
}

// TestWrongCodesOfAPublicLANCountAsTheNetwork has the internet try as many
// wrong codes as the room takes: a guest of the host's network, whose
// addresses are public, still tries its code, as one of its own networks.
func TestWrongCodesOfAPublicLANCountAsTheNetwork(t *testing.T) {
	far := true
	r := nearRoom(t, func(string) bool { return !far }, func(s *Server) {
		s.caps = &caps{unproven: maxUnproven, unprovenFrom: maxUnproven, tries: 100, roomTries: 3}
	})
	g := newGuest(t, "Guesser")
	for i := range 3 {
		if _, err := g.Knock(context.Background(), r.addr, nil, fmt.Appendf(nil, "ZZZ%03d", i), false); !errors.Is(err, ErrWrongCode) {
			t.Fatalf("wrong code %d: %v", i, err)
		}
	}
	if _, err := g.Knock(context.Background(), r.addr, nil, []byte("ABC123"), false); !errors.Is(err, ErrLocked) {
		t.Fatalf("the code from the internet, as it takes none: %v", err)
	}
	far = false
	s, err := newGuest(t, "Friend").Knock(context.Background(), r.addr, nil, []byte("ABC123"), false)
	if err != nil {
		t.Fatalf("the code from the host's network: %v", err)
	}
	s.Close()
}

// TestKnockOfAPublicLANIsOfTheNetwork has the host's user let in two
// computers, which the room takes as of its own networks and of the
// internet, as their addresses are: each knock tells which.
func TestKnockOfAPublicLANIsOfTheNetwork(t *testing.T) {
	far := false
	f := &fakeGuests{rules: Rules{Ask: true}}
	r := nearRoom(t, func(string) bool { return !far }, func(s *Server) { s.Guests = f.guests() })
	for _, want := range []bool{true, false} {
		far = !want
		s := r.knockByLink(newGuest(t, "Ana"))
		joined := joining(s)
		waitFor(t, "the knock", func() bool { return len(r.server.Knocks()) == 1 })
		k := r.server.Knocks()[0]
		if k.Local != want {
			t.Fatalf("a knock of this computer's networks %v: told %v", want, k.Local)
		}
		r.server.LetIn(k.ID, false)
		if err := <-joined; !errors.Is(err, ErrDeclined) {
			t.Fatal(err)
		}
	}
}
