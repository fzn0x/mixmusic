package app

import (
	"net"
	"net/netip"
	"testing"

	"github.com/fzn0x/mixmusic/internal/room/roomtest"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// TestNearbyOf tells the addresses of this computer's networks from the
// internet's.
func TestNearbyOf(t *testing.T) {
	nets := []netip.Prefix{netip.MustParsePrefix("198.51.100.0/24"), netip.MustParsePrefix("2001:db8:5::/64")}
	for a, want := range map[string]bool{
		"192.168.1.20": true, "10.0.0.5": true, "127.0.0.1": true, "169.254.1.1": true, "fe80::1%eth0": true, "fd00::5": true,
		"::ffff:192.168.1.20": true, "198.51.100.77": true, "2001:db8:5::99": true,
		"203.0.113.7": false, "100.101.102.103": false, "2001:db8:6::1": false,
	} {
		if got := nearbyOf(netip.MustParseAddr(a), nets); got != want {
			t.Errorf("nearbyOf(%s) = %v", a, got)
		}
	}
}

// fromConn is a connection of the address from.
type fromConn struct {
	net.Conn
	from net.Addr
}

func (c fromConn) RemoteAddr() net.Addr { return c.from }

// fromListener hands out connections of the addresses of from, in turn.
type fromListener struct {
	net.Listener
	from chan net.Addr
}

func (l fromListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return fromConn{c, <-l.from}, nil
}

// TestRoomOfTheNetworkTakesNoOneFromOutside hosts a room with the switch
// off: a connection from the internet is closed before TLS, one of the
// network is taken; with the switch on, both are.
func TestRoomOfTheNetworkTakesNoOneFromOutside(t *testing.T) {
	desk := newRoomApp(t, roomtest.New(), "Desk")
	setRooms(t, desk.App, func(s *settings.Settings) { s.RoomAddress = "127.0.0.1" })
	raw, err := roomtest.Listen()
	if err != nil {
		t.Fatal(err)
	}
	from := make(chan net.Addr, 4)
	ln, err := desk.reachable(func() (net.Listener, error) { return fromListener{raw, from}, nil })()
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	accepted := make(chan net.Addr, 4)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			accepted <- c.RemoteAddr()
			c.Close()
		}
	}()
	dial := func(addr string) {
		from <- &net.TCPAddr{IP: net.ParseIP(addr), Port: 4000}
		c, err := net.Dial("tcp", raw.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		c.Close()
	}
	dial("203.0.113.7")
	dial("192.168.1.20")
	if got := (<-accepted).String(); got != "192.168.1.20:4000" {
		t.Fatalf("took %s first, with the switch off", got)
	}
	desk.room.far.reach.Load().start()
	dial("203.0.113.8")
	if got := (<-accepted).String(); got != "203.0.113.8:4000" {
		t.Fatalf("took %s, with the switch on", got)
	}
}
