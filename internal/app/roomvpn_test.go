package app

import (
	"net"
	"net/netip"
	"testing"

	"github.com/fzn0x/mixmusic/internal/room/roomtest"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// TestRoomOfTheNetworkTakesVPNFriends hosts a room with the switch off on
// a computer whose networks are vpn: a friend on the same VPN of the
// providers' shared range, as Tailscale's, connects, while one of that
// range is turned away when this computer isn't on it, as is the internet.
func TestRoomOfTheNetworkTakesVPNFriends(t *testing.T) {
	for _, tc := range []struct {
		nets []string
		took string
	}{
		{[]string{"192.168.1.0/24", "100.70.1.2/32"}, "100.101.5.6:4000"},
		{[]string{"192.168.1.0/24"}, "192.168.1.20:4000"},
	} {
		desk := newRoomApp(t, roomtest.New(), "Desk")
		setRooms(t, desk.App, func(s *settings.Settings) { s.RoomAddress = "127.0.0.1" })
		var nets []netip.Prefix
		for _, n := range tc.nets {
			nets = append(nets, netip.MustParsePrefix(n))
		}
		desk.room.far.near.Nets = func() []netip.Prefix { return nets }
		raw, err := roomtest.Listen()
		if err != nil {
			t.Fatal(err)
		}
		from := make(chan net.Addr, 4)
		ln, err := desk.reachable(func() (net.Listener, error) { return fromListener{raw, from}, nil })()
		if err != nil {
			t.Fatal(err)
		}
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
		for _, addr := range []string{"203.0.113.7", "100.101.5.6", "192.168.1.20"} {
			from <- &net.TCPAddr{IP: net.ParseIP(addr), Port: 4000}
			c, err := net.Dial("tcp", raw.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			c.Close()
		}
		if got := (<-accepted).String(); got != tc.took {
			t.Errorf("on %v, took %s first, want %s", tc.nets, got, tc.took)
		}
		ln.Close()
	}
}
