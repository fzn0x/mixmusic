package main

import (
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/portmap"
	"github.com/fzn0x/mixmusic/internal/portmap/portmaptest"
	"github.com/fzn0x/mixmusic/internal/remote/remotetest"
	"github.com/fzn0x/mixmusic/internal/room"
	"github.com/fzn0x/mixmusic/internal/room/roomtest"
)

// newFarRoomShell is a test shell of the computer named name, whose rooms
// are on the network n, ask the fake router of the internet address
// external, and listen at a port of the loopback, whose address listened
// tells.
func newFarRoomShell(t *testing.T, n *roomtest.Network, name, external string) (*shell, *portmaptest.Router, func() string) {
	t.Helper()
	router, err := portmaptest.New(external)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(router.Close)
	var ln net.Listener
	s := newRemoteShellOf(t, remotetest.New(), name, func(o *app.Options) {
		o.Room = &app.RoomOptions{Discovery: n, Timing: roomTiming,
			Listen: func() (net.Listener, error) {
				var err error
				ln, err = roomtest.Listen()
				return ln, err
			},
			Router: &portmap.Client{
				Gateways:   func() ([]netip.Addr, error) { return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil },
				ServerPort: router.Port(), SSDP: router.SSDP(), Retry: 20 * time.Millisecond,
			}}
	})
	t.Cleanup(func() {
		s.a.Rooms().End()
		s.a.Rooms().Leave()
	})
	return s, router, func() string { return ln.Addr().String() }
}

// anywhereAbout is the text beside the switch of the room page.
const anywhereAbout = "MixMusic asks your router to forward the room's port, and the invite link carries where friends reach this computer. Anyone with the link or the code may knock: Remove and Block keep them out."

// TestRoomLetsFriendsOutsideJoin turns on the room page's switch: the page
// tells the router's forward, and copies the room's web link, which
// carries the router's address.
func TestRoomLetsFriendsOutsideJoin(t *testing.T) {
	s, router, listened := newFarRoomShell(t, roomtest.New(), "Desk", "203.0.113.7")
	tt := tester(t, s)
	if err := s.a.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	show(s, tt, roomsPath)
	until(t, s, tt, "the switch", func() bool { return tt.HasText("Let friends outside this network join") })
	if tt.HasText("copy web link") {
		t.Fatal("a web link while friends outside may not join")
	}
	toggle(t, s, tt, "Let friends outside this network join", anywhereAbout)
	_, port, _ := net.SplitHostPort(listened())
	until(t, s, tt, "the forward", func() bool {
		return tt.HasText("Your router forwards port " + port + " of its internet address, 203.0.113.7, to this computer (PCP).")
	})
	if len(router.Forwards()) != 1 {
		t.Fatalf("the router keeps %+v", router.Forwards())
	}
	was := copyText
	var copied string
	copyText = func(text string) { copied = text }
	t.Cleanup(func() { copyText = was })
	click(t, s, tt, "copy web link")
	in, err := room.ParseLink(copied)
	if err != nil || !strings.HasPrefix(copied, room.WebPage+"#") || len(in.Addrs) != 1 || in.Addrs[0] != "203.0.113.7:"+port {
		t.Fatalf("copied %q: %+v, %v", copied, in, err)
	}
	toggle(t, s, tt, "Let friends outside this network join", anywhereAbout)
	until(t, s, tt, "the switch off", func() bool { return !tt.HasText("copy web link") })
	for deadline := time.Now().Add(5 * time.Second); len(router.Forwards()) != 0; time.Sleep(5 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("the router keeps %+v", router.Forwards())
		}
	}
}

// TestRoomJoinAtAddress joins, from another network, a room at its
// address and with its code, on the rooms' page.
func TestRoomJoinAtAddress(t *testing.T) {
	desk, _, listened := newFarRoomShell(t, roomtest.New(), "Desk", "203.0.113.7")
	lap := newRoomShell(t, roomtest.New(), "Lap")
	dt, lt := tester(t, desk), tester(t, lap)
	if err := desk.a.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	hosted, _ := desk.a.Rooms().Hosted()
	show(lap, lt, roomsPath)
	until(t, lap, lt, "the address", func() bool {
		return lt.HasText("Or join a room at an address, with its code: its host's address on a VPN you both run, or its router's, with the port.")
	})
	click(t, lap, lt, "Room's address")
	lt.Type(listened())
	click(t, lap, lt, "Room code")
	lt.Type(strings.ToLower(hosted.Code))
	lap.drain()
	lt.Frame()
	click(t, lap, lt, "Join at the address")
	untilBoth(t, desk, dt, lap, lt, "the preview", func() bool { return lt.HasText("Desk hosts this room.") })
}
